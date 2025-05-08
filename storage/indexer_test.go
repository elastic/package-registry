// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/package-registry/internal/database"
	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
)

func TestInit(t *testing.T) {
	// given
	db, err := database.NewMemorySQLDB("main")
	require.NoError(t, err)

	swapDb, err := database.NewMemorySQLDB("swap")
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
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

	swapDbPath := filepath.Join(folder, "swap_test.db")
	swapDb, err := database.NewFileSQLDB(swapDbPath)
	require.NoError(b, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
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

	swapDbPath := filepath.Join(folder, "swap_test.db")
	swapDb, err := database.NewFileSQLDB(swapDbPath)
	require.NoError(b, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
	require.NoError(b, err)

	fs := PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	ctx := context.Background()

	indexer := NewIndexer(logger, storageClient, options)
	defer indexer.Close(ctx)

	start := time.Now()
	err = indexer.Init(ctx)
	b.Logf("Elapsed time init database: %s", time.Since(start))
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		revision := fmt.Sprintf("%d", i+2)
		updateFakeServer(b, fs, revision, "testdata/search-index-all-full.json")
		b.StartTimer()
		start = time.Now()
		err = indexer.updateIndex(ctx)
		b.Logf("Elapsed time updating database: %s", time.Since(start))
		require.NoError(b, err, "index should be updated successfully")
	}
}

func BenchmarkIndexerGet(b *testing.B) {
	// given
	folder := b.TempDir()
	dbPath := filepath.Join(folder, "test.db")
	db, err := database.NewFileSQLDB(dbPath)
	require.NoError(b, err)

	swapDbPath := filepath.Join(folder, "swap_test.db")
	swapDb, err := database.NewFileSQLDB(swapDbPath)
	require.NoError(b, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
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
	db, err := database.NewMemorySQLDB("main")
	require.NoError(t, err)

	swapDb, err := database.NewMemorySQLDB("swap")
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
	require.NoError(t, err)

	fs := PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	ctx := context.Background()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)
	defer indexer.Close(ctx)

	err = indexer.Init(ctx)
	require.NoError(t, err, "storage indexer must be initialized properly")

	cases := []struct {
		name     string
		options  *packages.GetOptions
		expected int
	}{
		{
			name:     "all packages filter nil",
			options:  &packages.GetOptions{},
			expected: 1133,
		},
		{
			name: "all packages including prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: true,
					Prerelease:  true,
				},
			},
			expected: 1133,
		},
		{
			name: "not all packages including prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: false,
					Prerelease:  false,
				},
			},
			expected: 99,
		},
		{
			name: "all packages with all versions and no prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: true,
				},
			},
			expected: 494,
		},
		{
			name: "all packages with all versions with no prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					Prerelease: false,
				},
			},
			expected: 99,
		},
		{
			name: "all packages prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					Prerelease: true,
				},
			},
			expected: 147,
		},
		{
			name: "all zeek packages with prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: true,
					Prerelease:  true,
					PackageName: "zeek",
				},
			},
			expected: 17,
		},
		{
			name: "all packages of a giventype",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: true,
					Prerelease:  true,
					PackageType: "solution",
				},
			},
			expected: 2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// when
			foundPackages, err := indexer.Get(ctx, c.options)
			// then
			require.NoError(t, err, "packages should be returned")
			require.Len(t, foundPackages, c.expected)
		})
	}
}

func TestGet_FindLatestPackage(t *testing.T) {
	// given
	db, err := database.NewMemorySQLDB("main")
	require.NoError(t, err)

	swapDb, err := database.NewMemorySQLDB("swap")
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
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
	db, err := database.NewMemorySQLDB("main")
	require.NoError(t, err)

	swapDb, err := database.NewMemorySQLDB("swap")
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
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
	db, err := database.NewMemorySQLDB("main")
	require.NoError(t, err)

	swapDb, err := database.NewMemorySQLDB("swap")
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
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
