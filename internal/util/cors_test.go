// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestCORSHeaders(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/", func(http.ResponseWriter, *http.Request) {})
	router.Use(CORSMiddleware())

	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/", nil)

	router.ServeHTTP(recorder, request)

	allowOrigin := recorder.Header().Values("Access-Control-Allow-Origin")
	assert.Equal(t, []string{"*"}, allowOrigin)
}
