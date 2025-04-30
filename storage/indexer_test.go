// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/package-registry/internal/database"
	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
)

func TestInit(t *testing.T) {
	// given
	db, err := database.NewMemorySQLDB()
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db)
	require.NoError(t, err)

	fs := PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	ctx := context.Background()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)
	defer indexer.Close(ctx)

	// when
	err = indexer.Init(ctx)

	// then
	require.NoError(t, err)
}

func BenchmarkInit(b *testing.B) {
	// given
	folder := b.TempDir()
	dbPath := filepath.Join(folder, "test.db")
	db, err := database.NewFileSQLDB(dbPath)
	require.NoError(b, err)

	options, err := CreateFakeIndexerOptions(db)
	require.NoError(b, err)

	fs := PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()

		indexer := NewIndexer(logger, storageClient, options)
		defer indexer.Close(ctx)

		err := indexer.Init(ctx)
		require.NoError(b, err)
	}
}

func BenchmarkIndexerUpdateIndex(b *testing.B) {
	// given
	folder := b.TempDir()
	dbPath := filepath.Join(folder, "test.db")
	db, err := database.NewFileSQLDB(dbPath)
	require.NoError(b, err)

	options, err := CreateFakeIndexerOptions(db)
	require.NoError(b, err)

	fs := PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	ctx := context.Background()

	indexer := NewIndexer(logger, storageClient, options)
	defer indexer.Close(ctx)

	err = indexer.Init(ctx)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		revision := fmt.Sprintf("%d", i+2)
		updateFakeServer(b, fs, revision, "testdata/search-index-all-full.json")
		b.StartTimer()
		err = indexer.updateIndex(ctx)
		require.NoError(b, err, "index should be updated successfully")
	}
}

func BenchmarkIndexerGet(b *testing.B) {
	// given
	folder := b.TempDir()
	dbPath := filepath.Join(folder, "test.db")
	db, err := database.NewFileSQLDB(dbPath)
	require.NoError(b, err)

	options, err := CreateFakeIndexerOptions(db)
	require.NoError(b, err)

	fs := PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)

	ctx := context.Background()
	indexer := NewIndexer(logger, storageClient, options)
	defer indexer.Close(ctx)

	err = indexer.Init(ctx)
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			indexer.Get(context.Background(), &packages.GetOptions{})
		}
	})
}

func TestGet_ListAllPackages(t *testing.T) {
	// given
	db, err := database.NewMemorySQLDB()
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db)
	require.NoError(t, err)

	fs := PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	ctx := context.Background()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)
	defer indexer.Close(ctx)

	err = indexer.Init(ctx)
	require.NoError(t, err, "storage indexer must be initialized properly")

	// when
	foundPackages, err := indexer.Get(ctx, &packages.GetOptions{})

	// then
	require.NoError(t, err, "packages should be returned")
	require.Len(t, foundPackages, 1133)
}

func TestGet_FindLatestPackage(t *testing.T) {
	// given
	db, err := database.NewMemorySQLDB()
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db)
	require.NoError(t, err)

	fs := PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	ctx := context.Background()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)
	defer indexer.Close(ctx)

	err = indexer.Init(ctx)
	require.NoError(t, err, "storage indexer must be initialized properly")

	// when
	foundPackages, err := indexer.Get(ctx, &packages.GetOptions{
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
	db, err := database.NewMemorySQLDB()
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db)
	require.NoError(t, err)

	fs := PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	ctx := context.Background()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)
	defer indexer.Close(ctx)

	err = indexer.Init(ctx)
	require.NoError(t, err, "storage indexer must be initialized properly")

	// when
	foundPackages, err := indexer.Get(ctx, &packages.GetOptions{
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
	db, err := database.NewMemorySQLDB()
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db)
	require.NoError(t, err)

	fs := PrepareFakeServer(t, "testdata/search-index-all-small.json")
	defer fs.Stop()
	storageClient := fs.Client()

	ctx := context.Background()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)
	defer indexer.Close(ctx)

	err = indexer.Init(ctx)
	require.NoError(t, err, "storage indexer must be initialized properly")

	// when
	foundPackages, err := indexer.Get(ctx, &packages.GetOptions{
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
	updateFakeServer(t, fs, "2", "testdata/search-index-all-full.json")
	err = indexer.updateIndex(ctx)
	require.NoError(t, err, "index should be updated successfully")

	foundPackages, err = indexer.Get(ctx, &packages.GetOptions{
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

	// when: index update is performed removing packages
	updateFakeServer(t, fs, "3", "testdata/search-index-all-small.json")
	err = indexer.updateIndex(ctx)
	require.NoError(t, err, "index should be updated successfully")

	foundPackages, err = indexer.Get(ctx, &packages.GetOptions{
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
	require.Equal(t, "1Password Events Reporting", *foundPackages[0].Title)

	// when: index update is performed removing packages
	updateFakeServer(t, fs, "4", "testdata/search-index-all-small-updated-fields.json")
	err = indexer.updateIndex(ctx)
	require.NoError(t, err, "index should be updated successfully")

	foundPackages, err = indexer.Get(ctx, &packages.GetOptions{
		Filter: &packages.Filter{
			PackageName: "1password",
			PackageType: "integration",
			Prerelease:  true,
		},
	})

	// then
	// Adding new fields require to update packages.Package struct definition
	// Tested updating one of the known fields (title)
	require.NoError(t, err, "packages should be returned")
	require.Len(t, foundPackages, 1)
	require.Equal(t, "1password", foundPackages[0].Name)
	require.Equal(t, "0.2.0", foundPackages[0].Version)
	require.Equal(t, "1Password Events Reporting UPDATED", *foundPackages[0].Title)
}
