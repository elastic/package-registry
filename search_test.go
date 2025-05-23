// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/package-registry/internal/database"
	"github.com/elastic/package-registry/internal/filesystem"
	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/proxymode"
)

func createWebServerSearch() *httptest.Server {
	// nginx 1.15.0 is not included as part of the local packages
	// datasources 1.0.0 is included as part of the local packages
	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `
[
  {
    "name": "nginx",
    "title": "Nginx",
    "version": "1.15.0",
    "release": "ga",
    "description": "Collect logs and metrics from Nginx HTTP servers with Elastic Agent.",
    "type": "integration",
    "download": "/epr/nginx/nginx-1.15.0.zip",
    "path": "/package/nginx/1.15.0",
    "icons": [
      {
        "src": "/img/logo_nginx.svg",
        "path": "/package/nginx/1.15.0/img/logo_nginx.svg",
        "title": "logo nginx",
        "size": "32x32",
        "type": "image/svg+xml"
      }
    ],
    "policy_templates": [
      {
        "name": "nginx",
        "title": "Nginx logs and metrics",
        "description": "Collect logs and metrics from Nginx instances"
      }
    ],
    "conditions": {
      "kibana": {
        "version": "^8.8.0"
      }
    },
    "owner": {
      "github": "elastic/obs-infraobs-integrations"
    },
    "categories": [
      "web",
      "observability"
    ],
    "signature_path": "/epr/nginx/nginx-1.15.0.zip.sig"
  },
  {
    "name": "datasources",
    "title": "Default datasource Integration",
    "version": "1.0.0",
    "release": "beta",
    "description": "Package with data sources",
    "type": "integration",
    "download": "/epr/datasources/datasources-1.0.0.zip",
    "path": "/package/datasources/1.0.0",
    "policy_templates": [
      {
        "name": "nginx",
        "title": "Datasource title",
        "description": "Details about the data source."
      }
    ],
    "categories": [
      "custom"
    ]
  }
]
`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, response)
	}))
	return webServer
}

func generateTestCasesSearchProxy(t *testing.T, indexer Indexer, proxyMode *proxymode.ProxyMode) []struct {
	endpoint string
	path     string
	file     string
	handler  func(w http.ResponseWriter, r *http.Request)
} {
	searchWithProxyHandler := searchHandlerWithProxyMode(testLogger, indexer, proxyMode, testCacheTime)
	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/search?all=true", "/search", "search-all-proxy.json", searchWithProxyHandler},
		{"/search", "/search", "search-just-latest-proxy.json", searchWithProxyHandler},
	}
	return tests
}

func TestSearchWithProxyModeSQL(t *testing.T) {

	webServer := createWebServerSearch()
	defer webServer.Close()

	zipDb, err := database.NewMemorySQLDB("zip")
	require.NoError(t, err)
	foldersDb, err := database.NewMemorySQLDB("folders")
	require.NoError(t, err)

	packagesBasePaths := []string{"./testdata/second_package_path", "./testdata/package"}
	indexer := NewCombinedIndexer(
		filesystem.NewZipFileSystemSQLIndexer(testLogger, zipDb, "./testdata/local-storage"),
		filesystem.NewFileSystemSQLIndexer(testLogger, foldersDb, packagesBasePaths...),
	)
	defer indexer.Close(context.Background())

	err = indexer.Init(context.Background())
	require.NoError(t, err)

	proxyMode, err := proxymode.NewProxyMode(
		testLogger,
		proxymode.ProxyOptions{
			Enabled: true,
			ProxyTo: webServer.URL,
		},
	)
	require.NoError(t, err)

	tests := generateTestCasesSearchProxy(t, indexer, proxyMode)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestSearchWithProxyMode(t *testing.T) {

	webServer := createWebServerSearch()
	defer webServer.Close()

	packagesBasePaths := []string{"./testdata/second_package_path", "./testdata/package"}
	indexer := NewCombinedIndexer(
		packages.NewZipFileSystemIndexer(testLogger, "./testdata/local-storage"),
		packages.NewFileSystemIndexer(testLogger, packagesBasePaths...),
	)
	defer indexer.Close(context.Background())

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	proxyMode, err := proxymode.NewProxyMode(
		testLogger,
		proxymode.ProxyOptions{
			Enabled: true,
			ProxyTo: webServer.URL,
		},
	)
	require.NoError(t, err)

	tests := generateTestCasesSearchProxy(t, indexer, proxyMode)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}
