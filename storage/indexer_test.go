// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	internalStorage "github.com/elastic/package-registry/internal/storage"
	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
)

func TestInit(t *testing.T) {
	// given
	fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-full.json")
	defer fs.Stop()

	indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), FakeIndexerOptions)
	defer indexer.Close(t.Context())

	// when
	err := indexer.Init(t.Context())

	// then
	require.NoError(t, err)
}

func BenchmarkInit(b *testing.B) {
	// given
	fs := internalStorage.PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := internalStorage.ClientNoAuth(fs)

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indexer := NewIndexer(logger, storageClient, FakeIndexerOptions)

		err := indexer.Init(b.Context())
		require.NoError(b, err)

		b.StopTimer()
		require.NoError(b, indexer.Close(b.Context()))
		b.StartTimer()
	}
}

func BenchmarkIndexerUpdateIndex(b *testing.B) {
	// given
	fs := internalStorage.PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer fs.Stop()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	indexer := NewIndexer(logger, internalStorage.ClientNoAuth(fs), FakeIndexerOptions)
	defer indexer.Close(b.Context())

	err := indexer.Init(b.Context())
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		revision := fmt.Sprintf("%d", i+2)
		internalStorage.UpdateFakeServer(b, fs, revision, "testdata/search-index-all-full.json")
		b.StartTimer()
		err = indexer.updateIndex(b.Context())
		require.NoError(b, err, "index should be updated successfully")
	}
}

func BenchmarkIndexerGet(b *testing.B) {
	// given
	fs := internalStorage.PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer fs.Stop()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	indexer := NewIndexer(logger, internalStorage.ClientNoAuth(fs), FakeIndexerOptions)
	defer indexer.Close(b.Context())

	err := indexer.Init(b.Context())
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indexer.Get(b.Context(), &packages.GetOptions{})
		indexer.Get(b.Context(), &packages.GetOptions{
			Filter: &packages.Filter{
				AllVersions: true,
				Prerelease:  true,
			},
		})
		indexer.Get(b.Context(), &packages.GetOptions{Filter: &packages.Filter{
			AllVersions: false,
			Prerelease:  false,
		}})
		indexer.Get(b.Context(), &packages.GetOptions{Filter: &packages.Filter{
			AllVersions: false,
			Prerelease:  false,
			SpecMin:     semver.MustParse("3.0.0"),
			SpecMax:     semver.MustParse("3.3.0"),
		}})
	}
}

func TestGet_ListPackages(t *testing.T) {
	t.Parallel()

	// given
	fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-full.json")
	t.Cleanup(fs.Stop)
	indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), FakeIndexerOptions)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
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
			expected: 1139,
		},
		{
			name: "all versions of packages including prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: true,
					Prerelease:  true,
				},
			},
			expected: 1139,
		},
		{
			name: "latest versions of packages not including prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: false,
					Prerelease:  false,
				},
			},
			expected: 121,
		},
		{
			name: "all packages with all versions and no prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: true,
				},
			},
			expected: 663,
		},
		{
			name: "all packages with latest versions and no prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					Prerelease: false,
				},
			},
			expected: 121,
		},
		{
			name: "all packages prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					Prerelease: true,
				},
			},
			expected: 151,
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
			name: "all packages of a given category",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: true,
					Prerelease:  true,
					Category:    "datastore",
				},
			},
			expected: 75,
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
			name: "packages in a specific spec version range",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: false,
					Prerelease:  false,
					SpecMin:     semver.MustParse("1.1"),
					SpecMax:     semver.MustParse("1.1"),
				},
			},
			expected: 1,
		},
		{
			name: "filtering packages with uptime capabilities",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions:  false,
					Prerelease:   false,
					Capabilities: []string{"uptime"},
				},
			},
			expected: 121,
		},
		{
			name: "filtering packages with security capabilities",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions:  false,
					Prerelease:   false,
					Capabilities: []string{"security"},
				},
			},
			expected: 121,
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
		{
			name: "all apache packages with deprecated notice included",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: true,
					PackageName: "apache",
				},
			},
			expected: 4,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// when
			foundPackages, err := indexer.Get(t.Context(), c.options)
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
	t.Parallel()

	// given
	fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-small.json")
	t.Cleanup(fs.Stop)

	indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), FakeIndexerOptions)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err, "storage indexer must be initialized properly")

	// when
	foundPackages, err := indexer.Get(t.Context(), &packages.GetOptions{
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
	internalStorage.UpdateFakeServer(t, fs, secondRevision, "testdata/search-index-all-full.json")
	err = indexer.updateIndex(t.Context())
	require.NoError(t, err, "index should be updated successfully")

	foundPackages, err = indexer.Get(t.Context(), &packages.GetOptions{
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
	internalStorage.UpdateFakeServer(t, fs, thirdRevision, "testdata/search-index-all-small.json")
	err = indexer.updateIndex(t.Context())
	require.NoError(t, err, "index should be updated successfully")

	foundPackages, err = indexer.Get(t.Context(), &packages.GetOptions{
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

	// when: index update is performed updating some field of an existing pacakage
	internalStorage.UpdateFakeServer(t, fs, "4", "testdata/search-index-all-small-updated-fields.json")
	err = indexer.updateIndex(t.Context())
	require.NoError(t, err, "index should be updated successfully")

	foundPackages, err = indexer.Get(t.Context(), &packages.GetOptions{
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
