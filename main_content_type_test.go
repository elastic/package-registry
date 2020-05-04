// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.
// +build linux

package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContentTypes(t *testing.T) {
	tests := []struct {
		endpoint            string
		expectedContentType string
	}{
		{"/index.json", "application/json"},
		{"/activemq-0.0.1.tar.gz", "application/gzip"},
		{"/favicon.ico", "image/vnd.microsoft.icon"},
		{"/metricbeat-mysql.png", "image/png"},
		{"/kibana-coredns.jpg", "image/jpeg"},
		{"/README.md", "text/markdown; charset=utf-8"},
		{"/logo_mysql.svg", "image/svg+xml"},
		{"/manifest.yml", "text/plain; charset=utf-8"},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
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
