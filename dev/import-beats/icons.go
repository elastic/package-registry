// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

const tutorialsPath = "src/plugins/home/server/tutorials"

var (
	errIconNotFound = errors.New("icon not found")
	iconRe          = regexp.MustCompile(`euiIconType: '[^']+'`)
)

type iconRepository struct {
	icons map[string]string
}

func newIconRepository(euiDir, kibanaDir string) (*iconRepository, error) {
	icons, err := populateIconRepository(euiDir, kibanaDir)
	if err != nil {
		return nil, errors.Wrapf(err, "populating icon repository failed")
	}
	return &iconRepository{icons: icons}, nil
}

func populateIconRepository(euiDir, kibanaDir string) (map[string]string, error) {
	log.Println("Populate icon registry")

	iconRefs, err := fetchIconReferencesFromTutorials(kibanaDir)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching icon references failed")
	}

	data, err := collectIconData(iconRefs, euiDir, kibanaDir)
	if err != nil {
		return nil, errors.Wrapf(err, "collecting icon data failed")
	}
	return data, nil
}

func fetchIconReferencesFromTutorials(kibanaDir string) (map[string]string, error) {
	refs := map[string]string{}

	tutorialsPath := filepath.Join(kibanaDir, "src/plugins/home/server/tutorials")
	tutorialFilePaths, err := filepath.Glob(filepath.Join(tutorialsPath, "*_*", "index.ts"))
	if err != nil {
		return nil, errors.Wrapf(err, "globbing tutorial files failed (path: %s)", tutorialsPath)
	}

	for _, tutorialFilePath := range tutorialFilePaths {
		log.Printf("Scan tutorial file: %s", tutorialFilePath)

		tutorialFile, err := ioutil.ReadFile(tutorialFilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading tutorial file failed (path: %s)", tutorialFile)
		}

		m := iconRe.Find(tutorialFile)
		if m == nil {
			log.Printf("\t%s: icon not found", tutorialFilePath)
			continue
		}

		s := strings.Split(string(m), `'`)

		// Extracting module name from tutorials path
		// e.g. ./src/plugins/home/server/tutorials//php_fpm_metrics/index.ts -> php_fpm
		moduleName := tutorialFilePath[len(tutorialsPath)+1:]
		moduleName = moduleName[:strings.Index(moduleName, "/")]
		moduleName = moduleName[:strings.LastIndex(moduleName, "_")]
		refs[moduleName] = s[1]
	}
	return refs, nil
}

func collectIconData(refs map[string]string, euiDir, kibanaDir string) (map[string]string, error) {
	for k, v := range refs {
		log.Println(k, v)
	}
	log.Fatal(1)
	// TODO
	return nil, nil
}

func (ir *iconRepository) iconForModule(moduleName string) (imageContent, error) {
	source, ok := ir.icons[moduleName]
	if !ok {
		return imageContent{}, errIconNotFound
	}
	return imageContent{source: source}, nil
}

func createIcons(iconRepository *iconRepository, moduleName string) ([]imageContent, error) {
	anIcon, err := iconRepository.iconForModule(moduleName)
	if err == errIconNotFound {
		return []imageContent{}, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "fetching icon for module failed (moduleName: %s)", moduleName)
	}
	return []imageContent{anIcon}, nil
}
