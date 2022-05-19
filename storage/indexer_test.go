// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/elastic/package-registry/packages"
)

func TestInit(t *testing.T) {
	// given
	fs := prepareFakeServer(t, "testdata/search-index-all-1.json")
	defer fs.Stop()
	storageClient := fs.Client()
	indexer := NewIndexer(storageClient, IndexerOptions{
		PackageStorageBucketInternal: "gs://" + fakePackageStorageBucketInternal,
		PackageStorageBucketPublic:   "gs://" + fakePackageStorageBucketPublic,
		WatchInterval:                1 * time.Second,
	})

	// when
	err := indexer.Init(context.Background())

	// then
	require.NoError(t, err)
}

func TestGet(t *testing.T) {
	// given
	fs := prepareFakeServer(t, "testdata/search-index-all-1.json")
	defer fs.Stop()
	storageClient := fs.Client()
	indexer := NewIndexer(storageClient, IndexerOptions{
		PackageStorageBucketInternal: "gs://" + fakePackageStorageBucketInternal,
		PackageStorageBucketPublic:   "gs://" + fakePackageStorageBucketPublic,
		WatchInterval:                1 * time.Second,
	})

	err := indexer.Init(context.Background())
	require.NoError(t, err, "storage indexer must be initialized properly")

	// when
	foundPackages, err := indexer.Get(context.Background(), &packages.GetOptions{})

	// then
	require.NoError(t, err, "packages should be returned")
	require.Len(t, foundPackages, 123)
}

// TODO Package not present, 503?
// TODO Package index got updated while running
