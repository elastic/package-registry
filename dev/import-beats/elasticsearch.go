// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type elasticsearchContent struct {
	ingestPipelines []ingestPipelineContent
}

type ingestPipelineContent struct {
	targetFileName string
	body           []byte
}

func loadElasticsearchContent(datasetPath string) (elasticsearchContent, error) {
	var esc elasticsearchContent

	datasetManifestPath := filepath.Join(datasetPath, "manifest.yml")
	datasetManifestFile, err := ioutil.ReadFile(datasetManifestPath)
	if os.IsNotExist(err) {
		return elasticsearchContent{}, nil // no manifest.yml file found,
	}
	if err != nil {
		return elasticsearchContent{}, errors.Wrapf(err, "reading dataset manifest file failed (path: %s)", datasetManifestPath)
	}

	var ingestPipelines []string
	var dmsp datasetManifestSinglePipeline
	err = yaml.Unmarshal(datasetManifestFile, &dmsp)
	if err == nil {
		if len(dmsp.IngestPipeline) > 0 {
			ingestPipelines = append(ingestPipelines, dmsp.IngestPipeline)
		}
	} else {
		var dmmp datasetManifestMultiplePipelines
		err = yaml.Unmarshal(datasetManifestFile, &dmmp)
		if err != nil {
			return elasticsearchContent{}, errors.Wrapf(err, "unmarshalling dataset manifest file failed (path: %s)", datasetManifestPath)
		}

		if len(dmmp.IngestPipeline) > 0 {
			ingestPipelines = append(ingestPipelines, dmmp.IngestPipeline...)
		}
	}

	for _, ingestPipeline := range ingestPipelines {
		ingestPipeline = ensurePipelineFormat(ingestPipeline)

		log.Printf("\tingest-pipeline found: %s", ingestPipeline)

		var targetFileName string
		if len(ingestPipelines) == 1 {
			targetFileName, err = buildSingleIngestPipelineTargetName(ingestPipeline)
			if err != nil {
				return elasticsearchContent{}, errors.Wrapf(err, "can't build single ingest pipeline target name (path: %s)", ingestPipeline)
			}
		} else {
			targetFileName, err = determineIngestPipelineTargetName(ingestPipeline)
			if err != nil {
				return elasticsearchContent{}, errors.Wrapf(err, "can't determine ingest pipeline target name (path: %s)", ingestPipeline)
			}
		}

		body, err := ioutil.ReadFile(filepath.Join(datasetPath, ingestPipeline))
		if err != nil {
			return elasticsearchContent{}, errors.Wrapf(err, "reading pipeline body failed")
		}

		// Fix missing "---" at the beginning of the YAML pipeline.
		if strings.HasSuffix(targetFileName, ".yml") && bytes.Index(body, []byte("---")) != 0 {
			body = append([]byte("---\n"), body...)
		}

		esc.ingestPipelines = append(esc.ingestPipelines, ingestPipelineContent{
			targetFileName: targetFileName,
			body:           body,
		})
	}

	return esc, nil
}

func buildSingleIngestPipelineTargetName(path string) (string, error) {
	lastDot := strings.LastIndex(path, ".")
	if lastDot == -1 {
		return "", fmt.Errorf("ingest pipeline file must have an extension")
	}
	fileExt := path[lastDot+1:]
	return "default." + fileExt, nil
}

func ensurePipelineFormat(ingestPipeline string) string {
	if strings.Contains(ingestPipeline, "{{.format}}") {
		ingestPipeline = strings.ReplaceAll(ingestPipeline, "{{.format}}", "json")
	}
	return ingestPipeline
}

func determineIngestPipelineTargetName(path string) (string, error) {
	fileName := path
	if strings.Contains(path, "/") {
		fileName = path[strings.LastIndex(path, "/")+1:]
	}

	lastDot := strings.LastIndex(fileName, ".")
	if lastDot == -1 {
		return "", fmt.Errorf("ingest pipeline file must have an extension")
	}
	fileNameWithoutExt := fileName[:lastDot]
	fileExt := fileName[lastDot+1:]

	if fileNameWithoutExt == "pipeline" || fileNameWithoutExt == "pipeline-entry" {
		return "default." + fileExt, nil
	}
	return fileName, nil
}
