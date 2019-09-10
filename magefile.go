// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// +build mage

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"

	"github.com/elastic/integrations-registry/util"
)

var (
	// GoImportsImportPath controls the import path used to install goimports.
	GoImportsImportPath = "golang.org/x/tools/cmd/goimports"

	// GoImportsLocalPrefix is a string prefix matching imports that should be
	// grouped after third-party packages.
	GoImportsLocalPrefix = "github.com/elastic"

	// GoLicenserImportPath controls the import path used to install go-licenser.
	GoLicenserImportPath = "github.com/elastic/go-licenser"

	publicDir    = "./public"
	buildDir     = "./build"
	packageDir   = "package"
	packagePaths = []string{"./dev/package-generated/", "./dev/package-examples/"}
)

func Check() error {
	Format()

	sh.RunV("git", "update-index", "--refresh")
	sh.RunV("git", "diff-index", "--exit-code", "HEAD", "--")

	return nil
}
func Test() error {
	sh.RunV("go", "get", "-v", "-u", "github.com/jstemmer/go-junit-report")
	return sh.RunV("go", "test", "./...", "-v", "2>&1", "|", "go-junit-report", ">", "junit-report.xml")
}

func Build() error {
	for _, p := range packagePaths {
		err := CopyPackages(p)
		if err != nil {
			return err
		}
	}

	err := BuildIntegrationPackages()
	if err != nil {
		return err
	}

	err = BuildRootFile()
	if err != nil {
		return err
	}

	return sh.Run("go", "build", ".")
}

// Format adds license headers, formats .go files with goimports, and formats
// .py files with autopep8.
func Format() {
	// Don't run AddLicenseHeaders and GoImports concurrently because they
	// both can modify the same files.
	mg.Deps(AddLicenseHeaders)
	mg.Deps(GoImports)
}

// BuildIntegrationPackages rebuilds the zip files inside packages
// PACKAGES_PATH env variable can be used to also rebuild testdata packages.
func BuildIntegrationPackages() error {

	// Check if PACKAGES_PATH is set.
	packagesBasePath := os.Getenv("PACKAGES_PATH")
	if packagesBasePath == "" {
		packagesBasePath = publicDir + "/" + packageDir + "/"
	}

	packagePaths, err := util.GetPackagePaths(packagesBasePath)
	if err != nil {
		return err
	}

	for _, path := range packagePaths {
		err = buildPackage(packagesBasePath, path)
		if err != nil {
			return err
		}
	}
	return nil
}

func buildPackage(packagesBasePath, path string) error {

	// Change path to simplify tar command
	currentPath, err := os.Getwd()
	if err != nil {
		return err
	}
	err = os.Chdir(packagesBasePath)
	if err != nil {
		return err
	}
	defer os.Chdir(currentPath)

	err = sh.RunV("tar", "cvzf", path+".tar.gz", filepath.Base(path)+"/")
	if err != nil {
		return err
	}

	// Build package endpoint
	p, err := util.NewPackage(".", path)
	if err != nil {
		return err
	}

	// Checks if the package is valid
	err = p.Validate()
	if err != nil {
		return fmt.Errorf("Invalid package %s-%s: %s", p.Name, p.Version, err)
	}

	err = p.LoadAssets(path)
	if err != nil {
		return err
	}

	err = writeJsonFile(p, path+"/index.json")
	if err != nil {
		return err
	}
	return nil
}

// Creates the `index.json` file
// For now only containing the version.
func BuildRootFile() error {
	rootData := map[string]string{
		"version":      "0.0.1",
		"service.name": "integration-registry",
	}

	return writeJsonFile(rootData, publicDir+"/index.json")
}

func writeJsonFile(v interface{}, path string) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, 0644)
}

// GoImports executes goimports against all .go files in and below the CWD. It
// ignores vendor/ directories.
func GoImports() error {
	goFiles, err := FindFilesRecursive(func(path string, _ os.FileInfo) bool {
		return filepath.Ext(path) == ".go" && !strings.Contains(path, "vendor/")
	})
	if err != nil {
		return err
	}
	if len(goFiles) == 0 {
		return nil
	}

	fmt.Println(">> fmt - goimports: Formatting Go code")
	args := append(
		[]string{"-local", GoImportsLocalPrefix, "-l", "-w"},
		goFiles...,
	)

	return sh.RunV("goimports", args...)
}

func CopyPackages(path string) error {
	fmt.Println(">> Copy packages: " + path)
	currentPath, err := os.Getwd()
	if err != nil {
		return err
	}
	err = os.Chdir(path)
	if err != nil {
		return err
	}
	defer os.Chdir(currentPath)

	dirs, err := ioutil.ReadDir("./")
	if err != nil {
		return err
	}

	os.MkdirAll("../../public/package/", 0755)
	for _, dir := range dirs {
		err := sh.RunV("cp", "-a", dir.Name(), "../../public/package/")
		if err != nil {
			return err
		}
	}

	return nil
}

// AddLicenseHeaders adds license headers to .go files. It applies the
// appropriate license header based on the value of mage.BeatLicense.
func AddLicenseHeaders() error {
	fmt.Println(">> fmt - go-licenser: Adding missing headers")
	return sh.RunV("go-licenser", "-license", "Elastic")
}

// FindFilesRecursive recursively traverses from the CWD and invokes the given
// match function on each regular file to determine if the given path should be
// returned as a match.
func FindFilesRecursive(match func(path string, info os.FileInfo) bool) ([]string, error) {
	var matches []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			// continue
			return nil
		}

		if match(filepath.ToSlash(path), info) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func Clean() error {
	err := os.RemoveAll(buildDir)
	if err != nil {
		return err
	}

	err = os.RemoveAll(publicDir)
	if err != nil {
		return err
	}

	return os.Remove("integrations-registry")
}
