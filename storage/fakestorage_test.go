// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareFakeServer(t *testing.T) {
	// given
	indexFile := "testdata/search-index-all-1.json"

	// when
	fs := prepareFakeServer(t, indexFile)
	defer fs.Stop()

	// then
	client := fs.Client()
	require.NotNil(t, client, "client should be initialized")

}
