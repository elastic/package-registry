// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/pkg/errors"
)

type datasetContent struct {
	fields        fieldsContent
	elasticsearch elasticsearchContent
}

func createDatasets(modulePath, moduleName string) (map[string]datasetContent, error) {
	moduleFieldsFiles, err := loadModuleFields(modulePath)
	if err != nil {
		return nil, errors.Wrapf(err, "loading module fields failed (modulePath: %s)", modulePath)
	}

	datasetDirs, err := ioutil.ReadDir(modulePath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read module directory %s", modulePath)
	}

	contents := map[string]datasetContent{}
	for _, datasetDir := range datasetDirs {
		if !datasetDir.IsDir() {
			continue
		}
		datasetName := datasetDir.Name()

		if datasetName == "_meta" {
			continue
		}

		datasetPath := path.Join(modulePath, datasetName)
		_, err := os.Stat(path.Join(datasetPath, "_meta"))
		if os.IsNotExist(err) {
			log.Printf("\t%s: not a valid dataset, skipped", datasetName)
			continue
		}

		log.Printf("\t%s: dataset found", datasetName)
		content := datasetContent{}

		fieldsFiles, err := loadDatasetFields(modulePath, moduleName, datasetName)
		if err != nil {
			return nil, errors.Wrapf(err, "loading dataset fields failed (modulePath: %s, datasetName: %s)",
				modulePath, datasetName)
		}
		content.fields = fieldsContent{
			files: map[string][]byte{
				"package-fields.yml": moduleFieldsFiles,
				"fields.yml":         fieldsFiles,
			},
		}

		elasticsearch, err := loadElasticsearchContent(datasetPath)
		if err != nil {
			return nil, errors.Wrapf(err, "loading elasticsearc content failed (datasetPath: %s)", datasetPath)
		}
		content.elasticsearch = elasticsearch

		contents[datasetName] = content
	}
	return contents, nil
}
