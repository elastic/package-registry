// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package ingestionmethod

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v2"
)

// IngestionMethod is a map of input types to their ingestion methods
type IngestionMethod map[string]string

// ReadIngestionMethods reads the ingestion method mappings from a reader
func ReadIngestionMethod(r io.Reader) (IngestionMethod, error) {
	var methodsFile struct {
		Mappings map[string]string `yaml:"mappings"`
	}

	dec := yaml.NewDecoder(r)
	err := dec.Decode(&methodsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ingestion methods: %w", err)
	}

	return IngestionMethod(methodsFile.Mappings), nil
}

// MustReadIngestionMethods reads the ingestion methods from a reader and panics if there is any error
func MustReadIngestionMethod(r io.Reader) IngestionMethod {
	methods, err := ReadIngestionMethod(r)
	if err != nil {
		panic(err)
	}
	return methods
}

// Get returns the ingestion method for a given input type
// Returns empty string if no mapping exists
func (im IngestionMethod) Get(inputType string) string {
	if method, ok := im[inputType]; ok {
		return method
	}
	return ""
}
