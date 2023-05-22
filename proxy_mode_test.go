// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/proxymode"
)

func TestCategoriesWithProxyMode(t *testing.T) {
	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `[
  {
    "id": "custom",
    "title": "Custom",
    "count": 10
  },
  {
    "id": "custom_logs",
    "title": "Custom Logs",
    "count": 3,
    "parent_id": "custom",
    "parent_title": "Custom"
  }
]`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, response)
	}))
	defer webServer.Close()

	indexerProxy := packages.NewFileSystemIndexer(testLogger, "./testdata/second_package_path")
	err := indexerProxy.Init(context.Background())
	require.NoError(t, err)

	proxyMode, err := proxymode.NewProxyMode(
		testLogger,
		proxymode.ProxyOptions{
			Enabled: true,
			ProxyTo: webServer.URL,
		},
	)
	require.NoError(t, err)

	categoriesWithProxyHandler := categoriesHandlerWithProxyMode(testLogger, indexerProxy, proxyMode, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/categories", "/categories", "categories-proxy.json", categoriesWithProxyHandler},
		{"/categories?kibana.version=6.5.0", "/categoies", "categories-proxy-kibana-filter.json", categoriesWithProxyHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}
