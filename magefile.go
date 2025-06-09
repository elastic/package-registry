// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	// GoImportsImportPath controls the import path used to install goimports.
	GoImportsImportPath = "golang.org/x/tools/cmd/goimports"

	// GoImportsLocalPrefix is a string prefix matching imports that should be
	// grouped after third-party packages.
	GoImportsLocalPrefix = "github.com/elastic"

	// GoLicenserImportPath controls the import path used to install go-licenser.
	GoLicenserImportPath = "github.com/elastic/go-licenser"

	// StaticcheckImport path is the import path of the staticcheck tool.
	StaticcheckImportPath = "honnef.co/go/tools/cmd/staticcheck"

	buildDir = "./build"
)

func Build() error {
	return sh.Run("go", "build", ".")
}

// DockerBuild builds the Docker image for the package registry. It must be specified
// the docker tag to be used as an argument (e.g. main, latest).
func DockerBuild(tag string) error {
	contents, err := os.ReadFile(".go-version")
	if err != nil {
		return fmt.Errorf("failed to read .go-version: %w", err)
	}
	goVersion := strings.TrimSpace(string(contents))
	if goVersion == "" {
		return fmt.Errorf("empty go version in .go-version")
	}
	dockerImage := fmt.Sprintf("docker.elastic.co/package-registry/package-registry:%s", tag)

	fmt.Println(">> Building Docker image:", dockerImage)
	err = sh.Run("docker", "build", "--build-arg", fmt.Sprintf("GO_VERSION=%s", goVersion), "-t", dockerImage, ".")
	if err != nil {
		return fmt.Errorf("failed to build docker image: %w", err)
	}
	return nil
}

func Check() error {
	mg.SerialDeps(
		Format,
		Build,
		ModTidy,
		Staticcheck,
	)

	// Check if no changes are shown
	err := sh.RunV("git", "update-index", "--refresh")
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
	mg.SerialDeps(
		AddLicenseHeaders,
		GoImports,
	)
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
	return sh.RunV("go", "run", GoLicenserImportPath, "-license", "Elasticv2")
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
	fmt.Println(">> fmt - go mod tidy: Generating go mod files")
	return sh.RunV("go", "mod", "tidy")
}

// Staticcheck runs a static code analyzer.
func Staticcheck() error {
	fmt.Println(">> check - staticcheck: Running static code analyzer")
	return sh.RunV("go", "run", StaticcheckImportPath, "./...")
}
