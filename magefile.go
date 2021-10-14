// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/pkg/errors"
)

var (
	// GoImportsImportPath controls the import path used to install goimports.
	GoImportsImportPath = "golang.org/x/tools/cmd/goimports"

	// GoImportsLocalPrefix is a string prefix matching imports that should be
	// grouped after third-party packages.
	GoImportsLocalPrefix = "github.com/elastic"

	// GoLicenserImportPath controls the import path used to install go-licenser.
	GoLicenserImportPath = "github.com/elastic/go-licenser"

	buildDir       = "./build"
	storageRepoDir = filepath.Join(buildDir, "package-storage")
	packagePaths   = []string{filepath.Join(storageRepoDir, "packages"), "./testdata/package/"}
)

func Build() error {
	err := FetchPackageStorage()
	if err != nil {
		return err
	}
	return sh.Run("go", "build", ".")
}

func FetchPackageStorage() error {
	// Remove old storage directory
	if err := os.RemoveAll(storageRepoDir); err != nil {
		return errors.Wrapf(err, "failed to remove existing storage directory: %s", storageRepoDir)
	}

	packageStorageRevision := os.Getenv("PACKAGE_STORAGE_REVISION")
	if packageStorageRevision == "" {
		packageStorageRevision = "production"
	}

	// Check out fresh storage directory
	return sh.Run("git", "clone", "--depth=1", "--single-branch",
		"--branch", packageStorageRevision, "https://github.com/elastic/package-storage.git", storageRepoDir)
}

func Check() error {
	Format()

	// Setup the variables for the tests and not create tarGz files
	packagePaths = []string{"testdata/package"}

	err := Build()
	if err != nil {
		return err
	}

	err = ModTidy()
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

func Test() error {
	return sh.RunV("go", "test", "./...", "-v")
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
		[]string{"run", GoImportsImportPath, "-local", GoImportsLocalPrefix, "-l", "-w"},
		goFiles...,
	)

	return sh.RunV("go", args...)
}

// AddLicenseHeaders adds license headers to .go files. It applies the
// appropriate license header based on the value of mage.BeatLicense.
func AddLicenseHeaders() error {
	fmt.Println(">> fmt - go-licenser: Adding missing headers")
	return sh.RunV("go", "run", GoLicenserImportPath, "-license", "Elastic")
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

	return os.RemoveAll("package-registry")
}

// ModTidy cleans unused dependencies.
func ModTidy() error {
	return sh.RunV("go", "mod", "tidy")
}
