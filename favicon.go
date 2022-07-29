// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	_ "embed"
	"net/http"
	"strings"
	"time"
)

// Elastic Icon
//go:embed favicon.ico
var faviconICOBlob []byte

//go:embed favicon.svg
var faviconSVGBlob []byte

func faviconHandler(cacheTime time.Duration) (func(w http.ResponseWriter, r *http.Request), error) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response []byte
		switch {
		case strings.HasSuffix(r.URL.Path, ".ico"):
			w.Header().Set("Content-Type", "image/x-icon")
			response = faviconICOBlob
		case strings.HasSuffix(r.URL.Path, ".svg"):
			w.Header().Set("Content-Type", "image/svg+xml")
			response = faviconSVGBlob
		}
		w.Header().Set("Content-Type", "image/x-icon")

		cacheHeaders(w, cacheTime)
		w.Write(response)
	}, nil
}
