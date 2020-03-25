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
	"strings"

	"github.com/magefile/mage/sh"

	"github.com/elastic/package-registry/util"
)

var (
	tarGz bool
	copy  bool
)

const (
	packageDirName = "package"
	streamFields   = `
- name: stream.type
  type: constant_keyword
  description: >
    Stream type
- name: stream.dataset
  type: constant_keyword
  description: >
    Stream dataset.
- name: stream.namespace
  type: constant_keyword
  description: >
    Stream namespace.
`
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

	err := BuildPackages(sourceDir, filepath.Join(publicDir, packageDirName))
	if err != nil {
		return err
	}
	return nil
}

// CopyPackage copies the files of a package to the public directory
func CopyPackage(src, dst string) error {
	log.Println(">> Copy package: " + src)
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

		// Finds the last occurence of "-" and then below splits it up in 2 parts.
		dashIndex := strings.LastIndex(packageName, "-")
		dstDir := filepath.Join(packagesPath, packageName[0:dashIndex], packageName[dashIndex+1:])

		if copy {
			// Trailing slash is to make sure content of package is copied
			err := CopyPackage(filepath.Join(sourceDir, packageName)+"/", dstDir)
			if err != nil {
				return err
			}
		}

		p, err := util.NewPackage(dstDir)
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

	// Checks if the package is valid
	err = p.Validate()
	if err != nil {
		return fmt.Errorf("Invalid package: %s: %s", p.GetPath(), err)
	}

	p.BasePath = filepath.Join(currentPath, packagesBasePath, p.GetPath())

	datasets, err := p.GetDatasets()
	if err != nil {
		return err
	}

	// Add stream.yml to all dataset with the basic stream fields
	for _, dataset := range datasets {
		dirPath := filepath.Join(p.BasePath, "dataset", dataset, "fields")
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(filepath.Join(dirPath, "stream.yml"), []byte(streamFields), 0644)
		if err != nil {
			return err
		}
	}

	err = p.LoadAssets(p.GetPath())
	if err != nil {
		return err
	}

	err = p.LoadDataSets(p.GetPath())
	if err != nil {
		return err
	}

	err = writeJsonFile(p, filepath.Join(packagesBasePath, p.GetPath(), "index.json"))
	if err != nil {
		return err
	}

	// Get all Kibana files
	savedObjects1, err := filepath.Glob(filepath.Join(packagesBasePath, p.GetPath(), "dataset", "*", "kibana", "*", "*"))
	if err != nil {
		return err
	}
	savedObjects2, err := filepath.Glob(filepath.Join(packagesBasePath, p.GetPath(), "kibana", "*", "*"))
	if err != nil {
		return err
	}
	savedObjects := append(savedObjects1, savedObjects2...)

	// Run each file through the saved object encoder
	for _, file := range savedObjects {

		data, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}
		output, err := encodedSavedObject(data)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(file, []byte(output), 0644)
		if err != nil {
			return err
		}
	}

	if tarGz {
		tarGzDirPath := filepath.Join(packagesBasePath, "..", "epr", p.Name)
		err = os.MkdirAll(tarGzDirPath, 0755)
		if err != nil {
			return err
		}

		tarGzName := p.Name + "-" + p.Version
		copiedPackagePath := filepath.Join(tarGzDirPath, tarGzName)

		// As the package directories are now {packagename}/{version} when just running tar, the dir inside
		// the package had the wrong name. Using `-s` or `--transform` for some reason worked on the command line
		// but not when run through Golang. So the hack for now is to just copy over all files with the correct name
		// and then run tar on it.
		// This could become even useful in the future as things like images or videos should potentially not be part of
		// a tar.gz to keep it small.
		err := CopyPackage(packagesBasePath+"/"+p.Name+"/"+p.Version+"/", copiedPackagePath)
		if err != nil {
			return err
		}

		err = sh.RunV("tar", "czf", filepath.Join(packagesBasePath, "..", "epr", p.Name, tarGzName+".tar.gz"), "-C", tarGzDirPath, tarGzName+"/")
		if err != nil {
			return fmt.Errorf("Error creating package: %s: %s", p.GetPath(), err)
		}

		err = os.RemoveAll(copiedPackagePath)
		if err != nil {
			return err
		}
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

var (
	fieldsToEncode = []string{
		"attributes.uiStateJSON",
		"attributes.visState",
		"attributes.optionsJSON",
		"attributes.panelsJSON",
		"attributes.kibanaSavedObjectMeta.searchSourceJSON",
	}
)

// encodeSavedObject encodes all the fields inside a saved object
// which are stored in encoded JSON in Kibana.
// The reason is that for versioning it is much nicer to have the full
// json so only on packaging this is changed.
func encodedSavedObject(data []byte) (string, error) {

	savedObject := MapStr{}
	json.Unmarshal(data, &savedObject)

	for _, v := range fieldsToEncode {
		out, err := savedObject.GetValue(v)
		// This means the key did not exists, no conversion needed
		if err != nil {
			continue
		}

		// Marshal the value to encode it properly
		r, err := json.Marshal(&out)
		if err != nil {
			return "", err
		}
		savedObject.Put(v, string(r))
	}

	return savedObject.StringToPrint(), nil
}
