// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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
