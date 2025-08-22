// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingestion_method

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/sample.yml
var sampleIngestionMethodsYaml []byte

func TestLoadIngestionMethods(t *testing.T) {
	methods, err := ReadIngestionMethod(bytes.NewReader(sampleIngestionMethodsYaml))
	require.NoError(t, err)

	assert.Equal(t, "API", methods["httpjson"])
	assert.Equal(t, "API", methods["cel"])
	assert.Equal(t, "API", methods["apache/metrics"])

	assert.Equal(t, "Database", methods["redis"])
	assert.Equal(t, "Database", methods["mysql/metrics"])

	assert.Equal(t, "File", methods["filestream"])
	assert.Equal(t, "File", methods["system/metrics"])

	assert.Equal(t, "Network Protocol", methods["tcp"])
	assert.Equal(t, "Network Protocol", methods["udp"])

	assert.Equal(t, "Webhook", methods["http_endpoint"])
}

func TestIngestionMethodsGet(t *testing.T) {
	methods, err := ReadIngestionMethod(bytes.NewReader(sampleIngestionMethodsYaml))
	require.NoError(t, err)

	// Test existing mappings
	assert.Equal(t, "API", methods.Get("httpjson"))
	assert.Equal(t, "Database", methods.Get("redis"))
	assert.Equal(t, "File", methods.Get("filestream"))

	// Test non-existing mapping
	assert.Equal(t, "", methods.Get("nonexistent"))
	assert.Equal(t, "", methods.Get("unknown_input"))
}

func TestDefaultIngestionMethod(t *testing.T) {
	methods := DefaultIngestionMethod()
	assert.NotNil(t, methods)

	// Test a few known mappings from the default file
	assert.Equal(t, "API", methods.Get("httpjson"))
	assert.Equal(t, "Database", methods.Get("redis"))
	assert.Equal(t, "File", methods.Get("filestream"))
	assert.Equal(t, "Network Protocol", methods.Get("tcp"))
}

func TestMustReadIngestionMethodsPanics(t *testing.T) {
	invalidYAML := []byte("invalid: yaml: content:")

	assert.Panics(t, func() {
		MustReadIngestionMethod(bytes.NewReader(invalidYAML))
	})
}
