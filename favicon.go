// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	_ "embed"
	"net/http"
	"time"
)

//go:embed favicon.ico
var faviconICOBlob []byte

func faviconHandler(cacheTime time.Duration) (func(w http.ResponseWriter, r *http.Request), error) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		cacheHeaders(w, cacheTime)
		w.Write(faviconICOBlob)
	}, nil
}
