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
	return sh.Run("go", "build", ".")
}

var (
	// GoImportsImportPath controls the import path used to install goimports.
	GoImportsImportPath = "golang.org/x/tools/cmd/goimports"

	// GoImportsLocalPrefix is a string prefix matching imports that should be
	// grouped after third-party packages.
	GoImportsLocalPrefix = "github.com/elastic"

	// GoLicenserImportPath controls the import path used to install go-licenser.
	GoLicenserImportPath = "github.com/elastic/go-licenser"
)

// Format adds license headers, formats .go files with goimports, and formats
// .py files with autopep8.
func Format() {
	// Don't run AddLicenseHeaders and GoImports concurrently because they
	// both can modify the same files.
	mg.Deps(AddLicenseHeaders)
	mg.Deps(GoImports)
}

// BuildIntegrationPackages rebuilds the zip files inside packages
func BuildIntegrationPackages() error {

	currentPath, err := os.Getwd()
	if err != nil {
		return err
	}
	err = os.Chdir("./packages/")
	if err != nil {
		return err
	}
	defer os.Chdir(currentPath)

	packages, err := filepath.Glob("./*")
	if err != nil {
		return err
	}

	for _, p := range packages {
		info, err := os.Stat(p)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			continue
		}

		err = sh.RunV("zip", "-r", p+".zip", p+"/")
		if err != nil {
			return err
		}

		err = sh.RunV("tar", "cvzf", p+".tar.gz", filepath.Base(p)+"/")
		if err != nil {
			return err
		}
	}
	return nil
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
