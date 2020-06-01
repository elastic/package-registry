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

	"gopkg.in/yaml.v2"

	"github.com/magefile/mage/sh"
	"github.com/pkg/errors"

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
- name: "@timestamp"
  type: date
  description: >
    Event timestamp.
`
)

type fieldEntry struct {
	name  string
	aType string
}

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
	}

	if err := Build(sourceDir, publicDir); err != nil {
		log.Fatal(err)
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

func BuildPackages(sourceDir, packagesPath string) error {
	var matches []string
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

		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		matches = append(matches, relativePath)
		return filepath.SkipDir
	})
	if err != nil {
		return err
	}

	for _, packagePath := range matches {
		srcDir := filepath.Join(sourceDir, packagePath) + "/"

		p, err := util.NewPackage(srcDir)
		if err != nil {
			return err
		}

		dstDir := filepath.Join(packagesPath, p.Name, p.Version)

		if copy {
			// Trailing slash is to make sure content of package is copied
			err := CopyPackage(srcDir, dstDir)
			if err != nil {
				return err
			}
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
		return errors.Wrapf(err, "package validation failed (path: %s", p.GetPath())
	}

	p.BasePath = filepath.Join(currentPath, packagesBasePath, p.GetPath())

	datasets, err := p.GetDatasetPaths()
	if err != nil {
		return err
	}

	// Add base-fields.yml if missing to all dataset with the basic stream fields and @timestamp
	for _, dataset := range datasets {
		datasetPath := filepath.Join(p.BasePath, "dataset", dataset)

		err = createBaseFieldsFile(datasetPath)
		if err != nil {
			return err
		}
	}

	// Validate if basic stream fields and @timestamp are present
	for _, dataset := range datasets {
		datasetPath := filepath.Join(p.BasePath, "dataset", dataset)

		err = validateRequiredFields(datasetPath)
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
	return nil
}

// createBaseFieldsFile method creates the base-fields.yml file if missing.
func createBaseFieldsFile(datasetPath string) error {
	fieldsDirPath := filepath.Join(datasetPath, "fields")
	baseFieldsPath := filepath.Join(fieldsDirPath, "base-fields.yml")
	_, err := os.Stat(baseFieldsPath)
	if err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	err = os.MkdirAll(fieldsDirPath, 0755)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(baseFieldsPath, []byte(streamFields), 0644)
}

// validateRequiredFields method loads fields from all files and checks if required fields are present.
func validateRequiredFields(datasetPath string) error {
	fieldsDirPath := filepath.Join(datasetPath, "fields")

	// Collect fields from all files
	var allFields []MapStr
	err := filepath.Walk(fieldsDirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(fieldsDirPath, path)
		if err != nil {
			return err
		}

		if relativePath == "." {
			return nil
		}

		body, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		var m []MapStr
		err = yaml.Unmarshal(body, &m)
		if err != nil {
			return err
		}

		allFields = append(allFields, m...)
		return nil
	})
	if err != nil {
		return err
	}

	// Flatten all fields
	for i, fields := range allFields {
		allFields[i] = fields.Flatten()
	}

	// Verify required keys
	err = requireField(allFields, "stream.type", "constant_keyword", err)
	err = requireField(allFields, "stream.dataset", "constant_keyword", err)
	err = requireField(allFields, "stream.namespace", "constant_keyword", err)
	err = requireField(allFields, "@timestamp", "date", err)
	return err
}

func requireField(allFields []MapStr, searchedName, expectedType string, validationErr error) error {
	if validationErr != nil {
		return validationErr
	}

	f, err := findField(allFields, searchedName)
	if err != nil {
		return err
	}

	if f.aType != expectedType {
		return fmt.Errorf("wrong field type for '%s' (expected: %s, got: %s)", searchedName, expectedType, f.aType)
	}
	return nil
}

func findField(allFields []MapStr, searchedName string) (*fieldEntry, error) {
	for _, fields := range allFields {
		name, err := fields.GetValue("name")
		if err != nil {
			return nil, err
		}

		if name != searchedName {
			continue
		}

		aType, err := fields.GetValue("type")
		if err != nil {
			return nil, err
		}

		if aType == "" {
			return nil, fmt.Errorf("field '%s' found, but type is undefined", searchedName)
		}

		return &fieldEntry{
			name:  name.(string),
			aType: aType.(string),
		}, nil
	}
	return nil, fmt.Errorf("field '%s' not found", searchedName)
}

var (
	fieldsToEncode = []string{
		"attributes.kibanaSavedObjectMeta.searchSourceJSON",
		"attributes.layerListJSON",
		"attributes.mapStateJSON",
		"attributes.optionsJSON",
		"attributes.panelsJSON",
		"attributes.uiStateJSON",
		"attributes.visState",
	}
)

// encodeSavedObject encodes all the fields inside a saved object
// which are stored in encoded JSON in Kibana.
// The reason is that for versioning it is much nicer to have the full
// json so only on packaging this is changed.
func encodedSavedObject(data []byte) (string, error) {
	savedObject := MapStr{}
	err := json.Unmarshal(data, &savedObject)
	if err != nil {
		return "", errors.Wrapf(err, "unmarshalling saved object failed")
	}

	for _, v := range fieldsToEncode {
		out, err := savedObject.GetValue(v)
		// This means the key did not exists, no conversion needed
		if err != nil {
			continue
		}

		// It may happen that some objects existing in example directory might be already encoded.
		// In this case skip the encoding.
		_, isString := out.(string)
		if isString {
			return "", fmt.Errorf("expect non-string field type (fieldName: %s)", v)
		}

		// Marshal the value to encode it properly
		r, err := json.Marshal(&out)
		if err != nil {
			return "", err
		}
		_, err = savedObject.Put(v, string(r))
		if err != nil {
			return "", errors.Wrapf(err, "can't put value to the saved object")
		}

	}

	return savedObject.StringToPrint(), nil
}
