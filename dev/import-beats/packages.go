// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"io/ioutil"
	"log"
	"os"
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

type datasetContent struct {
	fields fieldsContent
}

type fieldsContent struct {
	files map[string]fields
}

type packageRepository struct {
	packages map[string]packageContent
}

func newPackageRepository() *packageRepository {
	return &packageRepository{
		packages: map[string]packageContent{},
	}
}

func (r *packageRepository) loadFromSource(beatsDir, beatName, packageType string) error {
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

		log.Printf("Visit '%s:%s'\n", beatName, moduleDir.Name())
		if _, ok := ignoredModules[moduleDir.Name()]; ok {
			log.Printf("Ignoring '%s:%s'\n", beatName, moduleDir.Name())
			continue
		}

		_, ok := r.packages[moduleDir.Name()]
		if !ok {
			r.packages[moduleDir.Name()] = newPackageContent(moduleDir.Name())
		}

		aPackage := r.packages[moduleDir.Name()]
		manifest := aPackage.manifest
		manifest.Categories = append(manifest.Categories, packageType)
		aPackage.manifest = manifest
		r.packages[moduleDir.Name()] = aPackage
	}
	return nil
}

func (r *packageRepository) save(outputDir string) error {
	for name, content := range r.packages {
		manifest := content.manifest

		log.Printf("Writing package '%s' (version: %s)\n", name, manifest.Version)

		path := filepath.Join(outputDir, name+"-"+manifest.Version)
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return errors.Wrapf(err, "cannot make directory '%s'", path)
		}

		m, err := yaml.Marshal(content.manifest)
		if err != nil {
			return errors.Wrapf(err, "marshaling package content failed (package name: %s)", name)
		}

		manifestFilePath := filepath.Join(path, "manifest.yml")
		err = ioutil.WriteFile(manifestFilePath, m, 0644)
		if err != nil {
			return errors.Wrapf(err, "writing manifest file failed (path: %s)", manifestFilePath)
		}
	}
	return nil
}
