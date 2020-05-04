// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/stretchr/testify/assert"
)

var (
	generateFlag  = flag.Bool("generate", false, "Write golden files")
	testCacheTime = 1 * time.Second
)

func TestEndpoints(t *testing.T) {

	publicPath := "./testdata/public"
	packagesBasePath := publicPath + "/package"

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/", "", "info.json", catchAll(http.Dir(publicPath), testCacheTime)},
		{"/search", "/search", "search.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/search?all=true", "/search", "search-all.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/categories", "/categories", "categories.json", categoriesHandler(packagesBasePath, testCacheTime)},
		{"/categories?experimental=true", "/categories", "categories-experimental.json", categoriesHandler(packagesBasePath, testCacheTime)},
		{"/categories?experimental=foo", "/categories", "categories-experimental-error.json", categoriesHandler(packagesBasePath, testCacheTime)},
		{"/search?kibana=6.5.2", "/search", "search-kibana652.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/search?kibana=7.2.1", "/search", "search-kibana721.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/search?category=metrics", "/search", "search-category-metrics.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/search?category=logs", "/search", "search-category-logs.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/search?package=example", "/search", "search-package-example.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/search?package=example&all=true", "/search", "search-package-example-all.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/search?internal=true", "/search", "search-package-internal.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/search?internal=bar", "/search", "search-package-internal-error.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/search?experimental=true", "/search", "search-package-experimental.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/search?experimental=foo", "/search", "search-package-experimental-error.json", searchHandler(packagesBasePath, testCacheTime)},
		{"/package/example/1.0.0", "", "package.json", catchAll(http.Dir(publicPath), testCacheTime)},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
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

	fullPath := "./docs/api/" + file

	if *generateFlag {
		err = ioutil.WriteFile(fullPath, recorder.Body.Bytes(), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, strings.TrimSpace(string(data)), strings.TrimSpace(recorder.Body.String()))

	// Skip cache check if 400 error
	if recorder.Code != 400 {
		cacheTime := fmt.Sprintf("%.0f", testCacheTime.Seconds())
		assert.Equal(t, recorder.Header()["Cache-Control"], []string{"max-age=" + cacheTime, "public"})
	}
}

func TestContentTypes(t *testing.T) {
	tests := []struct {
		endpoint string
		expectedContentType string
	}{
		{"/index.json", "application/json"},
		{"/activemq-0.0.1.tar.gz", "application/x-gzip"},
		{"/favicon.ico", "image/x-icon"},
		{"/metricbeat-mysql.png", "image/png"},
		{"/kibana-coredns.jpg", "image/jpeg"},
		{"/README.md", "text/plain; charset=utf-8"},
		{"/logo_mysql.svg", "image/svg+xml"},
		{"/manifest.yml", "text/plain; charset=utf-8"},
	}

	for _, test := range tests {
		t.Run(test.	endpoint, func(t *testing.T) {
			runContentType(t, test.endpoint, test.expectedContentType)
		})
	}
}

func runContentType(t *testing.T, endpoint, expectedContentType string) {
	publicPath := "./testdata/content-types"

	recorder := httptest.NewRecorder()
	h := catchAll(http.Dir(publicPath), testCacheTime)
	h(recorder, &http.Request{
		URL: &url.URL{
			Path: endpoint,
		},
	})

	assert.Equal(t, expectedContentType, recorder.Header().Get("Content-Type"))
}
