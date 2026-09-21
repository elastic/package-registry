// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadConfigValid(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
address: "https://test.elastic.co"
queries:
  - kibana.version: "8.0.0"
  - prerelease: true
actions:
  - print:
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := readConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, "https://test.elastic.co", cfg.Address)
	assert.Len(t, cfg.Queries, 2)
	assert.Equal(t, "8.0.0", cfg.Queries[0].KibanaVersion)
	assert.True(t, cfg.Queries[1].Prerelease)
	assert.Len(t, cfg.Actions, 1)
}

func TestReadConfigInvalidPath(t *testing.T) {
	_, err := readConfig("/nonexistent/config.yaml")
	require.Error(t, err)
}

func TestReadConfigInvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")

	invalidContent := `
address: https://test.elastic.co
queries:
  - this is not: valid: yaml
`
	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	_, err = readConfig(configPath)
	require.Error(t, err)
}

func TestConfigActionFactory(t *testing.T) {
	tests := []struct {
		name        string
		actionName  string
		expectError bool
	}{
		{
			name:        "print action",
			actionName:  "print",
			expectError: false,
		},
		{
			name:        "download action",
			actionName:  "download",
			expectError: false,
		},
		{
			name:        "unknown action",
			actionName:  "unknown",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, err := configActionFactory(tt.actionName)
			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, action)
			} else {
				require.NoError(t, err)
				require.NotNil(t, action)
			}
		})
	}
}

func TestConfigSearchURLs(t *testing.T) {
	tests := []struct {
		name          string
		config        config
		expectedURLs  []string
		expectedError bool
	}{
		{
			name: "simple query",
			config: config{
				Address: "http://localhost:8080",
				Queries: []configQuery{
					{KibanaVersion: "8.0.0"},
				},
			},
			expectedURLs: []string{
				"http://localhost:8080/search?kibana.version=8.0.0",
			},
		},
		{
			name: "multiple queries",
			config: config{
				Address: "http://localhost:8080",
				Queries: []configQuery{
					{},
					{Prerelease: true},
				},
			},
			expectedURLs: []string{
				"http://localhost:8080/search",
				"http://localhost:8080/search?prerelease=true",
			},
		},
		{
			name: "matrix expansion",
			config: config{
				Address: "http://localhost:8080",
				Matrix: []configQuery{
					{},
					{Prerelease: true},
				},
				Queries: []configQuery{
					{KibanaVersion: "8.0.0"},
				},
			},
			expectedURLs: []string{
				"http://localhost:8080/search?kibana.version=8.0.0",
				"http://localhost:8080/search?kibana.version=8.0.0&prerelease=true",
			},
		},
		{
			name: "spec constraints",
			config: config{
				Address: "http://localhost:8080",
				Queries: []configQuery{
					{SpecMin: "2.0", SpecMax: "3.0"},
				},
			},
			expectedURLs: []string{
				"http://localhost:8080/search?spec.max=3.0&spec.min=2.0",
			},
		},
		{
			name: "default address",
			config: config{
				Queries: []configQuery{
					{Package: "nginx"},
				},
			},
			expectedURLs: []string{
				"https://epr.elastic.co/search?package=nginx",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls, err := tt.config.searchURLs()

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			var actualURLs []string
			for u := range urls {
				actualURLs = append(actualURLs, u.String())
			}

			assert.ElementsMatch(t, tt.expectedURLs, actualURLs)
		})
	}
}

func TestConfigDownloadPathForPackage(t *testing.T) {
	cfg := config{}

	pkgPath, sigPath := cfg.downloadPathForPackage("nginx", "1.0.0")
	require.Equal(t, "epr/nginx/nginx-1.0.0.zip", pkgPath)
	require.Equal(t, "epr/nginx/nginx-1.0.0.zip.sig", sigPath)
}

func TestConfigPinnedPackages(t *testing.T) {
	cfg := config{
		Packages: []configPackage{
			{Name: "nginx", Version: "1.0.0"},
			{Name: "apache", Version: "2.0.0"},
		},
	}

	packages, err := cfg.pinnedPackages()
	require.NoError(t, err)
	assert.Len(t, packages, 2)
	assert.Equal(t, "nginx", packages[0].Name)
	assert.Equal(t, "1.0.0", packages[0].Version)
	assert.Equal(t, "epr/nginx/nginx-1.0.0.zip", packages[0].Download)
	assert.Equal(t, "epr/nginx/nginx-1.0.0.zip.sig", packages[0].SignaturePath)
}

func TestConfigCollect(t *testing.T) {
	tests := []struct {
		name             string
		config           config
		mockResponse     []packageInfo
		expectedPackages int
	}{
		{
			name: "basic collection",
			config: config{
				Queries: []configQuery{
					{},
				},
			},
			mockResponse: []packageInfo{
				{Name: "nginx", Version: "1.0.0", Download: "/epr/nginx/nginx-1.0.0.zip"},
				{Name: "apache", Version: "2.0.0", Download: "/epr/apache/apache-2.0.0.zip"},
			},
			expectedPackages: 2,
		},
		{
			name: "deduplication",
			config: config{
				Queries: []configQuery{
					{KibanaVersion: "8.0.0"},
					{KibanaVersion: "8.1.0"},
				},
			},
			mockResponse: []packageInfo{
				{Name: "nginx", Version: "1.0.0", Download: "/epr/nginx/nginx-1.0.0.zip"},
			},
			expectedPackages: 1,
		},
		{
			name: "with pinned packages",
			config: config{
				Packages: []configPackage{
					{Name: "mysql", Version: "1.5.0"},
				},
				Queries: []configQuery{
					{},
				},
			},
			mockResponse: []packageInfo{
				{Name: "nginx", Version: "1.0.0", Download: "/epr/nginx/nginx-1.0.0.zip"},
			},
			expectedPackages: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/search", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			tt.config.Address = server.URL
			client := &http.Client{}

			packages, err := tt.config.collect(client)
			require.NoError(t, err)
			require.Len(t, packages, tt.expectedPackages)
		})
	}
}

func TestConfigCollectSorting(t *testing.T) {
	mockResponse := []packageInfo{
		{Name: "zebra", Version: "1.0.0"},
		{Name: "apache", Version: "2.0.0"},
		{Name: "apache", Version: "1.0.0"},
		{Name: "nginx", Version: "1.0.0"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config{
		Address: server.URL,
		Queries: []configQuery{{}},
	}

	packages, err := cfg.collect(&http.Client{})
	require.NoError(t, err)
	assert.Len(t, packages, 4)

	// Verify sorted by name, then by version
	assert.Equal(t, "apache", packages[0].Name)
	assert.Equal(t, "1.0.0", packages[0].Version)
	assert.Equal(t, "apache", packages[1].Name)
	assert.Equal(t, "2.0.0", packages[1].Version)
	assert.Equal(t, "nginx", packages[2].Name)
	assert.Equal(t, "zebra", packages[3].Name)
}

func TestConfigCollectHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := config{
		Address: server.URL,
		Queries: []configQuery{{}},
	}

	_, err := cfg.collect(&http.Client{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "status code 500")
}

func TestConfigQueryBuild(t *testing.T) {
	tests := []struct {
		name     string
		query    configQuery
		expected url.Values
	}{
		{
			name:     "empty query",
			query:    configQuery{},
			expected: url.Values{},
		},
		{
			name: "kibana version",
			query: configQuery{
				KibanaVersion: "8.0.0",
			},
			expected: url.Values{
				"kibana.version": []string{"8.0.0"},
			},
		},
		{
			name: "multiple fields",
			query: configQuery{
				Package:    "nginx",
				Prerelease: true,
				Type:       "integration",
			},
			expected: url.Values{
				"package":    []string{"nginx"},
				"prerelease": []string{"true"},
				"type":       []string{"integration"},
			},
		},
		{
			name: "spec constraints",
			query: configQuery{
				SpecMin: "2.0",
				SpecMax: "3.0",
			},
			expected: url.Values{
				"spec.min": []string{"2.0"},
				"spec.max": []string{"3.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := tt.query.Build()
			require.Equal(t, tt.expected, values)
		})
	}
}

func TestPrintAction(t *testing.T) {
	action := &printAction{}

	// Test init
	err := action.init(config{})
	require.NoError(t, err)

	// Test perform (just verify it doesn't error)
	err = action.perform(packageInfo{
		Name:    "nginx",
		Version: "1.0.0",
	})
	require.NoError(t, err)
}

func keepPtr(n int) *int { return &n }

func TestConfigKeepFor(t *testing.T) {
	tests := []struct {
		name     string
		doc      int
		matrix   *int
		query    *int
		expected int
	}{
		{name: "nothing set", expected: 0},
		{name: "document default only", doc: 3, expected: 3},
		{name: "matrix overrides document", doc: 3, matrix: keepPtr(5), expected: 5},
		{name: "query overrides both", doc: 3, matrix: keepPtr(5), query: keepPtr(2), expected: 2},
		{name: "query zero overrides non-zero default", doc: 3, query: keepPtr(0), expected: 0},
		{name: "negative clamped to zero", doc: -1, expected: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config{Keep: tt.doc}
			m := configQuery{Keep: tt.matrix}
			q := configQuery{Keep: tt.query}
			assert.Equal(t, tt.expected, cfg.keepFor(m, q))
		})
	}
}

func TestTruncateVersions(t *testing.T) {
	tests := []struct {
		name     string
		packages []packageInfo
		keep     int
		expected []packageInfo
	}{
		{
			name:     "keep zero returns all",
			packages: []packageInfo{{Name: "nginx", Version: "1.0.0"}, {Name: "nginx", Version: "2.0.0"}},
			keep:     0,
			expected: []packageInfo{{Name: "nginx", Version: "1.0.0"}, {Name: "nginx", Version: "2.0.0"}},
		},
		{
			name:     "keep above group size returns all",
			packages: []packageInfo{{Name: "nginx", Version: "1.0.0"}, {Name: "nginx", Version: "2.0.0"}},
			keep:     5,
			expected: []packageInfo{{Name: "nginx", Version: "1.0.0"}, {Name: "nginx", Version: "2.0.0"}},
		},
		{
			name: "two names truncated independently",
			packages: []packageInfo{
				{Name: "nginx", Version: "1.0.0"},
				{Name: "nginx", Version: "2.0.0"},
				{Name: "nginx", Version: "3.0.0"},
				{Name: "apache", Version: "1.0.0"},
				{Name: "apache", Version: "2.0.0"},
				{Name: "apache", Version: "3.0.0"},
			},
			keep: 2,
			expected: []packageInfo{
				{Name: "apache", Version: "2.0.0"},
				{Name: "apache", Version: "3.0.0"},
				{Name: "nginx", Version: "2.0.0"},
				{Name: "nginx", Version: "3.0.0"},
			},
		},
		{
			name: "invalid semver dropped first",
			packages: []packageInfo{
				{Name: "nginx", Version: "not-a-version"},
				{Name: "nginx", Version: "1.0.0"},
				{Name: "nginx", Version: "2.0.0"},
			},
			keep: 2,
			expected: []packageInfo{
				{Name: "nginx", Version: "1.0.0"},
				{Name: "nginx", Version: "2.0.0"},
			},
		},
		{
			name:     "empty slice",
			packages: []packageInfo{},
			keep:     2,
			expected: []packageInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateVersions(tt.packages, tt.keep)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigSearchURLsKeepForcesAll(t *testing.T) {
	tests := []struct {
		name         string
		keep         int
		queryKeep    *int
		expectedURLs []string
	}{
		{
			name:         "keep > 1 sets all=true",
			keep:         2,
			expectedURLs: []string{"http://localhost:8080/search?all=true&package=nginx"},
		},
		{
			name:         "keep == 1 does not set all=true",
			keep:         1,
			expectedURLs: []string{"http://localhost:8080/search?package=nginx"},
		},
		{
			name:         "keep == 0 does not set all=true",
			keep:         0,
			expectedURLs: []string{"http://localhost:8080/search?package=nginx"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config{
				Address: "http://localhost:8080",
				Keep:    tt.keep,
				Queries: []configQuery{{Package: "nginx"}},
			}
			urls, err := cfg.searchURLs()
			require.NoError(t, err)

			var actual []string
			for u := range urls {
				actual = append(actual, u.String())
			}
			assert.Equal(t, tt.expectedURLs, actual)
		})
	}
}

func TestConfigQueryBuildKeepExcluded(t *testing.T) {
	q := configQuery{Package: "nginx", Keep: keepPtr(3)}
	values := q.Build()
	assert.Equal(t, url.Values{"package": []string{"nginx"}}, values)
}

func TestConfigCollectKeep(t *testing.T) {
	packages := []packageInfo{
		{Name: "nginx", Version: "1.0.0"},
		{Name: "nginx", Version: "2.0.0"},
		{Name: "nginx", Version: "3.0.0"},
		{Name: "nginx", Version: "4.0.0"},
		{Name: "nginx", Version: "5.0.0"},
		{Name: "apache", Version: "1.0.0"},
		{Name: "apache", Version: "2.0.0"},
		{Name: "apache", Version: "3.0.0"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(packages)
	}))
	defer server.Close()

	cfg := config{
		Address: server.URL,
		Keep:    2,
		Queries: []configQuery{{Package: "nginx"}},
	}

	result, err := cfg.collect(&http.Client{})
	require.NoError(t, err)
	require.Len(t, result, 4)
	assert.Equal(t, "apache", result[0].Name)
	assert.Equal(t, "2.0.0", result[0].Version)
	assert.Equal(t, "apache", result[1].Name)
	assert.Equal(t, "3.0.0", result[1].Version)
	assert.Equal(t, "nginx", result[2].Name)
	assert.Equal(t, "4.0.0", result[2].Version)
	assert.Equal(t, "nginx", result[3].Name)
	assert.Equal(t, "5.0.0", result[3].Version)
}

func TestConfigCollectKeepIsPerResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp []packageInfo
		switch r.URL.Query().Get("kibana.version") {
		case "8.0.0":
			resp = []packageInfo{
				{Name: "nginx", Version: "1.0.0"},
				{Name: "nginx", Version: "2.0.0"},
			}
		case "9.0.0":
			resp = []packageInfo{
				{Name: "nginx", Version: "3.0.0"},
				{Name: "nginx", Version: "4.0.0"},
			}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := config{
		Address: server.URL,
		Keep:    1,
		Matrix: []configQuery{
			{KibanaVersion: "8.0.0"},
			{KibanaVersion: "9.0.0"},
		},
		Queries: []configQuery{{Package: "nginx"}},
	}

	result, err := cfg.collect(&http.Client{})
	require.NoError(t, err)
	// Each matrix entry contributes its own newest 1, so the union has 2.
	require.Len(t, result, 2)
	assert.Equal(t, "2.0.0", result[0].Version)
	assert.Equal(t, "4.0.0", result[1].Version)
}

func TestConfigCollectKeepDoesNotTruncatePinned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := []packageInfo{
			{Name: "nginx", Version: "1.0.0"},
			{Name: "nginx", Version: "2.0.0"},
			{Name: "nginx", Version: "3.0.0"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := config{
		Address: server.URL,
		Keep:    1,
		Packages: []configPackage{
			{Name: "nginx", Version: "1.0.0"},
		},
		Queries: []configQuery{{Package: "nginx"}},
	}

	result, err := cfg.collect(&http.Client{})
	require.NoError(t, err)
	// Pinned nginx 1.0.0 is kept; the search contributes nginx 3.0.0 (newest 1).
	require.Len(t, result, 2)
	assert.Equal(t, "nginx", result[0].Name)
	assert.Equal(t, "1.0.0", result[0].Version)
	assert.Equal(t, "epr/nginx/nginx-1.0.0.zip", result[0].Download)
	assert.Equal(t, "nginx", result[1].Name)
	assert.Equal(t, "3.0.0", result[1].Version)
}

func TestReadConfigValidKeep(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
address: "https://test.elastic.co"
keep: 3
queries:
  - package: nginx
    keep: 1
  - package: apache
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := readConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, 3, cfg.Keep)
	require.NotNil(t, cfg.Queries[0].Keep)
	assert.Equal(t, 1, *cfg.Queries[0].Keep)
	assert.Nil(t, cfg.Queries[1].Keep)
}

// TestConfigCollectFailFast verifies that when one search request fails
// terminally, the context is cancelled and the remaining URLs are not sent
// to the server. Total server hits must be well below len(urls)*maxAttempts.
func TestConfigCollectFailFast(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	// Build a config with 20 matrix entries so there are plenty of URLs to
	// cancel before they are sent.
	matrix := make([]configQuery, 20)
	for i := range matrix {
		matrix[i] = configQuery{KibanaVersion: fmt.Sprintf("8.%d.0", i)}
	}
	cfg := config{
		Address: server.URL,
		Matrix:  matrix,
		Queries: []configQuery{{Package: "nginx"}},
	}

	client, _ := newTestClient()
	_, err := cfg.collect(client)
	require.Error(t, err)

	// The error must be the real failure, not a context noise message.
	assert.Contains(t, err.Error(), "status code 500",
		"error must report the actual failure status")
	assert.NotContains(t, err.Error(), "context canceled",
		"context.Canceled must be swallowed; only the root cause should surface")

	// Fail-fast must bound total hits to far fewer than 20*maxAttempts = 80.
	// Using 20 as the bound: if fail-fast works, only a handful of URLs fire.
	assert.Less(t, hits.Load(), int64(20),
		"fail-fast should stop the majority of requests")
}

// TestConfigCollectRetriesTransientError verifies that a 503 on the first
// attempt is retried and collect ultimately returns the full package set.
func TestConfigCollectRetriesTransientError(t *testing.T) {
	var mu sync.Mutex
	hitCount := make(map[string]int)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hitCount[r.URL.String()]++
		count := hitCount[r.URL.String()]
		mu.Unlock()

		if count == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]packageInfo{
			{Name: "nginx", Version: "1.0.0", Download: "/epr/nginx/nginx-1.0.0.zip"},
		})
	}))
	defer server.Close()

	cfg := config{
		Address: server.URL,
		Matrix: []configQuery{
			{KibanaVersion: "8.0.0"},
			{KibanaVersion: "9.0.0"},
		},
		Queries: []configQuery{{Package: "nginx"}},
	}

	client, _ := newTestClient()
	packages, err := cfg.collect(client)
	require.NoError(t, err)
	// Both matrix entries return nginx 1.0.0; after dedup by (name, version) it's 1 package.
	require.Len(t, packages, 1)
	assert.Equal(t, "nginx", packages[0].Name)
}

// TestConfigSearchURLsDeduplicates verifies that identical URLs produced by
// different matrix/query combinations are collapsed into a single entry,
// and that keep windows are merged correctly.
func TestConfigSearchURLsDeduplicates(t *testing.T) {
	tests := []struct {
		name      string
		cfg       config
		wantCount int
		wantKeep  int // expected keep for the first (and only) URL
	}{
		{
			name: "query spec.max overrides both matrix spec.max values",
			cfg: config{
				Address: "http://localhost:8080",
				Matrix: []configQuery{
					{SpecMax: "3.0"},
					{SpecMax: "3.3"},
				},
				Queries: []configQuery{
					{Package: "nginx", SpecMax: "3.6"},
				},
			},
			wantCount: 1,
			wantKeep:  0, // both matrix entries inherit the document default of 0
		},
		{
			name: "merged keep takes the wider window",
			cfg: config{
				Address: "http://localhost:8080",
				Matrix: []configQuery{
					{Keep: keepPtr(2)},
					{Keep: keepPtr(3)},
				},
				// No package filter so both matrix entries produce the same
				// /search?all=true URL (keep>1 forces all=true in both).
				Queries: []configQuery{{}},
			},
			wantCount: 1,
			wantKeep:  3,
		},
		{
			name: "unlimited (keep=0) wins over any bounded window",
			cfg: config{
				Address: "http://localhost:8080",
				Matrix: []configQuery{
					{Keep: keepPtr(0)},
					{Keep: keepPtr(1)},
				},
				Queries: []configQuery{{}},
			},
			wantCount: 1,
			wantKeep:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls, err := tt.cfg.searchURLs()
			require.NoError(t, err)

			var gotURLs []string
			var gotKeeps []int
			for u, k := range urls {
				gotURLs = append(gotURLs, u.String())
				gotKeeps = append(gotKeeps, k)
			}

			require.Len(t, gotURLs, tt.wantCount,
				"expected %d unique URL(s), got: %v", tt.wantCount, gotURLs)
			assert.Equal(t, tt.wantKeep, gotKeeps[0])
		})
	}
}
