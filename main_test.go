// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
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

	"github.com/elastic/package-registry/util"
)

var (
	generateFlag       = flag.Bool("generate", false, "Write golden files")
	testCacheTime      = 1 * time.Second
	generatedFilesPath = filepath.Join("testdata", "generated")
)

func TestEndpoints(t *testing.T) {
	packagesBasePaths := []string{"./testdata/second_package_path", "./testdata/package"}
	indexer := util.NewFilesystemIndexer(packagesBasePaths...)

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
		{"/search", "/search", "search.json", searchHandler(indexer, testCacheTime)},
		//{"/search?all=true", "/search", "search-all.json", searchHandler(indexer, testCacheTime)},
		{"/categories", "/categories", "categories.json", categoriesHandler(indexer, testCacheTime)},
		{"/categories?experimental=true", "/categories", "categories-experimental.json", categoriesHandler(indexer, testCacheTime)},
		{"/categories?experimental=foo", "/categories", "categories-experimental-error.json", categoriesHandler(indexer, testCacheTime)},
		{"/categories?experimental=true&kibana.version=6.5.2", "/categories", "categories-kibana652.json", categoriesHandler(indexer, testCacheTime)},
		{"/categories?include_policy_templates=true", "/categories", "categories-include-policy-templates.json", categoriesHandler(indexer, testCacheTime)},
		{"/categories?include_policy_templates=foo", "/categories", "categories-include-policy-templates-error.json", categoriesHandler(indexer, testCacheTime)},
		//{"/search?kibana.version=6.5.2", "/search", "search-kibana652.json", searchHandler(indexer, testCacheTime)},
		//{"/search?kibana.version=7.2.1", "/search", "search-kibana721.json", searchHandler(indexer, testCacheTime)},
		//{"/search?category=web", "/search", "search-category-web.json", searchHandler(indexer, testCacheTime)},
		//{"/search?category=custom", "/search", "search-category-custom.json", searchHandler(indexer, testCacheTime)},
		{"/search?package=example", "/search", "search-package-example.json", searchHandler(indexer, testCacheTime)},
		{"/search?package=example&all=true", "/search", "search-package-example-all.json", searchHandler(indexer, testCacheTime)},
		//{"/search?internal=true", "/search", "search-package-internal.json", searchHandler(indexer, testCacheTime)},
		{"/search?internal=bar", "/search", "search-package-internal-error.json", searchHandler(indexer, testCacheTime)},
		//{"/search?experimental=true", "/search", "search-package-experimental.json", searchHandler(indexer, testCacheTime)},
		{"/search?experimental=foo", "/search", "search-package-experimental-error.json", searchHandler(indexer, testCacheTime)},
		{"/favicon.ico", "", "favicon.ico", faviconHandleFunc},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

func TestArtifacts(t *testing.T) {
	packagesBasePaths := []string{"./testdata/package"}
	indexer := util.NewFilesystemIndexer(packagesBasePaths...)

	artifactsHandler := artifactsHandler(indexer, testCacheTime)

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

func TestStatics(t *testing.T) {
	packagesBasePaths := []string{"./testdata/package"}
	indexer := util.NewFilesystemIndexer(packagesBasePaths...)

	staticHandler := staticHandler(indexer, testCacheTime)

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

func TestZippedArtifacts(t *testing.T) {
	indexer := util.NewZipFilesystemIndexer("./testdata/local-storage")

	artifactsHandler := artifactsHandler(indexer, testCacheTime)

	staticHandler := staticHandler(indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/epr/example/example-1.0.1.zip", artifactsRouterPath, "example-1.0.1.zip-preview.txt", artifactsHandler},
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
		util.NewFilesystemIndexer("./testdata/package"),
		util.NewZipFilesystemIndexer("./testdata/local-storage"),
	)

	packageIndexHandler := packageIndexHandler(indexer, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/package/example/1.0.0/", packageIndexRouterPath, "package.json", packageIndexHandler},
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

func TestZippedPackageIndex(t *testing.T) {
	packagesBasePaths := []string{"./testdata/local-storage"}
	indexer := util.NewZipFilesystemIndexer(packagesBasePaths...)

	packageIndexHandler := packageIndexHandler(indexer, testCacheTime)

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
	indexer := util.NewFilesystemIndexer(packagesBasePaths...)
	packageIndexHandler := packageIndexHandler(indexer, testCacheTime)

	// find all manifests
	var manifests []string
	for _, path := range packagesBasePaths {
		m, err := filepath.Glob(path + "/*/*/manifest.yml")
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
		d, err := ioutil.ReadFile(manifest)
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
		util.NewFilesystemIndexer("./testdata/package"),
		util.NewZipFilesystemIndexer("./testdata/local-storage"),
	)
	handler := staticHandler(indexer, testCacheTime)
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
		util.NewFilesystemIndexer("./testdata/package"),
		util.NewZipFilesystemIndexer("./testdata/local-storage"),
	)

	router := mux.NewRouter()
	router.HandleFunc(staticRouterPath, staticHandler(indexer, testCacheTime))
	router.HandleFunc(artifactsRouterPath, artifactsHandler(indexer, testCacheTime))

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

func runEndpoint(t *testing.T, endpoint, path, file string, handler func(w http.ResponseWriter, r *http.Request)) {
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

	assertExpectedBody(t, recorder.Body, file)

	// Skip cache check if 4xx error
	if recorder.Code >= 200 && recorder.Code < 300 {
		cacheTime := fmt.Sprintf("%.0f", testCacheTime.Seconds())
		assert.Equal(t, recorder.Header()["Cache-Control"], []string{"max-age=" + cacheTime, "public"})
	}
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
		err = ioutil.WriteFile(fullPath, recorded, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	data, err := ioutil.ReadFile(fullPath)
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
		listing.WriteString(fmt.Sprintf("%d %s\n", f.UncompressedSize64, f.Name))

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
