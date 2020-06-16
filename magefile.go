// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// +build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"

	"github.com/elastic/package-registry/util"
)

var (
	// GoImportsImportPath controls the import path used to install goimports.
	GoImportsImportPath = "golang.org/x/tools/cmd/goimports"

	// GoImportsLocalPrefix is a string prefix matching imports that should be
	// grouped after third-party packages.
	GoImportsLocalPrefix = "github.com/elastic"

	// GoLicenserImportPath controls the import path used to install go-licenser.
	GoLicenserImportPath = "github.com/elastic/go-licenser"

	publicDir      = "./public"
	buildDir       = "./build"
	storageRepoDir = filepath.Join(buildDir, "package-storage")
	packagePaths   = []string{filepath.Join(storageRepoDir, "packages"), "./dev/packages/example/"}
	tarGz          = true
)

func Build() error {
	packagePathsEnv := os.Getenv("PACKAGE_PATHS")
	if packagePathsEnv != "" {
		packagePaths = strings.Split(packagePathsEnv, ",")
	}

	err := os.RemoveAll(publicDir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(publicDir, 0755)
	if err != nil {
		return err
	}

	err = fetchPackageStorage()
	if err != nil {
		return err
	}

	err = buildPackages()
	if err != nil {
		return err
	}
	return sh.Run("go", "build", ".")
}

func fetchPackageStorage() error {
	err := os.RemoveAll(storageRepoDir)
	if err != nil {
		return err
	}

	err = sh.Run("git", "clone", "https://github.com/elastic/package-storage.git", storageRepoDir)
	if err != nil {
		return err
	}

	packageStorageRevision := os.Getenv("PACKAGE_STORAGE_REVISION")
	if packageStorageRevision == "" {
		packageStorageRevision = "master"
	}

	return sh.Run("git",
		"--git-dir", filepath.Join(storageRepoDir, ".git"),
		"--work-tree", storageRepoDir,
		"checkout",
		packageStorageRevision)
}

func buildPackages() error {
	packagePaths, err := findPackages()
	if err != nil {
		return err
	}

	for _, packagePath := range packagePaths {
		srcDir := packagePath + "/"
		p, err := util.NewPackage(srcDir)
		if err != nil {
			return err
		}
		dstDir := filepath.Join(publicDir, "package", p.Name, p.Version)

		err = copyPackageFromSource(srcDir, dstDir)
		if err != nil {
			return err
		}
	}
	return nil
}

func findPackages() ([]string, error) {
	var matches []string
	for _, sourceDir := range packagePaths {
		err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			f, err := os.Stat(path)
			if err != nil {
				return err
			}

			if !f.IsDir() {
				return nil // skip as the path is not a directory
			}

			manifestPath := filepath.Join(path, "manifest.yml")
			_, err = os.Stat(manifestPath)
			if os.IsNotExist(err) {
				return nil
			}
			matches = append(matches, path)
			return filepath.SkipDir
		})
		if err != nil {
			return nil, err
		}
	}
	return matches, nil
}

func copyPackageFromSource(src, dst string) error {
	err := os.MkdirAll(dst, 0755)
	if err != nil {
		return err
	}
	err = sh.RunV("rsync", "-a", src, dst)
	if err != nil {
		return err
	}

	return nil
}

func Check() error {
	Format()

	// Setup the variables for the tests and not create tarGz files
	publicDir = "./testdata/public"
	packagePaths = []string{"testdata/package"}
	tarGz = false

	err := Build()
	if err != nil {
		return err
	}

	err = Vendor()
	if err != nil {
		return err
	}

	err = PrepareTest()
	if err != nil {
		return err
	}

	// Check if no changes are shown
	err = sh.RunV("git", "update-index", "--refresh")
	if err != nil {
		return err
	}
	return sh.RunV("git", "diff-index", "--exit-code", "HEAD", "--")
}

func PrepareTest() error {
	return sh.RunV("go", "get", "-v", "-u", "github.com/jstemmer/go-junit-report")
}

func Test() error {
	err := PrepareTest()
	if err != nil {
		return err
	}
	return sh.RunV("go", "test", "./...", "-v", "2>&1", "|", "go-junit-report", ">", "junit-report.xml")
}

func TestIntegration() error {
	// Build is need to make sure all packages are built
	err := Build()
	if err != nil {
		return err
	}

	// Checks if the binary is properly running and does not return any errors
	_, err = sh.Output("go", "run", ".", "-dry-run=true")
	if err != nil {
		return err
	}

	return sh.RunV("go", "test", "./...", "-v", "-tags=integration", "2>&1", "|", "go-junit-report", ">", "junit-report.xml")
}

// Format adds license headers, formats .go files with goimports, and formats
// .py files with autopep8.
func Format() {
	// Don't run AddLicenseHeaders and GoImports concurrently because they
	// both can modify the same files.
	mg.Deps(AddLicenseHeaders)
	mg.Deps(GoImports)
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
	return os.RemoveAll("package-registry")
}

func Vendor() error {
	fmt.Println(">> mod - updating vendor directory")

	err := sh.RunV("go", "mod", "tidy")
	if err != nil {
		return err
	}

	sh.RunV("go", "mod", "vendor")
	if err != nil {
		return err
	}

	sh.RunV("go", "mod", "verify")
	if err != nil {
		return err
	}
	return nil
}
