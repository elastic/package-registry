// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
)

func TestInit(t *testing.T) {
	// given
	options, err := CreateFakeIndexerOptions()
	require.NoError(t, err)
	fs := PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)

	// when
	err = indexer.Init(context.Background())

	// then
	require.NoError(t, err)
}

func BenchmarkInit(b *testing.B) {
	// given
	options, err := CreateFakeIndexerOptions()
	require.NoError(b, err)
	fs := PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	logger := util.NewTestLogger()
	for i := 0; i < b.N; i++ {
		indexer := NewIndexer(logger, storageClient, options)
		err := indexer.Init(context.Background())
		require.NoError(b, err)
	}
}

func TestGet_ListAllPackages(t *testing.T) {
	// given
	options, err := CreateFakeIndexerOptions()
	require.NoError(t, err)
	fs := PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)

	err = indexer.Init(context.Background())
	require.NoError(t, err, "storage indexer must be initialized properly")

	// when
	foundPackages, err := indexer.Get(context.Background(), &packages.GetOptions{})

	// then
	require.NoError(t, err, "packages should be returned")
	require.Len(t, foundPackages, 1133)
}

func TestGet_FindLatestPackage(t *testing.T) {
	// given
	options, err := CreateFakeIndexerOptions()
	require.NoError(t, err)
	fs := PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)

	err = indexer.Init(context.Background())
	require.NoError(t, err, "storage indexer must be initialized properly")

	// when
	foundPackages, err := indexer.Get(context.Background(), &packages.GetOptions{
		Filter: &packages.Filter{
			PackageName: "apm",
			PackageType: "integration",
		},
	})

	// then
	require.NoError(t, err, "packages should be returned")
	require.Len(t, foundPackages, 1)
	require.Equal(t, "apm", foundPackages[0].Name)
	require.Equal(t, "8.2.0", foundPackages[0].Version)
}

func TestGet_UnknownPackage(t *testing.T) {
	// given
	options, err := CreateFakeIndexerOptions()
	require.NoError(t, err)
	fs := PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)

	err = indexer.Init(context.Background())
	require.NoError(t, err, "storage indexer must be initialized properly")

	// when
	foundPackages, err := indexer.Get(context.Background(), &packages.GetOptions{
		Filter: &packages.Filter{
			PackageName: "qwertyuiop",
			PackageType: "integration",
		},
	})

	// then
	require.NoError(t, err, "packages should be returned")
	require.Len(t, foundPackages, 0)
}

func TestGet_IndexUpdated(t *testing.T) {
	// given
	options, err := CreateFakeIndexerOptions()
	require.NoError(t, err)
	fs := PrepareFakeServer(t, "testdata/search-index-all-small.json")
	defer fs.Stop()
	storageClient := fs.Client()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)

	err = indexer.Init(context.Background())
	require.NoError(t, err, "storage indexer must be initialized properly")

	// when
	foundPackages, err := indexer.Get(context.Background(), &packages.GetOptions{
		Filter: &packages.Filter{
			PackageName: "1password",
			PackageType: "integration",
			Prerelease:  true,
		},
	})

	// then
	require.NoError(t, err, "packages should be returned")
	require.Len(t, foundPackages, 1)
	require.Equal(t, "1password", foundPackages[0].Name)
	require.Equal(t, "0.2.0", foundPackages[0].Version)

	// when: index update is performed
	const secondRevision = "2"
	updateFakeServer(t, fs, secondRevision, "testdata/search-index-all-full.json")
	err = indexer.updateIndex(context.Background())
	require.NoError(t, err, "index should be updated successfully")

	foundPackages, err = indexer.Get(context.Background(), &packages.GetOptions{
		Filter: &packages.Filter{
			PackageName: "1password",
			PackageType: "integration",
			Prerelease:  true,
		},
	})

	// then
	require.NoError(t, err, "packages should be returned")
	require.Len(t, foundPackages, 1)
	require.Equal(t, "1password", foundPackages[0].Name)
	require.Equal(t, "1.4.0", foundPackages[0].Version)
}
