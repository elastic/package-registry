// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

//go:build mage

package main

import (
	"errors"
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

	// GOFIPS140Version pins the certified Go FIPS 140-3 crypto module used by
	// FIPS builds. See https://go.dev/doc/security/fips140#fips-140-3-mode and docs/fips.md.
	GOFIPS140Version = "v1.0.0"
)

type module struct {
	name string // Display name
	path string // Relative path from repo root
}

// modules is the list of Go modules in this repository.
// Add new modules here to automatically include them in test, lint, and other targets.
var modules = []module{
	{name: "package-registry", path: "."},
	{name: "cmd/distribution", path: "cmd/distribution"},
}

func runInAllModules(fn func(mod module) error) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	defer func() {
		err := os.Chdir(wd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to change directory to working directory: %s\n", err)
			panic(err)
		}
	}()
	for _, mod := range modules {
		err = os.Chdir(filepath.Join(wd, mod.path))
		if err != nil {
			return fmt.Errorf("failed to change directory to %s: %w", mod.path, err)
		}
		err = fn(mod)
		if err != nil {
			return fmt.Errorf("%s failed: %w", mod.name, err)
		}
	}
	return nil
}

func Build() error {
	fmt.Println(">> Building package-registry")
	return sh.Run("go", "build", ".")
}

// BuildFIPS builds package-registry against the certified Go FIPS 140-3
// crypto module. See docs/fips.md.
func BuildFIPS() error {
	fmt.Println(">> Building package-registry (FIPS 140-3)")
	return sh.RunWith(map[string]string{"GOFIPS140": GOFIPS140Version}, "go", "build", ".")
}

// BuildDistribution builds the distribution binary in cmd/distribution.
func BuildDistribution() error {
	return runInAllModules(func(mod module) error {
		if mod.name != "cmd/distribution" {
			return nil
		}
		fmt.Fprintf(os.Stderr, ">> Building cmd/distribution\n")
		return sh.RunV("go", "build", "-o", "distribution", ".")
	})
}

// DockerBuild builds the Docker image for the package registry. It must be specified
// the docker tag to be used as an argument (e.g. main, latest).
func DockerBuild(tag string) error {
	return dockerBuild(tag, false)
}

// DockerBuildFIPS builds the Docker image for the package registry against the
// certified Go FIPS 140-3 crypto module. See docs/fips.md.
func DockerBuildFIPS(tag string) error {
	return dockerBuild(tag, true)
}

func dockerBuild(tag string, fips bool) error {
	contents, err := os.ReadFile(".go-version")
	if err != nil {
		return fmt.Errorf("failed to read .go-version: %w", err)
	}
	goVersion := strings.TrimSpace(string(contents))
	if goVersion == "" {
		return fmt.Errorf("empty go version in .go-version")
	}
	dockerImage := fmt.Sprintf("docker.elastic.co/package-registry/package-registry:%s", tag)

	args := []string{"build", "--rm", "--build-arg", fmt.Sprintf("GO_VERSION=%s", goVersion)}
	if fips {
		dockerImage += "-fips"
		args = append(args, "--build-arg", "FIPS=1")
	}
	args = append(args, "-t", dockerImage, ".")

	fmt.Println(">> Building Docker image:", dockerImage)
	if err := sh.Run("docker", args...); err != nil {
		return fmt.Errorf("failed to build docker image: %w", err)
	}
	return nil
}

func Check() error {
	mg.SerialDeps(
		Format,
		Build,
		BuildDistribution,
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
	return runInAllModules(func(mod module) error {
		fmt.Fprintf(os.Stderr, ">> test - running tests for %s\n", mod.name)
		return sh.RunV("go", "test", "./...", "-v")
	})
}

// TestFIPS runs the package-registry test suite compiled against the certified
// Go FIPS 140-3 crypto module (GOFIPS140) under strict GODEBUG=fips140=only.
// See docs/fips.md.
func TestFIPS() error {
	fmt.Fprintf(os.Stderr, ">> test - running FIPS 140-3 tests for package-registry (GODEBUG=fips140=only)\n")
	return sh.RunWithV(map[string]string{"GOFIPS140": GOFIPS140Version, "GODEBUG": "fips140=only"}, "go", "test", "./...", "-v")
}

func WriteTestGoldenFiles() error {
	errMain := sh.RunV("go", "test", ".", "-v", "-generate")
	errPackages := sh.RunV("go", "test", "./packages/...", "-v", "-generate")
	err := errors.Join(errMain, errPackages)
	return err
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

	// Clean main package-registry binary
	err = os.RemoveAll("package-registry")
	if err != nil {
		return err
	}

	// Clean distribution binary
	err = os.RemoveAll("cmd/distribution/distribution")
	if err != nil {
		return err
	}

	return nil
}

// ModTidy cleans unused dependencies.
func ModTidy() error {
	return runInAllModules(func(mod module) error {
		fmt.Fprintf(os.Stderr, ">> fmt - go mod tidy: Generating go mod files for %s\n", mod.name)
		return sh.RunV("go", "mod", "tidy")
	})
}

// Staticcheck runs a static code analyzer.
func Staticcheck() error {
	return runInAllModules(func(mod module) error {
		fmt.Fprintf(os.Stderr, ">> check - staticcheck: Running static code analyzer on %s\n", mod.name)
		return sh.RunV("go", "run", StaticcheckImportPath, "./...")
	})
}
