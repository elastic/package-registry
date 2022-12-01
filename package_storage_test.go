// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/package-registry/storage"
)

const storageIndexerGoldenDir = "storage-indexer"

func TestPackageStorage_Endpoints(t *testing.T) {
	fs := storage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()
	indexer := storage.NewIndexer(fs.Client(), storage.FakeIndexerOptions)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/search", "/search", "search.json", searchHandler(indexer, testCacheTime)},
		{"/search?all=true", "/search", "search-all.json", searchHandler(indexer, testCacheTime)},
		{"/categories", "/categories", "categories.json", categoriesHandler(indexer, testCacheTime)},
		{"/categories?experimental=true", "/categories", "categories-experimental.json", categoriesHandler(indexer, testCacheTime)},
		{"/categories?experimental=foo", "/categories", "categories-experimental-error.txt", categoriesHandler(indexer, testCacheTime)},
		{"/categories?experimental=true&kibana.version=6.5.2", "/categories", "categories-kibana652.json", categoriesHandler(indexer, testCacheTime)},
		{"/categories?prerelease=true", "/categories", "categories-prerelease.json", categoriesHandler(indexer, testCacheTime)},
		{"/categories?prerelease=foo", "/categories", "categories-prerelease-error.txt", categoriesHandler(indexer, testCacheTime)},
		{"/categories?prerelease=true&kibana.version=6.5.2", "/categories", "categories-prerelease-kibana652.json", categoriesHandler(indexer, testCacheTime)},
		{"/categories?include_policy_templates=true", "/categories", "categories-include-policy-templates.json", categoriesHandler(indexer, testCacheTime)},
		{"/categories?include_policy_templates=foo", "/categories", "categories-include-policy-templates-error.txt", categoriesHandler(indexer, testCacheTime)},
		{"/search?kibana.version=6.5.2", "/search", "search-kibana652.json", searchHandler(indexer, testCacheTime)},
		{"/search?kibana.version=7.2.1", "/search", "search-kibana721.json", searchHandler(indexer, testCacheTime)},
		{"/search?kibana.version=8.0.0", "/search", "search-kibana800.json", searchHandler(indexer, testCacheTime)},
		{"/search?category=web", "/search", "search-category-web.json", searchHandler(indexer, testCacheTime)},
		{"/search?category=web&all=true", "/search", "search-category-web-all.json", searchHandler(indexer, testCacheTime)},
		{"/search?category=custom", "/search", "search-category-custom.json", searchHandler(indexer, testCacheTime)},
		{"/search?experimental=true", "/search", "search-package-experimental.json", searchHandler(indexer, testCacheTime)},
		{"/search?experimental=foo", "/search", "search-package-experimental-error.txt", searchHandler(indexer, testCacheTime)},
		{"/search?category=datastore&experimental=true", "/search", "search-category-datastore.json", searchHandler(indexer, testCacheTime)},
		{"/search?prerelease=true", "/search", "search-package-prerelease.json", searchHandler(indexer, testCacheTime)},
		{"/search?prerelease=foo", "/search", "search-package-prerelease-error.txt", searchHandler(indexer, testCacheTime)},
		{"/search?category=datastore&prerelease=true", "/search", "search-category-datastore-prerelease.json", searchHandler(indexer, testCacheTime)},

		// Removed flags, kept ensure that they don't break requests from old versions.
		{"/search?internal=true", "/search", "search-package-internal.json", searchHandler(indexer, testCacheTime)},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestPackageStorage_PackageIndex(t *testing.T) {
	fs := storage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()
	indexer := storage.NewIndexer(fs.Client(), storage.FakeIndexerOptions)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	packageIndexHandler := packageIndexHandler(indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/package/1password/0.1.1/", packageIndexRouterPath, "1password-0.1.1.json", packageIndexHandler},
		{"/package/kubernetes/0.3.0/", packageIndexRouterPath, "kubernetes-0.3.0.json", packageIndexHandler},
		{"/package/osquery/1.0.3/", packageIndexRouterPath, "osquery-1.0.3.json", packageIndexHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestPackageStorage_Artifacts(t *testing.T) {
	fs := storage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.RequestURI)
	}))
	defer webServer.Close()

	testIndexerOptions := storage.FakeIndexerOptions
	testIndexerOptions.PackageStorageEndpoint = webServer.URL

	indexer := storage.NewIndexer(fs.Client(), testIndexerOptions)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	artifactsHandler := artifactsHandler(indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/epr/1password/1password-0.1.1.zip", artifactsRouterPath, "1password-0.1.1.zip.txt", artifactsHandler},
		{"/epr/kubernetes/kubernetes-999.999.999.zip", artifactsRouterPath, "artifact-package-version-not-found.txt", artifactsHandler},
		{"/epr/missing/missing-1.0.3.zip", artifactsRouterPath, "artifact-package-not-found.txt", artifactsHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestPackageStorage_Signatures(t *testing.T) {
	fs := storage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.RequestURI)
	}))
	defer webServer.Close()

	testIndexerOptions := storage.FakeIndexerOptions
	testIndexerOptions.PackageStorageEndpoint = webServer.URL

	indexer := storage.NewIndexer(fs.Client(), testIndexerOptions)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	signaturesHandler := signaturesHandler(indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/epr/1password/1password-0.1.1.zip.sig", signaturesRouterPath, "1password-0.1.1.zip.sig", signaturesHandler},
		{"/epr/checkpoint/checkpoint-0.5.2.zip.sig", signaturesRouterPath, "checkpoint-0.5.2.zip.sig", signaturesHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestPackageStorage_Statics(t *testing.T) {
	fs := storage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.RequestURI)
	}))
	defer webServer.Close()

	testIndexerOptions := storage.FakeIndexerOptions
	testIndexerOptions.PackageStorageEndpoint = webServer.URL

	indexer := storage.NewIndexer(fs.Client(), testIndexerOptions)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	staticHandler := staticHandler(indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/package/1password/0.1.1/img/1password-logo-light-bg.svg", staticRouterPath, "1password-logo-light-bg.svg", staticHandler},
		{"/package/cassandra/1.1.0/img/[Logs Cassandra] System Logs.jpg", staticRouterPath, "logs-cassandra-system-logs.jpg", staticHandler},
		{"/package/cef/0.1.0/docs/README.md", staticRouterPath, "cef-readme.md", staticHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}

}

func TestPackageStorage_ResolverHeadersResponse(t *testing.T) {
	fs := storage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Foo", "bar")
		w.Header().Set("Last-Modified", "time")
		fmt.Fprintf(w, "%s\n%s\n%+v\n", r.Method, r.RequestURI, r.Header)
	}))
	defer webServer.Close()

	testIndexerOptions := storage.FakeIndexerOptions
	testIndexerOptions.PackageStorageEndpoint = webServer.URL

	indexer := storage.NewIndexer(fs.Client(), testIndexerOptions)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	staticHandler := staticHandler(indexer, testCacheTime)

	tests := []struct {
		endpoint        string
		path            string
		file            string
		responseHeaders map[string]string
		handler         func(w http.ResponseWriter, r *http.Request)
	}{
		{
			endpoint:        "/package/1password/0.1.1/img/1password-logo-light-bg.svg",
			path:            staticRouterPath,
			file:            "1password-logo-light-bg.svg.response",
			responseHeaders: map[string]string{"Last-Modified": "time"},
			handler:         staticHandler,
		},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexerAndHeaders(t, test.endpoint, test.path, test.file, test.responseHeaders, test.handler)
		})
	}
}

func TestPackageStorage_ResolverErrorResponse(t *testing.T) {
	fs := storage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		message := fmt.Sprintf("internal error\n%s\n%s\n%+v\n", r.Method, r.RequestURI, r.Header)
		http.Error(w, message, http.StatusInternalServerError)
	}))
	defer webServer.Close()

	testIndexerOptions := storage.FakeIndexerOptions
	testIndexerOptions.PackageStorageEndpoint = webServer.URL

	indexer := storage.NewIndexer(fs.Client(), testIndexerOptions)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	staticHandler := staticHandler(indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{
			endpoint: "/package/1password/0.1.1/img/1password-logo-light-bg.svg",
			path:     staticRouterPath,
			file:     "1password-logo-light-bg.svg.response",
			handler:  staticHandler,
		},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}

}

func runEndpointWithStorageIndexer(t *testing.T, endpoint, path, file string, handler func(w http.ResponseWriter, r *http.Request)) {
	runEndpoint(t, endpoint, path, filepath.Join(storageIndexerGoldenDir, file), handler)
}

func runEndpointWithStorageIndexerAndHeaders(t *testing.T, endpoint, path, file string, headers map[string]string, handler func(w http.ResponseWriter, r *http.Request)) {
	runEndpointWithHeaders(t, endpoint, path, filepath.Join(storageIndexerGoldenDir, file), headers, handler)
}
