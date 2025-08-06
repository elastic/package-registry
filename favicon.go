// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	_ "embed"
	"net/http"
	"time"
)

//go:embed img/favicon.ico
var faviconBlob []byte

func faviconHandler(cacheTime time.Duration) (func(w http.ResponseWriter, r *http.Request), error) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Return error if any query parameter is present
		if len(r.URL.Query()) > 0 {
			badRequest(w, "unknown query parameters")
			return
		}

		w.Header().Set("Content-Type", "image/x-icon")
		cacheHeaders(w, cacheTime)
		w.Write(faviconBlob)
	}, nil
}
