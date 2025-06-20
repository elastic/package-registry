// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"io"
	"os"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareFakeServer(t *testing.T) {
	// given
	indexFile := "testdata/search-index-all-full.json"
	testIndexFile, err := os.ReadFile(indexFile)
	require.NoErrorf(t, err, "index file should be present in testdata")

	// when
	fs := PrepareFakeServer(t, indexFile)
	defer fs.Stop()

	// then
	client := fs.Client()
	require.NotNil(t, client, "client should be initialized")

	aCursor := readObject(t, client.Bucket(FakePackageStorageBucketInternal).Object(cursorStoragePath))
	assert.Equal(t, []byte(`{"current":"1"}`), aCursor)
	anIndex := readObject(t, client.Bucket(FakePackageStorageBucketInternal).Object(joinObjectPaths(v2MetadataStoragePath, "1", searchIndexAllFile)))
	assert.Equal(t, testIndexFile, anIndex)
}

func readObject(t *testing.T, handle *storage.ObjectHandle) []byte {
	reader, err := handle.NewReader(context.Background())
	require.NoErrorf(t, err, "can't initialize reader for object %s", handle.ObjectName())
	content, err := io.ReadAll(reader)
	require.NoErrorf(t, err, "io.ReadAll failed for object %s", handle.ObjectName())
	return content
}
