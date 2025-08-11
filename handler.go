// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"fmt"
	"net/http"
	"time"
)

func notFoundHandler(err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notFoundError(w, err)
	})
}

func notFoundError(w http.ResponseWriter, err error) {
	noCacheHeaders(w)
	http.Error(w, err.Error(), http.StatusNotFound)
}

func badRequest(w http.ResponseWriter, errorMessage string) {
	noCacheHeaders(w)
	http.Error(w, errorMessage, http.StatusBadRequest)
}

func cacheHeaders(w http.ResponseWriter, cacheTime time.Duration) {
	maxAge := fmt.Sprintf("max-age=%.0f", cacheTime.Seconds())
	w.Header().Add("Cache-Control", maxAge)
	w.Header().Add("Cache-Control", "public")
}

func noCacheHeaders(w http.ResponseWriter) {
	w.Header().Add("Cache-Control", "max-age=0")
	w.Header().Add("Cache-Control", "private, no-store")
}

func jsonHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}
