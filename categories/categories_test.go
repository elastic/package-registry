// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package categories

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/sample.yml
var sampleCategoriesYaml []byte

func TestLoadCategories(t *testing.T) {
	categories, err := ReadCategories(bytes.NewReader(sampleCategoriesYaml))
	require.NoError(t, err)

	assert.Equal(t, "Security", categories["security"].Title)
	assert.Equal(t, "Web Server", categories["webserver"].Title)

	assert.Equal(t, "security", categories["edr_xdr"].SubcategoryOf)
	assert.Equal(t, "security", categories["network_security"].SubcategoryOf)
}
