// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/package-registry/util"
)

type datasetContent struct {
	name     string
	beatType string

	manifest util.DataSet

	agent         agentContent
	elasticsearch elasticsearchContent
	fields        fieldsContent
}

type datasetContentArray []datasetContent

func (dca datasetContentArray) names() []string {
	var names []string
	for _, dc := range dca {
		names = append(names, dc.name)
	}
	return names
}

type datasetManifestMultiplePipelines struct {
	IngestPipeline []string `yaml:"ingest_pipeline"`
}

type datasetManifestSinglePipeline struct {
	IngestPipeline string `yaml:"ingest_pipeline"`
}

func createDatasets(modulePath, moduleName, moduleRelease, beatType string) (datasetContentArray, error) {
	moduleFieldsFiles, err := loadModuleFields(modulePath)
	if err != nil {
		return nil, errors.Wrapf(err, "loading module fields failed (modulePath: %s)", modulePath)
	}

	datasetDirs, err := ioutil.ReadDir(modulePath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read module directory %s", modulePath)
	}

	var contents []datasetContent
	for _, datasetDir := range datasetDirs {
		if !datasetDir.IsDir() {
			continue
		}
		datasetName := datasetDir.Name()

		if datasetName == "_meta" {
			continue
		}

		datasetPath := filepath.Join(modulePath, datasetName)
		_, err := os.Stat(filepath.Join(datasetPath, "_meta"))
		if os.IsNotExist(err) {
			log.Printf("\t%s: not a valid dataset, skipped", datasetName)
			continue
		}

		log.Printf("\t%s: dataset found", datasetName)

		// release
		datasetRelease, err := determineDatasetRelease(moduleRelease, datasetPath)
		if err != nil {
			return nil, errors.Wrapf(err, "loading release from fields failed (datasetPath: %s", datasetPath)
		}

		// fields
		fieldsFiles, err := loadDatasetFields(modulePath, moduleName, datasetName)
		if err != nil {
			return nil, errors.Wrapf(err, "loading dataset fields failed (modulePath: %s, datasetName: %s)",
				modulePath, datasetName)
		}

		fields := fieldsContent{
			files: map[string][]byte{
				"package-fields.yml": moduleFieldsFiles,
				"fields.yml":         fieldsFiles,
			},
		}

		// elasticsearch
		elasticsearch, err := loadElasticsearchContent(datasetPath)
		if err != nil {
			return nil, errors.Wrapf(err, "loading elasticsearch content failed (datasetPath: %s)", datasetPath)
		}

		// streams
		streams, err := createStreams(modulePath, moduleName, datasetName, beatType)
		if err != nil {
			return nil, errors.Wrapf(err, "creating streams failed (datasetPath: %s)", datasetPath)
		}

		// agent
		agent, err := createAgentContent(modulePath, moduleName, datasetName, beatType)
		if err != nil {
			return nil, errors.Wrapf(err, "creating agent content failed (modulePath: %s, datasetName: %s)",
				modulePath, datasetName)
		}

		// manifest
		manifest := util.DataSet{
			ID:      datasetName,
			Title:   strings.Title(fmt.Sprintf("%s %s %s", moduleName, datasetName, beatType)),
			Release: datasetRelease,
			Type:    beatType,
			Streams: streams,
		}

		contents = append(contents, datasetContent{
			name:          datasetName,
			beatType:      beatType,
			manifest:      manifest,
			agent:         agent,
			elasticsearch: elasticsearch,
			fields:        fields,
		})
	}
	return contents, nil
}
