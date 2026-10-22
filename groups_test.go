// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/package-registry/internal/util"
)

func getGroups(t *testing.T, groupsFile string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	config := defaultConfig
	config.GroupsFile = groupsFile
	indexer := NewCombinedIndexer()
	defer indexer.Close(t.Context())

	router, err := getRouter(util.NewTestLogger(), serverOptions{config: &config, indexer: indexer})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/groups", nil)
	router.ServeHTTP(recorder, request)
	return recorder, recorder.Body.Bytes()
}

func TestGroups(t *testing.T) {
	recorder, body := getGroups(t, "testdata/groups/groups.yml")
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	expected := `[
  {
    "name": "apache",
    "title": "Apache HTTP Server",
    "icon": "apache",
    "schemas": {"ecs": {"package": "apache"}}
  },
  {
    "name": "nginx",
    "title": "Nginx",
    "description": "Collect logs and metrics from Nginx.",
    "icon": "nginx",
    "schemas": {
      "ecs": {"package": "nginx"},
      "otel": {"package": "nginx_otel", "default": true}
    }
  }
]`
	assert.JSONEq(t, expected, string(body))
}

func TestGroupsNoFile(t *testing.T) {
	recorder, body := getGroups(t, "")
	require.Equal(t, http.StatusOK, recorder.Code)
	var groups []any
	require.NoError(t, json.Unmarshal(body, &groups))
	assert.Empty(t, groups)
	assert.Equal(t, "[]", strings.TrimSpace(string(body)))
}

func TestGroupsMissingFile(t *testing.T) {
	config := defaultConfig
	config.GroupsFile = "testdata/groups/missing.yml"
	indexer := NewCombinedIndexer()
	defer indexer.Close(t.Context())
	_, err := getRouter(util.NewTestLogger(), serverOptions{config: &config, indexer: indexer})
	assert.Error(t, err)
}
