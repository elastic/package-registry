// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DataSet struct {
	ID             string                   `config:"id" json:"id"`
	Title          string                   `config:"title" json:"title" validate:"required"`
	Release        string                   `config:"release" json:"release"`
	Type           string                   `config:"type" json:"type" validate:"required"`
	IngestPipeline string                   `config:"ingest_pipeline,omitempty" config:"ingest_pipeline" json:"ingest_pipeline,omitempty"`
	Vars           []map[string]interface{} `config:"vars" json:"vars,omitempty"`
	Streams        []Stream                 `config:"streams" json:"streams,omitempty"`
	Package        string                   `json:"package"`

	// Generated fields
	Path string `json:"path"`
}

type Input struct {
	Type        string                   `config:"type" json:"type" validate:"required"`
	Vars        []map[string]interface{} `config:"vars" json:"vars,omitempty" `
	Description string                   `config:"description" json:"description,omitempty" `
	Streams     []Stream                 `config:"streams" json:"streams,omitempty"`
}

type Stream struct {
	Input       string                   `config:"input" json:"input" validate:"required"`
	Vars        []map[string]interface{} `config:"vars" json:"vars,omitempty" `
	Dataset     string                   `config:"dataset" json:"dataset,omitempty" `
	Description string                   `config:"description" json:"description,omitempty" `
}

func (d *DataSet) Validate() error {
	pipelineDir := d.Path + "/elasticsearch/ingest-pipeline/"
	paths, err := filepath.Glob(pipelineDir + "*")
	if err != nil {
		return err
	}

	if strings.Contains(d.ID, "-") {
		return fmt.Errorf("dataset name is not allowed to contain `-`: %s", d.ID)
	}

	if d.IngestPipeline == "" {
		// Check that no ingest pipeline exists in the directory except default
		for _, path := range paths {
			if filepath.Base(path) == "default.json" || filepath.Base(path) == "default.yml" {
				d.IngestPipeline = "default"
				break
			}
		}
	}

	if d.IngestPipeline == "" && len(paths) > 0 {
		return fmt.Errorf("Package contains pipelines which are not used: %v, %s", paths, d.ID)
	}

	// In case an ingest pipeline is set, check if it is around
	if d.IngestPipeline != "" {
		_, errJSON := os.Stat(pipelineDir + d.IngestPipeline + ".json")
		_, errYAML := os.Stat(pipelineDir + d.IngestPipeline + ".yml")

		if os.IsNotExist(errYAML) && os.IsNotExist(errJSON) {
			return fmt.Errorf("Defined ingest_pipeline does not exist: %s", pipelineDir+d.IngestPipeline)
		}
	}
	return nil
}
