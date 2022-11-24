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

	if assert.Len(t, categories, 3) {
		assert.Len(t, categories["security"].SubCategories, 2)
	}

	titles := categories.TitlesMap()
	assert.Equal(t, "Security", titles["security"])
	assert.Equal(t, "Web Server", titles["webserver"])
}
