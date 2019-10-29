// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"

	"github.com/elastic/package-registry/util"
)

var (
	tarGz bool
	copy  bool
)

const (
	packageDirName = "package"
)

func main() {
	// Directory with a list of packages inside
	var sourceDir string
	// Target public directory where the generated packages should end up in
	var publicDir string

	flag.StringVar(&sourceDir, "sourceDir", "", "Path to the source packages")
	flag.StringVar(&publicDir, "publicDir", "", "Path to the public directory ")
	flag.BoolVar(&copy, "copy", true, "If packages should be copied over")
	flag.BoolVar(&tarGz, "tarGz", true, "If packages should be tar gz")
	flag.Parse()

	if sourceDir == "" || publicDir == "" {
		log.Fatal("sourceDir and publicDir must be set")
		os.Exit(1)
	}

	if err := Build(sourceDir, publicDir); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func Build(sourceDir, publicDir string) error {

	err := BuildPackages(sourceDir, publicDir+"/"+packageDirName)
	if err != nil {
		return err
	}
	return nil
}

// CopyPackage copies the files of a package to the public directory
func CopyPackage(src, dst string) error {
	fmt.Println(">> Copy package: " + src)
	os.MkdirAll(dst, 0755)
	err := sh.RunV("cp", "-a", src, dst)
	if err != nil {
		return err
	}

	return nil
}

// BuildPackage rebuilds the zip files inside packages
func BuildPackages(sourceDir, packagesPath string) error {

	dirs, err := ioutil.ReadDir(sourceDir)
	if err != nil {
		return err
	}

	for _, d := range dirs {

		packageName := d.Name()
		if !d.IsDir() {
			continue
		}

		if copy {
			err := CopyPackage(sourceDir+"/"+packageName, packagesPath)
			if err != nil {
				return err
			}
		}

		p, err := util.NewPackage(packagesPath, packageName)
		if err != nil {
			return err
		}

		err = buildPackage(packagesPath, *p)
		if err != nil {
			return err
		}
	}
	return nil
}

func buildPackage(packagesBasePath string, p util.Package) error {

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

	if tarGz {
		err = sh.RunV("tar", "cvzf", p.GetPath()+".tar.gz", filepath.Base(p.GetPath())+"/")
		if err != nil {
			return fmt.Errorf("Error creating package: %s: %s", p.GetPath(), err)
		}
	}

	// Checks if the package is valid
	err = p.Validate()
	if err != nil {
		return fmt.Errorf("Invalid package: %s: %s", p.GetPath(), err)
	}

	err = p.LoadAssets(p.GetPath())
	if err != nil {
		return err
	}

	err = writeJsonFile(p, p.GetPath()+"/index.json")
	if err != nil {
		return err
	}
	return nil
}

func writeJsonFile(v interface{}, path string) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, 0644)
}
