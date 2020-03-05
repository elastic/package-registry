// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/elastic/package-registry/util"
)

type packageMap map[string]*util.Package

func (p packageMap) loadFromSource(beatsDir, beatName, packageType string) error {
	path := filepath.Join(beatsDir, beatName, "module")
	moduleDirs, err := ioutil.ReadDir(path)
	if err != nil {
		return errors.Wrapf(err, "cannot read directory '%s'", path)
	}

	for _, moduleDir := range moduleDirs {
		if !moduleDir.IsDir() {
			continue
		}

		log.Printf("Visit '%s:%s'\n", beatName, moduleDir.Name())

		_, ok := p[moduleDir.Name()]
		if !ok {
			p[moduleDir.Name()] = &util.Package{
				FormatVersion: "1.0.0",
				Name:          moduleDir.Name(),
				Version:       "0.0.1", // TODO
				Type:          "integration",
				Categories:    []string{},
			}
		}

		p[moduleDir.Name()].Categories = append(p[moduleDir.Name()].Categories, packageType)
	}
	return nil
}

func (p packageMap) writePackages(outputDir string) error {
	for name, content := range p {
		log.Printf("Writing package '%s' (version: %s)\n", name, content.Version)

		path := filepath.Join(outputDir, name+"-"+content.Version)
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return errors.Wrapf(err, "cannot make directory '%s'", path)
		}

		m, err := yaml.Marshal(content)
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
