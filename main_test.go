// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

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

func TestEndpoints(t *testing.T) {
	packagesBasePaths := []string{"./testdata/second_package_path", "./testdata/package"}
	indexer := NewCombinedIndexer(
		packages.NewZipFileSystemIndexer(testLogger, "./testdata/local-storage"),
		packages.NewFileSystemIndexer(testLogger, packagesBasePaths...),
	)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	faviconHandleFunc, err := faviconHandler(testCacheTime)
	require.NoError(t, err)

	indexHandleFunc, err := indexHandler(testCacheTime)
	require.NoError(t, err)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/", "", "index.json", indexHandleFunc},
		{"/index.json", "", "index.json", indexHandleFunc},
		{"/search", "/search", "search.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?all=true", "/search", "search-all.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/categories", "/categories", "categories.json", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?experimental=true", "/categories", "categories-experimental.json", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?experimental=foo", "/categories", "categories-experimental-error.txt", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?experimental=true&kibana.version=6.5.2", "/categories", "categories-kibana652.json", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?prerelease=true", "/categories", "categories-prerelease.json", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?prerelease=foo", "/categories", "categories-prerelease-error.txt", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?prerelease=true&kibana.version=6.5.2", "/categories", "categories-prerelease-kibana652.json", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?include_policy_templates=true", "/categories", "categories-include-policy-templates.json", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?include_policy_templates=foo", "/categories", "categories-include-policy-templates-error.txt", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?capabilities=observability,security&prerelease=true", "/categories", "categories-prerelease-capabilities-observability-security.json", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?capabilities=none&prerelease=true", "/categories", "categories-prerelease-capabilities-none.json", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?spec.min=1.1&spec.max=2.10&prerelease=true", "/categories", "categories-spec-min-1.1.0-max-2.10.0.json", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?spec.max=2.10&prerelease=true", "/categories", "categories-spec-max-2.10.0.json", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/categories?spec.max=2.10.1&prerelease=true", "/categories", "categories-spec-max-error.txt", categoriesHandler(testLogger, indexer, testCacheTime)},
		{"/search?kibana.version=6.5.2", "/search", "search-kibana652.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?kibana.version=7.2.1", "/search", "search-kibana721.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?kibana.version=8.0.0", "/search", "search-kibana800.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?category=web", "/search", "search-category-web.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?category=observability", "/search", "search-category-observability-subcategories.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?category=web&all=true", "/search", "search-category-web-all.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?category=custom", "/search", "search-category-custom.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?package=example", "/search", "search-package-example.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?package=example&all=true", "/search", "search-package-example-all.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?experimental=true", "/search", "search-package-experimental.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?experimental=foo", "/search", "search-package-experimental-error.txt", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?category=datastore&experimental=true", "/search", "search-category-datastore.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?prerelease=true", "/search", "search-package-prerelease.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?prerelease=foo", "/search", "search-package-prerelease-error.txt", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?category=datastore&prerelease=true", "/search", "search-category-datastore-prerelease.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?type=content&prerelease=true", "/search", "search-content-packages.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?type=input&prerelease=true", "/search", "search-input-packages.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?type=input&package=integration_input&prerelease=true", "/search", "search-input-integration-package.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?type=integration&package=integration_input&prerelease=true", "/search", "search-integration-integration-package.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?capabilities=observability,security&prerelease=true", "/search", "search-prerelease-capabilities-observability-security.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?capabilities=none&prerelease=true", "/search", "search-prerelease-capabilities-none.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?spec.min=1.1&spec.max=2.10&prerelease=true", "/search", "search-spec-min-1.1.0-max-2.10.0.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?spec.max=2.10&prerelease=true", "/search", "search-spec-max-2.10.0.json", searchHandler(testLogger, indexer, testCacheTime)},
		{"/search?spec.max=2.10.1&prerelease=true", "/search", "search-spec-max-error.txt", searchHandler(testLogger, indexer, testCacheTime)},
		{"/favicon.ico", "", "favicon.ico", faviconHandleFunc},

		// Removed flags, kept to ensure that they don't break requests from old versions.
		{"/search?internal=true", "/search", "search-package-internal.json", searchHandler(testLogger, indexer, testCacheTime)},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestArtifacts(t *testing.T) {
	packagesBasePaths := []string{"./testdata/package"}
	indexer := packages.NewFileSystemIndexer(testLogger, packagesBasePaths...)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	artifactsHandler := artifactsHandler(testLogger, indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
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
	indexer := packages.NewZipFileSystemIndexer(testLogger, "./testdata/local-storage")

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	signaturesHandler := signaturesHandler(testLogger, indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
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
	packagesBasePaths := []string{"./testdata/package"}
	indexer := packages.NewFileSystemIndexer(testLogger, packagesBasePaths...)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	staticHandler := staticHandler(testLogger, indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
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
		packages.NewZipFileSystemIndexer(testLogger, "./testdata/local-storage"),
		packages.NewFileSystemIndexer(testLogger, "./testdata/package"),
	)
	err := indexer.Init(context.Background())
	require.NoError(t, err)

	router := mux.NewRouter()
	router.HandleFunc(staticRouterPath, staticHandler(testLogger, indexer, testCacheTime))

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
	indexer := packages.NewZipFileSystemIndexer(testLogger, "./testdata/local-storage")

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	artifactsHandler := artifactsHandler(testLogger, indexer, testCacheTime)

	staticHandler := staticHandler(testLogger, indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
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
	indexer := NewCombinedIndexer(
		packages.NewZipFileSystemIndexer(testLogger, "./testdata/local-storage"),
		packages.NewFileSystemIndexer(testLogger, "./testdata/package"),
	)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	packageIndexHandler := packageIndexHandler(testLogger, indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/package/example/1.0.0/", packageIndexRouterPath, "package.json", packageIndexHandler},
		{"/package/example/1.0.1/", packageIndexRouterPath, "package-zip.json", packageIndexHandler},
		{"/package/nodirentries/1.0.0/", packageIndexRouterPath, "package-zip-nodirentries.json", packageIndexHandler},
		{"/package/missing/1.0.0/", packageIndexRouterPath, "index-package-not-found.txt", packageIndexHandler},
		{"/package/example/999.0.0/", packageIndexRouterPath, "index-package-revision-not-found.txt", packageIndexHandler},
		{"/package/example/a.b.c/", packageIndexRouterPath, "index-package-invalid-version.txt", packageIndexHandler},
		{"/package/sql_input/1.0.1/", packageIndexRouterPath, "sql-input-package.json", packageIndexHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestZippedPackageIndex(t *testing.T) {
	packagesBasePaths := []string{"./testdata/local-storage"}
	indexer := packages.NewZipFileSystemIndexer(testLogger, packagesBasePaths...)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	packageIndexHandler := packageIndexHandler(testLogger, indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
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
	testPackagePath := filepath.Join("testdata", "package")
	secondPackagePath := filepath.Join("testdata", "second_package_path")
	packagesBasePaths := []string{secondPackagePath, testPackagePath}
	indexer := packages.NewFileSystemIndexer(testLogger, packagesBasePaths...)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	packageIndexHandler := packageIndexHandler(testLogger, indexer, testCacheTime)

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
		packages.NewZipFileSystemIndexer(testLogger, "./testdata/local-storage"),
		packages.NewFileSystemIndexer(testLogger, "./testdata/package"),
	)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	handler := staticHandler(testLogger, indexer, testCacheTime)
	router := mux.NewRouter()
	router.HandleFunc(staticRouterPath, handler)

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
	indexer := NewCombinedIndexer(
		packages.NewZipFileSystemIndexer(testLogger, "./testdata/local-storage"),
		packages.NewFileSystemIndexer(testLogger, "./testdata/package"),
	)

	err := indexer.Init(context.Background())
	require.NoError(t, err)

	router := mux.NewRouter()
	router.HandleFunc(staticRouterPath, staticHandler(testLogger, indexer, testCacheTime))
	router.HandleFunc(artifactsRouterPath, artifactsHandler(testLogger, indexer, testCacheTime))

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

func runEndpointWithHeaders(t *testing.T, endpoint, path, file string, headers map[string]string, handler func(w http.ResponseWriter, r *http.Request)) {
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

func runEndpoint(t *testing.T, endpoint, path, file string, handler func(w http.ResponseWriter, r *http.Request)) {
	recorder := recordRequest(t, endpoint, path, handler)

	assertExpectedBody(t, recorder.Body, file)

	// Skip cache check if 4xx error
	if recorder.Code >= 200 && recorder.Code < 300 {
		cacheTime := fmt.Sprintf("%.0f", testCacheTime.Seconds())
		assert.Equal(t, recorder.Header()["Cache-Control"], []string{"max-age=" + cacheTime, "public"})
	}
}

func recordRequest(t *testing.T, endpoint, path string, handler func(w http.ResponseWriter, r *http.Request)) *httptest.ResponseRecorder {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	router := mux.NewRouter()
	if path == "" {
		router.PathPrefix("/").HandlerFunc(handler)
	} else {
		router.HandleFunc(path, handler)
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
