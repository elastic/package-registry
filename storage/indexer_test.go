// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
)

func TestInit(t *testing.T) {
	// given
	fs := PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, FakeIndexerOptions)

	// when
	err := indexer.Init(context.Background())

	// then
	require.NoError(t, err)
}

func BenchmarkInit(b *testing.B) {
	// given
	fs := PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indexer := NewIndexer(logger, storageClient, FakeIndexerOptions)
		err := indexer.Init(context.Background())
		require.NoError(b, err)
	}
}

func BenchmarkIndexerUpdateIndex(b *testing.B) {
	// given
	fs := PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	indexer := NewIndexer(logger, storageClient, FakeIndexerOptions)
	err := indexer.Init(context.Background())
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		revision := fmt.Sprintf("%d", i+2)
		updateFakeServer(b, fs, revision, "testdata/search-index-all-full.json")
		b.StartTimer()
		err = indexer.updateIndex(context.Background())
		require.NoError(b, err, "index should be updated successfully")
	}
}

func BenchmarkIndexerGet(b *testing.B) {
	// given
	fs := PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	indexer := NewIndexer(logger, storageClient, FakeIndexerOptions)
	err := indexer.Init(context.Background())
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			indexer.Get(context.Background(), &packages.GetOptions{})
		}
	})
}

func TestGet_ListPackages(t *testing.T) {
	// given
	fs := PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, FakeIndexerOptions)

	ctx := context.Background()
	err := indexer.Init(ctx)
	require.NoError(t, err, "storage indexer must be initialized properly")

	cases := []struct {
		name            string
		options         *packages.GetOptions
		expected        int
		expectedName    string
		expectedVersion string
	}{
		{
			name:     "all packages filter nil",
			options:  &packages.GetOptions{},
			expected: 1133,
		},
		{
			name: "all versions of packages including prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: true,
					Prerelease:  true,
				},
			},
			expected: 1133,
		},
		{
			name: "latest versions of packages not including prerelease",
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
			name: "all packages with latest versions and no prerelease",
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
			name: "all packages with all versions of a giventype",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: true,
					Prerelease:  true,
					PackageType: "solution",
				},
			},
			expected: 2,
		},
		{
			name: "one package of a giventype",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					Prerelease:     true,
					PackageName:    "tomcat",
					PackageVersion: "0.3.0",
				},
			},
			expected: 1,
		},
		{
			name: "unknown package",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					PackageName: "qwertyuiop",
					PackageType: "integration",
				},
			},
			expected: 0,
		},
		{
			name: "latest package",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					PackageName: "apm",
					PackageType: "integration",
				},
			},
			expected:        1,
			expectedName:    "apm",
			expectedVersion: "8.2.0",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// when
			foundPackages, err := indexer.Get(ctx, c.options)
			// then
			require.NoError(t, err, "packages should be returned")
			require.Len(t, foundPackages, c.expected)
			if c.expectedName != "" {
				assert.Equal(t, c.expectedName, foundPackages[0].Name)
			}
			if c.expectedVersion != "" {
				assert.Equal(t, c.expectedVersion, foundPackages[0].Version)
			}
		})
	}
}

func TestGet_IndexUpdated(t *testing.T) {
	// given
	fs := PrepareFakeServer(t, "testdata/search-index-all-small.json")
	defer fs.Stop()
	storageClient := fs.Client()
	indexer := NewIndexer(util.NewTestLogger(), storageClient, FakeIndexerOptions)

	err := indexer.Init(context.Background())
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

	// when: index update is performed adding new packages
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

	// when: index update is performed removing packages
	const thirdRevision = "3"
	updateFakeServer(t, fs, thirdRevision, "testdata/search-index-all-small.json")
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
	require.Equal(t, "0.2.0", foundPackages[0].Version)
}
