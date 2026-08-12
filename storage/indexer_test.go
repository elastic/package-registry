// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"fmt"
	"os"
	"sync"
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
	defer func() { fs.Stop() }()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	indexer := NewIndexer(logger, internalStorage.ClientNoAuth(fs), FakeIndexerOptions)
	defer indexer.Close(b.Context())

	err := indexer.Init(b.Context())
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		revision := fmt.Sprintf("%d", i+2)
		fs, indexer.storageClient = internalStorage.UpdateFakeServer(b, fs, revision, "testdata/search-index-all-full.json")
		b.StartTimer()
		err = indexer.updateIndex(b.Context())
		require.NoError(b, err, "index should be updated successfully")
	}
}

// BenchmarkIndexerUpdateIndex_Incremental measures the cost of applying a small
// delta (1 package added) against a large in-memory index (search-index-all-full.json,
// ~1139 packages). Compare directly with BenchmarkIndexerUpdateIndex which does a
// full reload of the same index every poll cycle.
//
// The delta server is set up once at revision "2" (lexicographically > "1", the
// initial revision from PrepareFakeServer). Each iteration resets the indexer cursor
// to "1" so updateIndex always sees a real "1"→"2" incremental transition, measuring
// only the steady-state cost: GCS cursor list + delta read + applyDelta.
func BenchmarkIndexerUpdateIndex_Incremental(b *testing.B) {
	// given
	fs := internalStorage.PrepareFakeServer(b, "testdata/search-index-all-full.json")
	defer func() { fs.Stop() }()

	deltaContent, err := os.ReadFile("testdata/search-index-delta-add.json")
	require.NoError(b, err)

	incrementalOptions := IndexerOptions{
		PackageStorageBucketInternal: "gs://" + internalStorage.FakePackageStorageBucketInternal,
		WatchInterval:                0,
		IncrementalUpdates:           true,
	}

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	indexer := NewIndexer(logger, internalStorage.ClientNoAuth(fs), incrementalOptions)
	defer indexer.Close(b.Context())

	err = indexer.Init(b.Context())
	require.NoError(b, err)

	// Set up the delta at revision "2" once — "2" > "1" lexicographically so
	// ListCursorsBetween will find it on every iteration.
	fs, indexer.storageClient = internalStorage.UpdateFakeServerWithDelta(b, fs, "2", deltaContent)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset cursor so updateIndex sees a live "1"→"2" incremental transition.
		indexer.cursor = "1"
		err = indexer.updateIndex(b.Context())
		require.NoError(b, err, "incremental index update should succeed")
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
			expected: 664,
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
	t.Cleanup(func() { fs.Stop() })

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
	fs, indexer.storageClient = internalStorage.UpdateFakeServer(t, fs, secondRevision, "testdata/search-index-all-full.json")
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
	fs, indexer.storageClient = internalStorage.UpdateFakeServer(t, fs, thirdRevision, "testdata/search-index-all-small.json")
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
	fs, indexer.storageClient = internalStorage.UpdateFakeServer(t, fs, "4", "testdata/search-index-all-small-updated-fields.json")
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

func TestIncrementalUpdate(t *testing.T) {
	t.Parallel()

	incrementalOptions := IndexerOptions{
		PackageStorageBucketInternal: "gs://" + internalStorage.FakePackageStorageBucketInternal,
		WatchInterval:                0,
		IncrementalUpdates:           true,
	}

	readDeltaFile := func(t *testing.T, path string) []byte {
		t.Helper()
		content, err := os.ReadFile(path)
		require.NoError(t, err, "delta test data file must be readable")
		return content
	}

	t.Run("add_package", func(t *testing.T) {
		t.Parallel()

		fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-small.json")
		t.Cleanup(fs.Stop)

		indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), incrementalOptions)
		t.Cleanup(func() { indexer.Close(context.Background()) })

		err := indexer.Init(t.Context())
		require.NoError(t, err)

		deltaContent := readDeltaFile(t, "testdata/search-index-delta-add.json")
		_, indexer.storageClient = internalStorage.UpdateFakeServerWithDelta(t, fs, "2", deltaContent)

		err = indexer.updateIndex(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "2", indexer.cursor, "cursor must advance to the applied revision")

		foundPackages, err := indexer.Get(t.Context(), &packages.GetOptions{
			Filter: &packages.Filter{AllVersions: true, Prerelease: true, PackageName: "1password"},
		})
		require.NoError(t, err)
		require.Len(t, foundPackages, 3, "should have original 2 packages plus the new 0.3.0")

		versions := make(map[string]bool)
		for _, p := range foundPackages {
			versions[p.Version] = true
		}
		assert.True(t, versions["0.3.0"], "new 0.3.0 package must be present")
		assert.True(t, versions["0.1.1"], "original 0.1.1 package must be present")
		assert.True(t, versions["0.2.0"], "original 0.2.0 package must be present")
	})

	t.Run("remove_package", func(t *testing.T) {
		t.Parallel()

		fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-small.json")
		t.Cleanup(fs.Stop)

		indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), incrementalOptions)
		t.Cleanup(func() { indexer.Close(context.Background()) })

		err := indexer.Init(t.Context())
		require.NoError(t, err)

		deltaContent := readDeltaFile(t, "testdata/search-index-delta-remove.json")
		_, indexer.storageClient = internalStorage.UpdateFakeServerWithDelta(t, fs, "2", deltaContent)

		err = indexer.updateIndex(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "2", indexer.cursor, "cursor must advance to the applied revision")

		foundPackages, err := indexer.Get(t.Context(), &packages.GetOptions{
			Filter: &packages.Filter{AllVersions: true, Prerelease: true, PackageName: "1password"},
		})
		require.NoError(t, err)
		require.Len(t, foundPackages, 1, "0.1.1 should have been removed")
		assert.Equal(t, "0.2.0", foundPackages[0].Version)
	})

	t.Run("update_package_fields", func(t *testing.T) {
		t.Parallel()

		fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-small.json")
		t.Cleanup(fs.Stop)

		indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), incrementalOptions)
		t.Cleanup(func() { indexer.Close(context.Background()) })

		err := indexer.Init(t.Context())
		require.NoError(t, err)

		deltaContent := readDeltaFile(t, "testdata/search-index-delta-update.json")
		_, indexer.storageClient = internalStorage.UpdateFakeServerWithDelta(t, fs, "2", deltaContent)

		err = indexer.updateIndex(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "2", indexer.cursor, "cursor must advance to the applied revision")

		foundPackages, err := indexer.Get(t.Context(), &packages.GetOptions{
			Filter: &packages.Filter{PackageName: "1password", PackageVersion: "0.2.0", Prerelease: true},
		})
		require.NoError(t, err)
		require.Len(t, foundPackages, 1)
		require.NotNil(t, foundPackages[0].Title)
		assert.Equal(t, "1Password Events Reporting UPDATED DELTA", *foundPackages[0].Title)
	})

	t.Run("update_is_full_replacement", func(t *testing.T) {
		// Verifies that an updated package is fully replaced, not merged with the original.
		t.Parallel()

		fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-small.json")
		t.Cleanup(fs.Stop)

		indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), incrementalOptions)
		t.Cleanup(func() { indexer.Close(context.Background()) })

		err := indexer.Init(t.Context())
		require.NoError(t, err)

		deltaContent := readDeltaFile(t, "testdata/search-index-delta-update.json")
		_, indexer.storageClient = internalStorage.UpdateFakeServerWithDelta(t, fs, "2", deltaContent)

		err = indexer.updateIndex(t.Context())
		require.NoError(t, err)

		allPackages, err := indexer.Get(t.Context(), &packages.GetOptions{
			Filter: &packages.Filter{AllVersions: true, Prerelease: true, PackageName: "1password"},
		})
		require.NoError(t, err)
		// Exact count: update must not duplicate — still exactly 2 versions.
		require.Len(t, allPackages, 2, "update must replace, not duplicate the package entry")

		byVersion := make(map[string]*packages.Package)
		for _, p := range allPackages {
			pkg := p
			byVersion[p.Version] = pkg
		}

		require.Contains(t, byVersion, "0.2.0")
		require.Contains(t, byVersion, "0.1.1")

		updated := byVersion["0.2.0"]
		require.NotNil(t, updated.Title)
		assert.Equal(t, "1Password Events Reporting UPDATED DELTA", *updated.Title, "0.2.0 title must reflect the delta, not the original")

		untouched := byVersion["0.1.1"]
		require.NotNil(t, untouched.Title)
		assert.Equal(t, "1Password Events Reporting", *untouched.Title, "0.1.1 must not be affected by the update delta")
	})

	t.Run("multiple_deltas_in_order", func(t *testing.T) {
		t.Parallel()

		fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-small.json")
		t.Cleanup(fs.Stop)

		indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), incrementalOptions)
		t.Cleanup(func() { indexer.Close(context.Background()) })

		err := indexer.Init(t.Context())
		require.NoError(t, err)

		// revision "2": add 0.3.0
		fs, indexer.storageClient = internalStorage.UpdateFakeServerWithDelta(t, fs, "2", readDeltaFile(t, "testdata/search-index-delta-add.json"))
		// revision "3": remove 0.1.1
		fs, indexer.storageClient = internalStorage.UpdateFakeServerWithDelta(t, fs, "3", readDeltaFile(t, "testdata/search-index-delta-remove.json"))
		// revision "4": update 0.2.0 title — cursor.json now points to "4"
		_, indexer.storageClient = internalStorage.UpdateFakeServerWithDelta(t, fs, "4", readDeltaFile(t, "testdata/search-index-delta-update.json"))

		// single updateIndex call should apply all three deltas
		err = indexer.updateIndex(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "4", indexer.cursor, "cursor must advance to the last applied revision")

		foundPackages, err := indexer.Get(t.Context(), &packages.GetOptions{
			Filter: &packages.Filter{AllVersions: true, Prerelease: true, PackageName: "1password"},
		})
		require.NoError(t, err)
		require.Len(t, foundPackages, 2, "0.1.1 removed, 0.2.0 and 0.3.0 remain")

		versions := make(map[string]*packages.Package)
		for _, p := range foundPackages {
			pkg := p
			versions[p.Version] = pkg
		}
		assert.Contains(t, versions, "0.2.0")
		assert.Contains(t, versions, "0.3.0")
		assert.NotContains(t, versions, "0.1.1")
		if p, ok := versions["0.2.0"]; ok {
			require.NotNil(t, p.Title)
			assert.Equal(t, "1Password Events Reporting UPDATED DELTA", *p.Title)
		}
	})

	t.Run("flag_off_uses_full_sync", func(t *testing.T) {
		t.Parallel()

		fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-small.json")
		t.Cleanup(fs.Stop)

		indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), FakeIndexerOptions)
		t.Cleanup(func() { indexer.Close(context.Background()) })

		err := indexer.Init(t.Context())
		require.NoError(t, err)

		_, indexer.storageClient = internalStorage.UpdateFakeServer(t, fs, "2", "testdata/search-index-all-full.json")
		err = indexer.updateIndex(t.Context())
		require.NoError(t, err)

		foundPackages, err := indexer.Get(t.Context(), &packages.GetOptions{
			Filter: &packages.Filter{AllVersions: true, Prerelease: true},
		})
		require.NoError(t, err)
		assert.Len(t, foundPackages, 1139, "full sync should have loaded all packages")
	})

	t.Run("same_cursor_skips", func(t *testing.T) {
		t.Parallel()

		fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-small.json")
		t.Cleanup(fs.Stop)

		indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), incrementalOptions)
		t.Cleanup(func() { indexer.Close(context.Background()) })

		err := indexer.Init(t.Context())
		require.NoError(t, err)

		// cursor still "1" — no change on server
		err = indexer.updateIndex(t.Context())
		require.NoError(t, err)

		foundPackages, err := indexer.Get(t.Context(), &packages.GetOptions{
			Filter: &packages.Filter{AllVersions: true, Prerelease: true, PackageName: "1password"},
		})
		require.NoError(t, err)
		assert.Len(t, foundPackages, 2, "package list should be unchanged")
	})

	t.Run("startup_always_full_sync", func(t *testing.T) {
		t.Parallel()

		fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-small.json")
		t.Cleanup(fs.Stop)

		indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), incrementalOptions)
		t.Cleanup(func() { indexer.Close(context.Background()) })

		// cursor is "" on startup — must do full sync even with IncrementalUpdates: true
		err := indexer.Init(t.Context())
		require.NoError(t, err)

		foundPackages, err := indexer.Get(t.Context(), &packages.GetOptions{
			Filter: &packages.Filter{AllVersions: true, Prerelease: true, PackageName: "1password"},
		})
		require.NoError(t, err)
		assert.Len(t, foundPackages, 2, "full sync on startup must load all packages")
	})

	t.Run("delta_missing_falls_back_to_full_sync", func(t *testing.T) {
		t.Parallel()

		fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-small.json")
		t.Cleanup(fs.Stop)

		indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), incrementalOptions)
		t.Cleanup(func() { indexer.Close(context.Background()) })

		err := indexer.Init(t.Context())
		require.NoError(t, err)

		// revision "2" has a full search-index-all.json but no delta file
		_, indexer.storageClient = internalStorage.UpdateFakeServer(t, fs, "2", "testdata/search-index-all-full.json")

		err = indexer.updateIndex(t.Context())
		require.NoError(t, err)

		foundPackages, err := indexer.Get(t.Context(), &packages.GetOptions{
			Filter: &packages.Filter{AllVersions: true, Prerelease: true},
		})
		require.NoError(t, err)
		assert.Len(t, foundPackages, 1139, "fallback full sync should have loaded all packages")
	})

	// This test is intentionally racy without the fix in applyDelta that allocates a
	// fresh backing array. Run with -race to detect regressions.
	t.Run("concurrent_get_and_apply_delta", func(t *testing.T) {
		t.Parallel()

		fs := internalStorage.PrepareFakeServer(t, "testdata/search-index-all-small.json")
		t.Cleanup(fs.Stop)

		indexer := NewIndexer(util.NewTestLogger(), internalStorage.ClientNoAuth(fs), incrementalOptions)
		t.Cleanup(func() { indexer.Close(context.Background()) })

		err := indexer.Init(t.Context())
		require.NoError(t, err)

		deltaContent := readDeltaFile(t, "testdata/search-index-delta-add.json")
		_, indexer.storageClient = internalStorage.UpdateFakeServerWithDelta(t, fs, "2", deltaContent)

		var wg sync.WaitGroup
		for range 50 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = indexer.Get(t.Context(), &packages.GetOptions{})
			}()
		}

		err = indexer.updateIndex(t.Context())
		require.NoError(t, err)
		wg.Wait()
	})
}
