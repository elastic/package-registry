// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/stretchr/testify/require"

	"github.com/elastic/package-registry/internal/database"
	internalStorage "github.com/elastic/package-registry/internal/storage"
	"github.com/elastic/package-registry/storage"
)

const (
	storageIndexerGoldenDir                 = "storage-indexer"
	defaultAllowUnknownQueryParametersTests = false
)

func generateSQLStorageIndexer(fs *fakestorage.Server, webServer string) (Indexer, error) {
	db, err := database.NewMemorySQLDB(database.MemorySQLDBOptions{Path: "main"})
	if err != nil {
		return nil, err
	}

	swapDb, err := database.NewMemorySQLDB(database.MemorySQLDBOptions{Path: "swap"})
	if err != nil {
		return nil, err
	}

	options, err := internalStorage.CreateFakeIndexerOptions(db, swapDb)
	if err != nil {
		return nil, err
	}
	options.PackageStorageEndpoint = webServer

	return internalStorage.NewIndexer(testLogger, fs.Client(), options), nil
}

func generateTestCaseStorageEndpoints(indexer Indexer) []struct {
	endpoint string
	path     string
	file     string
	handler  func(w http.ResponseWriter, r *http.Request)
} {
	return []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/search", "/search", "search.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?all=true", "/search", "search-all.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/categories", "/categories", "categories.json", categoriesHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/categories?experimental=true", "/categories", "categories-experimental.json", categoriesHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/categories?experimental=foo", "/categories", "categories-experimental-error.txt", categoriesHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/categories?experimental=true&kibana.version=6.5.2", "/categories", "categories-kibana652.json", categoriesHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/categories?prerelease=true", "/categories", "categories-prerelease.json", categoriesHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/categories?prerelease=foo", "/categories", "categories-prerelease-error.txt", categoriesHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/categories?prerelease=true&kibana.version=6.5.2", "/categories", "categories-prerelease-kibana652.json", categoriesHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/categories?include_policy_templates=true", "/categories", "categories-include-policy-templates.json", categoriesHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/categories?include_policy_templates=foo", "/categories", "categories-include-policy-templates-error.txt", categoriesHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?kibana.version=6.5.2", "/search", "search-kibana652.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?kibana.version=7.2.1", "/search", "search-kibana721.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?kibana.version=8.0.0", "/search", "search-kibana800.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?category=web", "/search", "search-category-web.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?category=web&all=true", "/search", "search-category-web-all.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?category=observability", "/search", "search-category-observability-subcategories.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?category=custom", "/search", "search-category-custom.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?experimental=true", "/search", "search-package-experimental.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?experimental=foo", "/search", "search-package-experimental-error.txt", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?category=datastore&experimental=true", "/search", "search-category-datastore.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?prerelease=true", "/search", "search-package-prerelease.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?prerelease=foo", "/search", "search-package-prerelease-error.txt", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
		{"/search?category=datastore&prerelease=true", "/search", "search-category-datastore-prerelease.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},

		// Removed flags, kept ensure that they don't break requests from old versions.
		{"/search?internal=true", "/search", "search-package-internal.json", searchHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)},
	}
}

func TestPackageStorage_Endpoints(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	indexer := storage.NewIndexer(testLogger, fs.Client(), storage.FakeIndexerOptions)
	defer indexer.Close(context.Background())

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestCaseStorageEndpoints(indexer)
	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestPackageStorageSQL_Endpoints(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	indexer, err := generateSQLStorageIndexer(fs, "")
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	err = indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestCaseStorageEndpoints(indexer)
	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func generateTestPackageIndexEndpoints(indexer Indexer) []struct {
	endpoint string
	path     string
	file     string
	handler  func(w http.ResponseWriter, r *http.Request)
} {
	packageIndexHandler := packageIndexHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)
	return []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/package/1password/0.1.1/", packageIndexRouterPath, "1password-0.1.1.json", packageIndexHandler},
		{"/package/kubernetes/0.3.0/", packageIndexRouterPath, "kubernetes-0.3.0.json", packageIndexHandler},
		{"/package/osquery/1.0.3/", packageIndexRouterPath, "osquery-1.0.3.json", packageIndexHandler},
	}
}

func TestPackageStorage_PackageIndex(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()
	indexer := storage.NewIndexer(testLogger, fs.Client(), storage.FakeIndexerOptions)
	defer indexer.Close(context.Background())

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestPackageIndexEndpoints(indexer)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestPackageSQLStorage_PackageIndex(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	indexer, err := generateSQLStorageIndexer(fs, "")
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	err = indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestPackageIndexEndpoints(indexer)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func generateTestArtifactsEndpoints(indexer Indexer) []struct {
	endpoint string
	path     string
	file     string
	handler  func(w http.ResponseWriter, r *http.Request)
} {
	artifactsHandler := artifactsHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)
	return []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/epr/1password/1password-0.1.1.zip", artifactsRouterPath, "1password-0.1.1.zip.txt", artifactsHandler},
		{"/epr/kubernetes/kubernetes-999.999.999.zip", artifactsRouterPath, "artifact-package-version-not-found.txt", artifactsHandler},
		{"/epr/missing/missing-1.0.3.zip", artifactsRouterPath, "artifact-package-not-found.txt", artifactsHandler},
	}
}

func TestPackageStorage_Artifacts(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.RequestURI)
	}))
	defer webServer.Close()

	testIndexerOptions := storage.FakeIndexerOptions
	testIndexerOptions.PackageStorageEndpoint = webServer.URL

	indexer := storage.NewIndexer(testLogger, fs.Client(), testIndexerOptions)
	defer indexer.Close(context.Background())

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestArtifactsEndpoints(indexer)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestPackageSQLStorage_Artifacts(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.RequestURI)
	}))
	defer webServer.Close()

	indexer, err := generateSQLStorageIndexer(fs, webServer.URL)
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	err = indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestArtifactsEndpoints(indexer)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func generateTestSignaturesEndpoints(indexer Indexer) []struct {
	endpoint string
	path     string
	file     string
	handler  func(w http.ResponseWriter, r *http.Request)
} {
	signaturesHandler := signaturesHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)
	return []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/epr/1password/1password-0.1.1.zip.sig", signaturesRouterPath, "1password-0.1.1.zip.sig", signaturesHandler},
		{"/epr/checkpoint/checkpoint-0.5.2.zip.sig", signaturesRouterPath, "checkpoint-0.5.2.zip.sig", signaturesHandler},
	}
}

func TestPackageStorage_Signatures(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.RequestURI)
	}))
	defer webServer.Close()

	testIndexerOptions := storage.FakeIndexerOptions
	testIndexerOptions.PackageStorageEndpoint = webServer.URL

	indexer := storage.NewIndexer(testLogger, fs.Client(), testIndexerOptions)
	defer indexer.Close(context.Background())

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestSignaturesEndpoints(indexer)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestPackageSQLStorage_Signatures(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.RequestURI)
	}))
	defer webServer.Close()

	indexer, err := generateSQLStorageIndexer(fs, webServer.URL)
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	err = indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestSignaturesEndpoints(indexer)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func generateTestStaticEndpoints(indexer Indexer) []struct {
	endpoint string
	path     string
	file     string
	handler  func(w http.ResponseWriter, r *http.Request)
} {
	staticHandler := staticHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)
	return []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/package/1password/0.1.1/img/1password-logo-light-bg.svg", staticRouterPath, "1password-logo-light-bg.svg", staticHandler},
		{"/package/cassandra/1.1.0/img/[Logs Cassandra] System Logs.jpg", staticRouterPath, "logs-cassandra-system-logs.jpg", staticHandler},
		{"/package/cef/0.1.0/docs/README.md", staticRouterPath, "cef-readme.md", staticHandler},
	}
}

func TestPackageStorage_Statics(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.RequestURI)
	}))
	defer webServer.Close()

	testIndexerOptions := storage.FakeIndexerOptions
	testIndexerOptions.PackageStorageEndpoint = webServer.URL

	indexer := storage.NewIndexer(testLogger, fs.Client(), testIndexerOptions)
	defer indexer.Close(context.Background())

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestStaticEndpoints(indexer)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestPackagesQLStorage_Statics(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.RequestURI)
	}))
	defer webServer.Close()

	indexer, err := generateSQLStorageIndexer(fs, webServer.URL)
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	err = indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestStaticEndpoints(indexer)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func generateTestResolveHeadersEndpoints(indexer Indexer) []struct {
	endpoint        string
	path            string
	file            string
	responseHeaders map[string]string
	handler         func(w http.ResponseWriter, r *http.Request)
} {
	staticHandler := staticHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)
	return []struct {
		endpoint        string
		path            string
		file            string
		responseHeaders map[string]string
		handler         func(w http.ResponseWriter, r *http.Request)
	}{
		{
			endpoint: "/package/1password/0.1.1/img/1password-logo-light-bg.svg",
			path:     staticRouterPath,
			file:     "1password-logo-light-bg.svg.response",
			responseHeaders: map[string]string{
				"Last-Modified": "time",
				"Content-Type":  "image/svg+xml",
			},
			handler: staticHandler,
		},
	}
}

func TestPackageStorage_ResolverHeadersResponse(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Foo", "bar")
		w.Header().Set("Last-Modified", "time")
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprintf(w, "%s\n%s\n%+v\n", r.Method, r.RequestURI, r.Header)
	}))
	defer webServer.Close()

	testIndexerOptions := storage.FakeIndexerOptions
	testIndexerOptions.PackageStorageEndpoint = webServer.URL

	indexer := storage.NewIndexer(testLogger, fs.Client(), testIndexerOptions)
	defer indexer.Close(context.Background())

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestResolveHeadersEndpoints(indexer)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexerAndHeaders(t, test.endpoint, test.path, test.file, test.responseHeaders, test.handler)
		})
	}
}

func TestPackageSQLStorage_ResolverHeadersResponse(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Foo", "bar")
		w.Header().Set("Last-Modified", "time")
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprintf(w, "%s\n%s\n%+v\n", r.Method, r.RequestURI, r.Header)
	}))
	defer webServer.Close()

	indexer, err := generateSQLStorageIndexer(fs, webServer.URL)
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	err = indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestResolveHeadersEndpoints(indexer)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexerAndHeaders(t, test.endpoint, test.path, test.file, test.responseHeaders, test.handler)
		})
	}
}

func generateTestResolveErrorResponseEndpoints(indexer Indexer) []struct {
	endpoint string
	path     string
	file     string
	handler  func(w http.ResponseWriter, r *http.Request)
} {
	staticHandler := staticHandler(testLogger, indexer, testCacheTime, defaultAllowUnknownQueryParametersTests)
	return []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{
			endpoint: "/package/1password/0.1.1/img/1password-logo-light-bg.svg",
			path:     staticRouterPath,
			file:     "1password-logo-light-bg.svg.error-response",
			handler:  staticHandler,
		},
	}
}

func TestPackageStorage_ResolverErrorResponse(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		message := fmt.Sprintf("internal error\n%s\n%s\n%+v\n", r.Method, r.RequestURI, r.Header)
		http.Error(w, message, http.StatusInternalServerError)
	}))
	defer webServer.Close()

	testIndexerOptions := storage.FakeIndexerOptions
	testIndexerOptions.PackageStorageEndpoint = webServer.URL

	indexer := storage.NewIndexer(testLogger, fs.Client(), testIndexerOptions)
	defer indexer.Close(context.Background())

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestResolveErrorResponseEndpoints(indexer)
	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestPackageSQLStorage_ResolverErrorResponse(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		message := fmt.Sprintf("internal error\n%s\n%s\n%+v\n", r.Method, r.RequestURI, r.Header)
		http.Error(w, message, http.StatusInternalServerError)
	}))
	defer webServer.Close()

	indexer, err := generateSQLStorageIndexer(fs, webServer.URL)
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	err = indexer.Init(context.Background())
	require.NoError(t, err)

	tests := generateTestResolveErrorResponseEndpoints(indexer)
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
