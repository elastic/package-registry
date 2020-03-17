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

type elasticsearchContent struct {
	ingestPipelines []ingestPipelineContent
}

type ingestPipelineContent struct {
	source string
}

func loadElasticsearchContent(datasetPath string) (elasticsearchContent, error) {
	var esc elasticsearchContent
	ingestPath := path.Join(datasetPath, "ingest")
	ingestFiles, err := ioutil.ReadDir(ingestPath)
	if os.IsNotExist(err) {
		log.Printf("No ingest pipelines defined. Skipping. (path: %s)\n", ingestPath)
		return elasticsearchContent{}, nil
	} else if err != nil {
		return elasticsearchContent{}, errors.Wrapf(err, "cannot read ingest directory (path: %s)", ingestPath)
	}

	for _, ingestFile := range ingestFiles {
		log.Printf("\tingest-pipeline found: %s", ingestFile.Name())
		esc.ingestPipelines = append(esc.ingestPipelines, ingestPipelineContent{
			source: path.Join(ingestPath, ingestFile.Name()),
		})
	}
	return esc, nil
}
