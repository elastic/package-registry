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

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/package-registry/internal/database"
	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
)

func TestSQLInit(t *testing.T) {
	t.Parallel()

	// given
	db, err := database.NewMemorySQLDB(database.MemorySQLDBOptions{Path: "main"})
	require.NoError(t, err)

	swapDb, err := database.NewMemorySQLDB(database.MemorySQLDBOptions{Path: "swap"})
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
	require.NoError(t, err)

	fs := PrepareFakeServer(t, "../../storage/testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)
	defer indexer.Close(t.Context())

	// when
	err = indexer.Init(t.Context())

	// then
	require.NoError(t, err)
}

func BenchmarkSQLInit(b *testing.B) {
	// given
	folder := b.TempDir()
	dbPath := filepath.Join(folder, "test.db")
	db, err := database.NewFileSQLDB(database.FileSQLDBOptions{Path: dbPath})
	require.NoError(b, err)

	swapDbPath := filepath.Join(folder, "swap_test.db")
	swapDb, err := database.NewFileSQLDB(database.FileSQLDBOptions{Path: swapDbPath})
	require.NoError(b, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
	require.NoError(b, err)

	fs := PrepareFakeServer(b, "../../storage/testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indexer := NewIndexer(logger, storageClient, options)

		err := indexer.Init(b.Context())
		require.NoError(b, err)

		b.StopTimer()
		require.NoError(b, indexer.Close(b.Context()))
		b.StartTimer()
	}
}

func BenchmarkSQLIndexerUpdateIndex(b *testing.B) {
	// given
	folder := b.TempDir()
	dbPath := filepath.Join(folder, "test.db")
	db, err := database.NewFileSQLDB(database.FileSQLDBOptions{Path: dbPath})
	require.NoError(b, err)

	swapDbPath := filepath.Join(folder, "swap_test.db")
	swapDb, err := database.NewFileSQLDB(database.FileSQLDBOptions{Path: swapDbPath})
	require.NoError(b, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
	require.NoError(b, err)

	fs := PrepareFakeServer(b, "../../storage/testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)

	indexer := NewIndexer(logger, storageClient, options)
	defer indexer.Close(b.Context())

	start := time.Now()
	err = indexer.Init(b.Context())
	b.Logf("Elapsed time init database: %s", time.Since(start))
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		revision := fmt.Sprintf("%d", i+2)
		UpdateFakeServer(b, fs, revision, "../../storage/testdata/search-index-all-full.json")
		b.StartTimer()
		start = time.Now()
		err = indexer.updateIndex(b.Context())
		b.Logf("Elapsed time updating database: %s", time.Since(start))
		require.NoError(b, err, "index should be updated successfully")
	}
}

func BenchmarkSQLIndexerGet(b *testing.B) {
	// given
	folder := b.TempDir()
	dbPath := filepath.Join(folder, "test.db")
	db, err := database.NewFileSQLDB(database.FileSQLDBOptions{Path: dbPath})
	require.NoError(b, err)

	swapDbPath := filepath.Join(folder, "swap_test.db")
	swapDb, err := database.NewFileSQLDB(database.FileSQLDBOptions{Path: swapDbPath})
	require.NoError(b, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
	require.NoError(b, err)

	fs := PrepareFakeServer(b, "../../storage/testdata/search-index-all-full.json")
	defer fs.Stop()
	storageClient := fs.Client()

	logger := util.NewTestLoggerLevel(zapcore.FatalLevel)

	indexer := NewIndexer(logger, storageClient, options)
	defer indexer.Close(b.Context())

	err = indexer.Init(b.Context())
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
			SpecMin:     semver.MustParse("3.0"),
			SpecMax:     semver.MustParse("3.3"),
		}})
	}
}

func TestSQLGet_ListPackages(t *testing.T) {
	t.Parallel()

	// given
	// db, err := database.NewMemorySQLDB(database.MemorySQLDBOptions{Path: "main"})
	// require.NoError(t, err)

	// swapDb, err := database.NewMemorySQLDB(database.MemorySQLDBOptions{Path: "swap"})
	// require.NoError(t, err)
	folder := t.TempDir()
	dbPath := filepath.Join(folder, "test.db")
	db, err := database.NewFileSQLDB(database.FileSQLDBOptions{Path: dbPath})
	require.NoError(t, err)

	swapDbPath := filepath.Join(folder, "swap_test.db")
	swapDb, err := database.NewFileSQLDB(database.FileSQLDBOptions{Path: swapDbPath})
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
	require.NoError(t, err)

	fs := PrepareFakeServer(t, "../../storage/testdata/search-index-all-full.json")
	t.Cleanup(fs.Stop)
	storageClient := fs.Client()

	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err = indexer.Init(t.Context())
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
			expected: 1138,
		},
		{
			name: "all versions of packages including prerelease",
			options: &packages.GetOptions{
				Filter: &packages.Filter{
					AllVersions: true,
					Prerelease:  true,
				},
			},
			expected: 1138,
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
			expected: 495,
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
			expected: 151,
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
			expected: 98,
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
			expected: 99,
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
			t.Parallel()

			// when
			startTest := time.Now()
			foundPackages, err := indexer.Get(t.Context(), c.options)
			t.Logf("Elapsed time GET: %s", time.Since(startTest))
			// then
			require.NoError(t, err, "packages should be returned")
			assert.Len(t, foundPackages, c.expected, "number of packages should be equal to expected")
			if c.expectedName != "" {
				assert.Equal(t, c.expectedName, foundPackages[0].Name)
			}
			if c.expectedVersion != "" {
				assert.Equal(t, c.expectedVersion, foundPackages[0].Version)
			}
		})
	}
}

func TestSQLGet_IndexUpdated(t *testing.T) {
	t.Parallel()

	// given
	db, err := database.NewMemorySQLDB(database.MemorySQLDBOptions{Path: "main"})
	require.NoError(t, err)

	swapDb, err := database.NewMemorySQLDB(database.MemorySQLDBOptions{Path: "swap"})
	require.NoError(t, err)

	options, err := CreateFakeIndexerOptions(db, swapDb)
	require.NoError(t, err)

	fs := PrepareFakeServer(t, "../../storage/testdata/search-index-all-small.json")
	t.Cleanup(fs.Stop)
	storageClient := fs.Client()

	indexer := NewIndexer(util.NewTestLogger(), storageClient, options)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err = indexer.Init(t.Context())
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

	// when: index update is performed
	UpdateFakeServer(t, fs, "2", "../../storage/testdata/search-index-all-full.json")
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
	UpdateFakeServer(t, fs, "3", "../../storage/testdata/search-index-all-small.json")
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
	require.Equal(t, "1Password Events Reporting", *foundPackages[0].Title)

	// when: index update is performed updating some field of an existing package
	UpdateFakeServer(t, fs, "4", "../../storage/testdata/search-index-all-small-updated-fields.json")
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
