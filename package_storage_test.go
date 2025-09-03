// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
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

const storageIndexerGoldenDir = "storage-indexer"

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

func generateTestCaseStorageEndpoints(indexer Indexer) ([]struct {
	endpoint string
	path     string
	file     string
	handler  http.Handler
}, error) {
	searchHandler, err := newSearchHandler(testLogger, indexer, testCacheTime)
	if err != nil {
		return nil, err
	}
	categoriesHandler, err := newCategoriesHandler(testLogger, indexer, testCacheTime)
	if err != nil {
		return nil, err
	}
	disallowUnknownQueryParamsSearchHandler, err := newSearchHandler(testLogger, indexer, testCacheTime,
		searchWithAllowUnknownQueryParameters(false),
	)
	if err != nil {
		return nil, err
	}
	return []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		// TODO: Ensure that the these requests are the same as the ones defined in "main_test.go" to avoid regressions in storage indexers.
		{"/search", "/search", "search.json", searchHandler},
		{"/search?all=true", "/search", "search-all.json", searchHandler},
		{"/categories", "/categories", "categories.json", categoriesHandler},
		{"/categories?experimental=true", "/categories", "categories-experimental.json", categoriesHandler},
		{"/categories?experimental=foo", "/categories", "categories-experimental-error.txt", categoriesHandler},
		{"/categories?experimental=true&kibana.version=6.5.2", "/categories", "categories-kibana652.json", categoriesHandler},
		{"/categories?prerelease=true", "/categories", "categories-prerelease.json", categoriesHandler},
		{"/categories?prerelease=foo", "/categories", "categories-prerelease-error.txt", categoriesHandler},
		{"/categories?prerelease=true&kibana.version=6.5.2", "/categories", "categories-prerelease-kibana652.json", categoriesHandler},
		{"/categories?include_policy_templates=true", "/categories", "categories-include-policy-templates.json", categoriesHandler},
		{"/categories?include_policy_templates=foo", "/categories", "categories-include-policy-templates-error.txt", categoriesHandler},
		{"/categories?capabilities=observability,security&prerelease=true", "/categories", "categories-prerelease-capabilities-observability-security.json", categoriesHandler},
		{"/categories?capabilities=none&prerelease=true", "/categories", "categories-prerelease-capabilities-none.json", categoriesHandler},
		{"/categories?spec.min=1.1&spec.max=2.10&prerelease=true", "/categories", "categories-spec-min-1.1.0-max-2.10.0.json", categoriesHandler},
		{"/categories?spec.max=2.10&prerelease=true", "/categories", "categories-spec-max-2.10.0.json", categoriesHandler},
		{"/categories?spec.max=2.10.1&prerelease=true", "/categories", "categories-spec-max-error.txt", categoriesHandler},
		{"/categories?discovery=fields:process.pid&prerelease=true", "/categories", "categories-discovery-fields-process-pid.txt", categoriesHandler},
		{"/categories?discovery=datasets:good_content.errors&prerelease=true", "/categories", "categories-discovery-datasets.txt", categoriesHandler},
		{"/categories?discovery=datasets:good_content.errors&prerelease=true&discovery=fields:process.pid", "/categories", "categories-discovery-multiple.txt", categoriesHandler},
		{"/categories?discovery=datasets:good_content.errors&prerelease=true&discovery=fields:process.path", "/categories", "categories-discovery-multiple-no-match.txt", categoriesHandler},
		{"/search?kibana.version=6.5.2", "/search", "search-kibana652.json", searchHandler},
		{"/search?kibana.version=7.2.1", "/search", "search-kibana721.json", searchHandler},
		{"/search?kibana.version=8.0.0", "/search", "search-kibana800.json", searchHandler},
		{"/search?category=web", "/search", "search-category-web.json", searchHandler},
		{"/search?category=web&all=true", "/search", "search-category-web-all.json", searchHandler},
		{"/search?category=observability", "/search", "search-category-observability-subcategories.json", searchHandler},
		{"/search?category=custom", "/search", "search-category-custom.json", searchHandler},
		{"/search?package=example", "/search", "search-package-example.json", searchHandler},
		{"/search?package=example&all=true", "/search", "search-package-example-all.json", searchHandler},
		{"/search?experimental=true", "/search", "search-package-experimental.json", searchHandler},
		{"/search?experimental=foo", "/search", "search-package-experimental-error.txt", searchHandler},
		{"/search?category=datastore&experimental=true", "/search", "search-category-datastore.json", searchHandler},
		{"/search?prerelease=true", "/search", "search-package-prerelease.json", searchHandler},
		{"/search?prerelease=foo", "/search", "search-package-prerelease-error.txt", searchHandler},
		{"/search?category=datastore&prerelease=true", "/search", "search-category-datastore-prerelease.json", searchHandler},
		{"/search?type=content&prerelease=true", "/search", "search-content-packages.json", searchHandler},
		{"/search?type=input&prerelease=true", "/search", "search-input-packages.json", searchHandler},
		{"/search?type=input&package=integration_input&prerelease=true", "/search", "search-input-integration-package.json", searchHandler},
		{"/search?type=integration&package=integration_input&prerelease=true", "/search", "search-integration-integration-package.json", searchHandler},
		{"/search?capabilities=observability,security&prerelease=true", "/search", "search-prerelease-capabilities-observability-security.json", searchHandler},
		{"/search?capabilities=none&prerelease=true", "/search", "search-prerelease-capabilities-none.json", searchHandler},
		{"/search?spec.min=1.1&spec.max=2.10&prerelease=true", "/search", "search-spec-min-1.1.0-max-2.10.0.json", searchHandler},
		{"/search?spec.max=2.10&prerelease=true", "/search", "search-spec-max-2.10.0.json", searchHandler},
		{"/search?spec.max=2.10.1&prerelease=true", "/search", "search-spec-max-error.txt", searchHandler},
		{"/search?prerelease=true&discovery=fields:process.pid", "/search", "search-discovery-fields-process-pid.txt", searchHandler},
		{"/search?prerelease=true&discovery=fields:non.existing.field", "/search", "search-discovery-fields-empty.txt", searchHandler},
		{"/search?prerelease=true&discovery=datasets:good_content.errors", "/search", "search-discovery-datasets.txt", searchHandler},
		{"/search?prerelease=true&discovery=datasets:good_content.errors&discovery=fields:process.pid", "/search", "search-discovery-multiple.txt", searchHandler},
		{"/search?prerelease=true&discovery=datasets:good_content.errors&discovery=fields:process.path", "/search", "search-discovery-multiple-no-match.txt", searchHandler},

		// Removed flags, kept ensure that they don't break requests from old versions.
		{"/search?internal=true", "/search", "search-package-internal.json", searchHandler},

		// Test queries with unknown query parameters
		{"/search?package=yamlpipeline&unknown=true", "/search", "search-unknown-query-parameter-error.txt", disallowUnknownQueryParamsSearchHandler},
		{"/search?package=yamlpipeline&unknown=true", "/search", "search-allowed-unknown-query-parameter.json", searchHandler},
	}, nil
}

func TestPackageStorage_Endpoints(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()

	indexer := storage.NewIndexer(testLogger, fs.Client(), storage.FakeIndexerOptions)
	defer indexer.Close(t.Context())

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestCaseStorageEndpoints(indexer)
	require.NoError(t, err)

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
	defer indexer.Close(t.Context())

	err = indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestCaseStorageEndpoints(indexer)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func generateTestPackageIndexEndpoints(indexer Indexer) ([]struct {
	endpoint string
	path     string
	file     string
	handler  http.Handler
}, error) {
	packageIndexHandler, err := newPackageIndexHandler(testLogger, indexer, testCacheTime)
	if err != nil {
		return nil, err
	}
	return []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		{"/package/1password/0.1.1/", packageIndexRouterPath, "1password-0.1.1.json", packageIndexHandler},
		{"/package/kubernetes/0.3.0/", packageIndexRouterPath, "kubernetes-0.3.0.json", packageIndexHandler},
		{"/package/osquery/1.0.3/", packageIndexRouterPath, "osquery-1.0.3.json", packageIndexHandler},
	}, nil
}

func TestPackageStorage_PackageIndex(t *testing.T) {
	fs := internalStorage.PrepareFakeServer(t, "./storage/testdata/search-index-all-full.json")
	defer fs.Stop()
	indexer := storage.NewIndexer(testLogger, fs.Client(), storage.FakeIndexerOptions)
	defer indexer.Close(t.Context())

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestPackageIndexEndpoints(indexer)
	require.NoError(t, err)

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
	defer indexer.Close(t.Context())

	err = indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestPackageIndexEndpoints(indexer)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func generateTestArtifactsEndpoints(indexer Indexer) ([]struct {
	endpoint string
	path     string
	file     string
	handler  http.Handler
}, error) {
	artifactsHandler, err := newArtifactsHandler(testLogger, indexer, testCacheTime)
	if err != nil {
		return nil, err
	}
	return []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		{"/epr/1password/1password-0.1.1.zip", artifactsRouterPath, "1password-0.1.1.zip.txt", artifactsHandler},
		{"/epr/kubernetes/kubernetes-999.999.999.zip", artifactsRouterPath, "artifact-package-version-not-found.txt", artifactsHandler},
		{"/epr/missing/missing-1.0.3.zip", artifactsRouterPath, "artifact-package-not-found.txt", artifactsHandler},
	}, nil
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
	defer indexer.Close(t.Context())

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestArtifactsEndpoints(indexer)
	require.NoError(t, err)

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
	defer indexer.Close(t.Context())

	err = indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestArtifactsEndpoints(indexer)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func generateTestSignaturesEndpoints(indexer Indexer) ([]struct {
	endpoint string
	path     string
	file     string
	handler  http.Handler
}, error) {
	signaturesHandler, err := newSignaturesHandler(testLogger, indexer, testCacheTime)
	if err != nil {
		return nil, err
	}
	return []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		{"/epr/1password/1password-0.1.1.zip.sig", signaturesRouterPath, "1password-0.1.1.zip.sig", signaturesHandler},
		{"/epr/checkpoint/checkpoint-0.5.2.zip.sig", signaturesRouterPath, "checkpoint-0.5.2.zip.sig", signaturesHandler},
	}, nil
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
	defer indexer.Close(t.Context())

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestSignaturesEndpoints(indexer)
	require.NoError(t, err)

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
	defer indexer.Close(t.Context())

	err = indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestSignaturesEndpoints(indexer)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func generateTestStaticEndpoints(indexer Indexer) ([]struct {
	endpoint string
	path     string
	file     string
	handler  http.Handler
}, error) {
	staticHandler, err := newStaticHandler(testLogger, indexer, testCacheTime)
	if err != nil {
		return nil, err
	}
	return []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		{"/package/1password/0.1.1/img/1password-logo-light-bg.svg", staticRouterPath, "1password-logo-light-bg.svg", staticHandler},
		{"/package/cassandra/1.1.0/img/[Logs Cassandra] System Logs.jpg", staticRouterPath, "logs-cassandra-system-logs.jpg", staticHandler},
		{"/package/cef/0.1.0/docs/README.md", staticRouterPath, "cef-readme.md", staticHandler},
	}, nil
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
	defer indexer.Close(t.Context())

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestStaticEndpoints(indexer)
	require.NoError(t, err)

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
	defer indexer.Close(t.Context())

	err = indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestStaticEndpoints(indexer)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func generateTestResolveHeadersEndpoints(indexer Indexer) ([]struct {
	endpoint        string
	path            string
	file            string
	responseHeaders map[string]string
	handler         http.Handler
}, error) {
	staticHandler, err := newStaticHandler(testLogger, indexer, testCacheTime)
	if err != nil {
		return nil, err
	}
	return []struct {
		endpoint        string
		path            string
		file            string
		responseHeaders map[string]string
		handler         http.Handler
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
	}, nil
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
	defer indexer.Close(t.Context())

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestResolveHeadersEndpoints(indexer)
	require.NoError(t, err)

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
	defer indexer.Close(t.Context())

	err = indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestResolveHeadersEndpoints(indexer)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexerAndHeaders(t, test.endpoint, test.path, test.file, test.responseHeaders, test.handler)
		})
	}
}

func generateTestResolveErrorResponseEndpoints(indexer Indexer) ([]struct {
	endpoint string
	path     string
	file     string
	handler  http.Handler
}, error) {
	staticHandler, err := newStaticHandler(testLogger, indexer, testCacheTime)
	if err != nil {
		return nil, err
	}
	return []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		{
			endpoint: "/package/1password/0.1.1/img/1password-logo-light-bg.svg",
			path:     staticRouterPath,
			file:     "1password-logo-light-bg.svg.error-response",
			handler:  staticHandler,
		},
	}, nil
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
	defer indexer.Close(t.Context())

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestResolveErrorResponseEndpoints(indexer)
	require.NoError(t, err)

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
	defer indexer.Close(t.Context())

	err = indexer.Init(t.Context())
	require.NoError(t, err)

	tests, err := generateTestResolveErrorResponseEndpoints(indexer)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpointWithStorageIndexer(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func runEndpointWithStorageIndexer(t *testing.T, endpoint, path, file string, handler http.Handler) {
	runEndpoint(t, endpoint, path, filepath.Join(storageIndexerGoldenDir, file), handler)
}

func runEndpointWithStorageIndexerAndHeaders(t *testing.T, endpoint, path, file string, headers map[string]string, handler http.Handler) {
	runEndpointWithHeaders(t, endpoint, path, filepath.Join(storageIndexerGoldenDir, file), headers, handler)
}
