// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
)

var (
	generateFlag       = flag.Bool("generate", false, "Write golden files")
	testCacheTime      = 1 * time.Second
	generatedFilesPath = filepath.Join("testdata", "generated")
	testLogger         = util.NewTestLogger()
)

func TestRouter(t *testing.T) {
	logger := util.NewTestLogger()
	config := defaultConfig
	indexer := NewCombinedIndexer()
	defer indexer.Close(t.Context())

	router, err := getRouter(logger, serverOptions{
		config:  &config,
		indexer: indexer,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/", nil)

	router.ServeHTTP(recorder, request)

	allowOrigin := recorder.Header().Values("Access-Control-Allow-Origin")
	assert.Equal(t, []string{"*"}, allowOrigin)
}

func TestEndpoints(t *testing.T) {
	t.Parallel()

	packagesBasePaths := []string{"./testdata/second_package_path", "./testdata/package"}
	indexer := NewCombinedIndexer(
		packages.NewZipFileSystemIndexer(testLogger, 0, "./testdata/local-storage"),
		packages.NewFileSystemIndexer(testLogger, 0, packagesBasePaths...),
	)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	faviconHandler, err := newFaviconHandler(testCacheTime)
	require.NoError(t, err)

	indexHandler, err := newIndexHandler(testCacheTime)
	require.NoError(t, err)

	searchHandler, err := newSearchHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)

	categoriesHandler, err := newCategoriesHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)

	disallowUnknownQueryParamsSearchHandler, err := newSearchHandler(testLogger, indexer, testCacheTime,
		searchWithAllowUnknownQueryParameters(false),
	)
	require.NoError(t, err)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		// TODO: Ensure that the these requests are the same as the ones defined in "package_storage_test.go" to avoid regressions.
		{"/", "", "index.json", indexHandler},
		{"/index.json", "", "index.json", indexHandler},
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
		{"/search?category=observability", "/search", "search-category-observability-subcategories.json", searchHandler},
		{"/search?category=web&all=true", "/search", "search-category-web-all.json", searchHandler},
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
		{"/favicon.ico", "", "favicon.ico", faviconHandler},

		{"/search?package=agent_version&agent.version=9.1.0", "/search", "search-agent-910.json", searchHandler},
		{"/search?package=agent_version&agent.version=9.5.0", "/search", "search-agent-950.json", searchHandler},
		{"/categories?agent.version=9.1.0", "/categories", "categories-agent-910.json", categoriesHandler},
		{"/categories?agent.version=9.5.0", "/categories", "categories-agent-950.json", categoriesHandler},

		// Removed flags, kept to ensure that they don't break requests from old versions.
		{"/search?internal=true", "/search", "search-package-internal.json", searchHandler},

		// Test queries with unknown query parameters
		{"/search?package=yamlpipeline&unknown=true", "/search", "search-unknown-query-parameter-error.txt", disallowUnknownQueryParamsSearchHandler},
		{"/search?package=yamlpipeline&unknown=true", "/search", "search-allowed-unknown-query-parameter.json", searchHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestArtifacts(t *testing.T) {
	t.Parallel()

	packagesBasePaths := []string{"./testdata/package"}
	indexer := packages.NewFileSystemIndexer(testLogger, 0, packagesBasePaths...)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	artifactsHandler, err := newArtifactsHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		{"/epr/example/example-0.0.2.zip", artifactsRouterPath, "example-0.0.2.zip-preview.txt", artifactsHandler},
		{"/epr/example/example-999.0.2.zip", artifactsRouterPath, "artifact-package-version-not-found.txt", artifactsHandler},
		{"/epr/example/missing-0.1.2.zip", artifactsRouterPath, "artifact-package-not-found.txt", artifactsHandler},
		{"/epr/example/example-a.b.c.zip", artifactsRouterPath, "artifact-package-invalid-version.txt", artifactsHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestSignatures(t *testing.T) {
	t.Parallel()

	indexer := packages.NewZipFileSystemIndexer(testLogger, 0, "./testdata/local-storage")
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	signaturesHandler, err := newSignaturesHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		{"/epr/example/example-1.0.1.zip.sig", signaturesRouterPath, "example-1.0.1.zip.sig", signaturesHandler},
		{"/epr/example/example-0.0.1.zip.sig", signaturesRouterPath, "missing-signature.txt", signaturesHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestStatics(t *testing.T) {
	t.Parallel()

	packagesBasePaths := []string{"./testdata/package"}
	indexer := packages.NewFileSystemIndexer(testLogger, 0, packagesBasePaths...)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	staticHandler, err := newStaticHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		{"/package/example/1.0.0/docs/README.md", staticRouterPath, "example-1.0.0-README.md", staticHandler},
		{"/package/example/1.0.0/img/kibana-envoyproxy.jpg", staticRouterPath, "example-1.0.0-screenshot.jpg", staticHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestStaticsModifiedTime(t *testing.T) {
	t.Parallel()

	const ifModifiedSinceHeader = "If-Modified-Since"
	const lastModifiedHeader = "Last-Modified"

	tests := []struct {
		title    string
		endpoint string
		headers  map[string]string
		code     int
	}{
		{
			title:    "No cache headers",
			endpoint: "/package/example/1.0.0/img/kibana-envoyproxy.jpg",
			code:     200,
		},
		{
			title:    "Doesn't exist",
			endpoint: "/package/none/1.0.0/img/foo.jpg",
			code:     404,
		},
		{
			title:    "Cached entry",
			endpoint: "/package/example/1.0.0/img/kibana-envoyproxy.jpg",
			headers: map[string]string{
				// Assuming that the file hasn't been modified in the future.
				ifModifiedSinceHeader: time.Now().UTC().Format(http.TimeFormat),
			},
			code: 304,
		},
		{
			title:    "Old cached entry",
			endpoint: "/package/example/1.0.0/img/kibana-envoyproxy.jpg",
			headers: map[string]string{
				ifModifiedSinceHeader: time.Time{}.Format(http.TimeFormat),
			},
			code: 200,
		},

		// From zip
		{
			title:    "No cache headers from zip",
			endpoint: "/package/example/1.0.1/img/kibana-envoyproxy.jpg",
			code:     200,
		},
		{
			title:    "Cached entry from zip",
			endpoint: "/package/example/1.0.1/img/kibana-envoyproxy.jpg",
			headers: map[string]string{
				// Assuming that the file hasn't been modified in the future.
				ifModifiedSinceHeader: time.Now().UTC().Format(http.TimeFormat),
			},
			code: 304,
		},
		{
			title:    "Old cached entry from zip",
			endpoint: "/package/example/1.0.1/img/kibana-envoyproxy.jpg",
			headers: map[string]string{
				ifModifiedSinceHeader: time.Time{}.Format(http.TimeFormat),
			},
			code: 200,
		},
	}

	indexer := NewCombinedIndexer(
		packages.NewZipFileSystemIndexer(testLogger, 0, "./testdata/local-storage"),
		packages.NewFileSystemIndexer(testLogger, 0, "./testdata/package"),
	)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	router := mux.NewRouter()
	staticHandler, err := newStaticHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)
	router.Handle(staticRouterPath, staticHandler)

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			req, err := http.NewRequest("GET", test.endpoint, nil)
			require.NoError(t, err)

			for k, v := range test.headers {
				req.Header.Add(k, v)
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.Equal(t, test.code, recorder.Code)
			if test.code >= 0 && test.code < 400 {
				assert.NotEmpty(t, recorder.Header().Values(lastModifiedHeader))
			}
		})
	}
}

func TestZippedArtifacts(t *testing.T) {
	t.Parallel()

	indexer := packages.NewZipFileSystemIndexer(testLogger, 0, "./testdata/local-storage")
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	artifactsHandler, err := newArtifactsHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)

	staticHandler, err := newStaticHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		{"/epr/example/example-1.0.1.zip", artifactsRouterPath, "example-1.0.1.zip-preview.txt", artifactsHandler},
		{"/epr/example/nodirentries-1.0.0.zip", artifactsRouterPath, "nodirentries-1.0.0.zip-preview.txt", artifactsHandler},
		{"/epr/example/example-999.0.2.zip", artifactsRouterPath, "artifact-package-version-not-found.txt", artifactsHandler},
		{"/package/example/1.0.1/docs/README.md", staticRouterPath, "example-1.0.1-README.md", staticHandler},
		{"/package/example/1.0.1/img/kibana-envoyproxy.jpg", staticRouterPath, "example-1.0.1-screenshot.jpg", staticHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestPackageIndex(t *testing.T) {
	t.Parallel()

	indexer := NewCombinedIndexer(
		packages.NewZipFileSystemIndexer(testLogger, 0, "./testdata/local-storage"),
		packages.NewFileSystemIndexer(testLogger, 0, "./testdata/package"),
	)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	packageIndexHandler, err := newPackageIndexHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		{"/package/example/1.0.0/", packageIndexRouterPath, "package.json", packageIndexHandler},
		{"/package/example/1.0.1/", packageIndexRouterPath, "package-zip.json", packageIndexHandler},
		{"/package/nodirentries/1.0.0/", packageIndexRouterPath, "package-zip-nodirentries.json", packageIndexHandler},
		{"/package/missing/1.0.0/", packageIndexRouterPath, "index-package-not-found.txt", packageIndexHandler},
		{"/package/example/999.0.0/", packageIndexRouterPath, "index-package-revision-not-found.txt", packageIndexHandler},
		{"/package/example/a.b.c/", packageIndexRouterPath, "index-package-invalid-version.txt", packageIndexHandler},
		{"/package/sql_input/1.0.1/", packageIndexRouterPath, "sql-input-package-not-found.json", packageIndexHandler},
		{"/package/sql_input/0.3.0/", packageIndexRouterPath, "sql-input-package.json", packageIndexHandler},
		{"/package/datasources/1.0.0/", packageIndexRouterPath, "datasources-1.0.0-package.json", packageIndexHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestZippedPackageIndex(t *testing.T) {
	t.Parallel()

	packagesBasePaths := []string{"./testdata/local-storage"}
	indexer := packages.NewZipFileSystemIndexer(testLogger, 0, packagesBasePaths...)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	packageIndexHandler, err := newPackageIndexHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  http.Handler
	}{
		{"/package/example/1.0.1/", packageIndexRouterPath, "package-zip.json", packageIndexHandler},
		{"/package/missing/1.0.0/", packageIndexRouterPath, "index-package-not-found.txt", packageIndexHandler},
		{"/package/example/999.0.0/", packageIndexRouterPath, "index-package-revision-not-found.txt", packageIndexHandler},
		{"/package/example/a.b.c/", packageIndexRouterPath, "index-package-invalid-version.txt", packageIndexHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

// TestAllPackageIndex generates and compares all index.json files for the test packages
func TestAllPackageIndex(t *testing.T) {
	t.Parallel()

	testPackagePath := filepath.Join("testdata", "package")
	secondPackagePath := filepath.Join("testdata", "second_package_path")
	packagesBasePaths := []string{secondPackagePath, testPackagePath}
	indexer := packages.NewFileSystemIndexer(testLogger, 0, packagesBasePaths...)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	packageIndexHandler, err := newPackageIndexHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)

	// find all manifests
	var manifests []string
	for _, path := range packagesBasePaths {
		m, err := filepath.Glob(filepath.Join(path, "*", "*", "manifest.yml"))
		require.NoError(t, err)
		manifests = append(manifests, m...)
	}

	type Test struct {
		PackageName    string `yaml:"name"`
		PackageVersion string `yaml:"version"`
	}
	var tests []Test
	for _, manifest := range manifests {
		var test Test
		d, err := os.ReadFile(manifest)
		require.NoError(t, err)
		err = yaml.Unmarshal(d, &test)
		require.NoError(t, err)
		tests = append(tests, test)
	}

	for _, test := range tests {
		t.Run(test.PackageName+"/"+test.PackageVersion, func(t *testing.T) {
			packageEndpoint := "/package/" + test.PackageName + "/" + test.PackageVersion + "/"
			fileName := filepath.Join("package", test.PackageName, test.PackageVersion, "index.json")
			runEndpoint(t, packageEndpoint, packageIndexRouterPath, fileName, packageIndexHandler)
		})
	}
}

func TestContentTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		endpoint    string
		contentType string
	}{
		{"/package/example/1.0.0/manifest.yml", "text/yaml; charset=UTF-8"},
		{"/package/example/1.0.0/docs/README.md", "text/markdown; charset=utf-8"},
		{"/package/example/1.0.0/img/kibana-envoyproxy.jpg", "image/jpeg"},

		// From zip
		{"/package/example/1.0.1/manifest.yml", "text/yaml; charset=UTF-8"},
		{"/package/example/1.0.1/docs/README.md", "text/markdown; charset=utf-8"},
		{"/package/example/1.0.1/img/kibana-envoyproxy.jpg", "image/jpeg"},
	}

	indexer := NewCombinedIndexer(
		packages.NewZipFileSystemIndexer(testLogger, 0, "./testdata/local-storage"),
		packages.NewFileSystemIndexer(testLogger, 0, "./testdata/package"),
	)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	staticHandler, err := newStaticHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)

	router := mux.NewRouter()
	router.Handle(staticRouterPath, staticHandler)

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req, err := http.NewRequest("GET", test.endpoint, nil)
			require.NoError(t, err)

			router.ServeHTTP(recorder, req)
			t.Logf("status response: %d", recorder.Code)

			assert.Equal(t, test.contentType, recorder.Header().Get("Content-Type"))
		})
	}
}

// TestRangeDownloads tests that range downloads continue working for packages stored
// on different file systems.
func TestRangeDownloads(t *testing.T) {
	t.Parallel()

	indexer := NewCombinedIndexer(
		packages.NewZipFileSystemIndexer(testLogger, 0, "./testdata/local-storage"),
		packages.NewFileSystemIndexer(testLogger, 0, "./testdata/package"),
	)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	err := indexer.Init(t.Context())
	require.NoError(t, err)

	router := mux.NewRouter()

	staticHandler, err := newStaticHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)
	router.Handle(staticRouterPath, staticHandler)

	artifactsHandler, err := newArtifactsHandler(testLogger, indexer, testCacheTime)
	require.NoError(t, err)
	router.Handle(artifactsRouterPath, artifactsHandler)

	tests := []struct {
		endpoint  string
		supported bool
		file      string
	}{
		{"/epr/example/example-0.0.2.zip", false, "example-0.0.2.zip-preview.txt"},
		{"/package/example/1.0.0/img/kibana-envoyproxy.jpg", true, "example-1.0.0-screenshot.jpg"},

		// zip
		{"/epr/example/example-1.0.1.zip", true, "example-1.0.1.zip-preview.txt"},
		{"/package/example/1.0.1/img/kibana-envoyproxy.jpg", true, "example-1.0.1-screenshot.jpg"},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			buf, supported := downloadWithRanges(t, router, test.endpoint)
			assert.Equal(t, test.supported, supported)
			if supported {
				assertExpectedBody(t, &buf, test.file)
			}
		})
	}
}

func runEndpointWithHeaders(t *testing.T, endpoint, path, file string, headers map[string]string, handler http.Handler) {
	recorder := recordRequest(t, endpoint, path, handler)

	assertExpectedBody(t, recorder.Body, file)

	// Skip cache check if 4xx error
	if recorder.Code >= 200 && recorder.Code < 300 {
		cacheTime := fmt.Sprintf("%.0f", testCacheTime.Seconds())
		assert.Equal(t, recorder.Header()["Cache-Control"], []string{"max-age=" + cacheTime, "public"})

		for key, value := range headers {
			log.Printf("Checking header %s", key)
			assert.Contains(t, recorder.Header(), key)
			assert.Equal(t, []string{value}, recorder.Header()[key])
		}
	}
}

func runEndpoint(t *testing.T, endpoint, path, file string, handler http.Handler) {
	recorder := recordRequest(t, endpoint, path, handler)

	assertExpectedBody(t, recorder.Body, file)

	// Skip cache check if 4xx error
	if recorder.Code >= 200 && recorder.Code < 300 {
		cacheTime := fmt.Sprintf("%.0f", testCacheTime.Seconds())
		assert.Equal(t, recorder.Header()["Cache-Control"], []string{"max-age=" + cacheTime, "public"})
	}
}

func recordRequest(t *testing.T, endpoint, path string, handler http.Handler) *httptest.ResponseRecorder {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	router := mux.NewRouter()
	if path == "" {
		router.PathPrefix("/").Handler(handler)
	} else {
		router.Handle(path, handler)
	}
	req.RequestURI = endpoint
	router.ServeHTTP(recorder, req)
	return recorder
}

type recordedBody interface {
	Bytes() []byte
}

func assertExpectedBody(t *testing.T, body recordedBody, expectedFile string) {
	fullPath := filepath.Join(generatedFilesPath, expectedFile)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	require.NoError(t, err)

	recorded := body.Bytes()
	if strings.HasSuffix(expectedFile, "-preview.txt") {
		recorded = listArchivedFiles(t, recorded)
	}

	if *generateFlag {
		err = os.WriteFile(fullPath, recorded, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, string(bytes.TrimSpace(data)), string(bytes.TrimSpace(recorded)))
}

func listArchivedFiles(t *testing.T, body []byte) []byte {
	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)

	var listing bytes.Buffer

	for _, f := range zipReader.File {
		// f.Name is populated from the zip file directly and is not validated for correctness.
		// Using filepath.ToSlash(f.Name) ensures that the file name has the expected format
		// regardless of the OS.
		listing.WriteString(fmt.Sprintf("%d %s\n", f.UncompressedSize64, filepath.ToSlash(f.Name)))
	}
	return listing.Bytes()
}

func downloadWithRanges(t *testing.T, handler http.Handler, endpoint string) (bytes.Buffer, bool) {
	var buf bytes.Buffer

	req, err := http.NewRequest("HEAD", endpoint, nil)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	ranges := recorder.Header().Get("Accept-Ranges")
	if ranges == "" {
		t.Logf("ranges not supported for endpoint (%s)", endpoint)
		return buf, false
	}
	if ranges != "bytes" {
		t.Fatalf("ranges supported in endpoint (%s), but not in bytes, found: %s", endpoint, ranges)
	}
	totalSize, err := strconv.ParseInt(recorder.Header().Get("Content-Length"), 10, 64)
	require.NoError(t, err)
	require.True(t, totalSize > 0)

	t.Logf("endpoint: %s, size: %d", endpoint, totalSize)

	maxSize := 100 * int64(1024)
	var start, end int64
	for {
		end = start + maxSize
		if end > totalSize {
			end = totalSize
		}
		req, err := http.NewRequest("GET", endpoint, nil)
		require.NoError(t, err)
		req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", start, end))

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		n, err := io.Copy(&buf, recorder.Body)
		require.NoError(t, err)
		require.GreaterOrEqual(t, maxSize+1, n)

		size, err := strconv.ParseInt(recorder.Header().Get("Content-Length"), 10, 64)
		require.NoError(t, err)
		if size < maxSize {
			break
		}
		start = start + size
	}

	return buf, true
}
