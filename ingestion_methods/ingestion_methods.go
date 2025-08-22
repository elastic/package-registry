// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingestion_methods

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v2"
)

// IngestionMethods is a map of input types to their ingestion methods
type IngestionMethods map[string]string

// ReadIngestionMethods reads the ingestion method mappings from a reader
func ReadIngestionMethods(r io.Reader) (IngestionMethods, error) {
	var methodsFile struct {
		Mappings map[string]string `yaml:"mappings"`
	}

	dec := yaml.NewDecoder(r)
	err := dec.Decode(&methodsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ingestion methods: %w", err)
	}

	return IngestionMethods(methodsFile.Mappings), nil
}

// MustReadIngestionMethods reads the ingestion methods from a reader and panics if there is any error
func MustReadIngestionMethods(r io.Reader) IngestionMethods {
	methods, err := ReadIngestionMethods(r)
	if err != nil {
		panic(err)
	}
	return methods
}

// Get returns the ingestion method for a given input type
// Returns empty string if no mapping exists
func (im IngestionMethods) Get(inputType string) string {
	if method, ok := im[inputType]; ok {
		return method
	}
	return ""
}
