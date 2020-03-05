// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/elastic/package-registry/util"
)

var ignoredModules = map[string]bool{"apache2": true}

type packageContent struct {
	manifest util.Package
	datasets map[string]datasetContent
}

func newPackageContent(name string) packageContent {
	title := strings.Title(name)
	return packageContent{
		manifest: util.Package{
			FormatVersion: "1.0.0",
			Name:          name,
			Description:   strings.Title(name + " integration"),
			Title:         &title,
			Version:       "0.0.1", // TODO
			Type:          "integration",
			License:       "basic",
		},
		datasets: map[string]datasetContent{},
	}
}

type packageRepository struct {
	packages map[string]packageContent
}

func newPackageRepository() *packageRepository {
	return &packageRepository{
		packages: map[string]packageContent{},
	}
}

func (r *packageRepository) createPackagesFromSource(beatsDir, beatName, packageType string) error {
	beatPath := filepath.Join(beatsDir, beatName)
	beatModulesPath := filepath.Join(beatPath, "module")

	moduleDirs, err := ioutil.ReadDir(beatModulesPath)
	if err != nil {
		return errors.Wrapf(err, "cannot read directory '%s'", beatModulesPath)
	}

	for _, moduleDir := range moduleDirs {
		if !moduleDir.IsDir() {
			continue
		}
		moduleName := moduleDir.Name()

		log.Printf("Found module '%s:%s'\n", beatName, moduleName)
		if _, ok := ignoredModules[moduleName]; ok {
			log.Printf("Ignoring '%s:%s'\n", beatName, moduleName)
			continue
		}

		_, ok := r.packages[moduleName]
		if !ok {
			r.packages[moduleName] = newPackageContent(moduleName)
		}

		aPackage := r.packages[moduleName]
		manifest := aPackage.manifest
		manifest.Categories = append(manifest.Categories, packageType)
		aPackage.manifest = manifest

		modulePath := path.Join(beatModulesPath, moduleName)
		datasets, err := createDatasets(modulePath)
		if err != nil {
			return err
		}

		aPackage.datasets = datasets
		r.packages[moduleDir.Name()] = aPackage
	}
	return nil
}

func (r *packageRepository) save(outputDir string) error {
	for packageName, content := range r.packages {
		manifest := content.manifest

		log.Printf("Writing package data '%s' (version: %s)\n", packageName, manifest.Version)

		packagePath := filepath.Join(outputDir, packageName+"-"+manifest.Version)
		err := os.MkdirAll(packagePath, 0755)
		if err != nil {
			return errors.Wrapf(err, "cannot make directory for module: '%s'", packagePath)
		}

		m, err := yaml.Marshal(content.manifest)
		if err != nil {
			return errors.Wrapf(err, "marshaling package content failed (package packageName: %s)", packageName)
		}

		manifestFilePath := filepath.Join(packagePath, "manifest.yml")
		err = ioutil.WriteFile(manifestFilePath, m, 0644)
		if err != nil {
			return errors.Wrapf(err, "writing manifest file failed (path: %s)", manifestFilePath)
		}

		for datasetName, dataset := range content.datasets {
			datasetPath := filepath.Join(packagePath, "dataset", datasetName)
			err := os.MkdirAll(datasetPath, 0755)
			if err != nil {
				return errors.Wrapf(err, "cannot make directory for dataset: '%s'", datasetPath)
			}

			if len(dataset.fields.files) > 0 {
				datasetFieldsPath := filepath.Join(datasetPath, "fields")
				err := os.MkdirAll(datasetFieldsPath, 0755)
				if err != nil {
					return errors.Wrapf(err, "cannot make directory for dataset fields: '%s'", datasetPath)
				}

				for fieldsFileName, fieldsFile := range dataset.fields.files {
					log.Printf("\tWriting file '%s' for dataset '%s'\n", fieldsFileName, datasetName)

					fieldsFilePath := filepath.Join(datasetFieldsPath, fieldsFileName)
					err = ioutil.WriteFile(fieldsFilePath, fieldsFile, 0644)
					if err != nil {
						return errors.Wrapf(err, "writing fields file failed (path: %s)", fieldsFilePath)
					}
				}
			}
		}
	}
	return nil
}
