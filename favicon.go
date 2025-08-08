// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	_ "embed"
	"errors"
	"net/http"
)

//go:embed img/favicon.ico
var faviconBlob []byte

func faviconHandler(options handlerOptions) (func(w http.ResponseWriter, r *http.Request), error) {
	if options.cacheTime < 0 {
		return nil, errors.New("cache time must be non-negative for favicon handler")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// Return error if any query parameter is present
		if !options.allowUnknownQueryParameters && len(r.URL.Query()) > 0 {
			badRequest(w, "not supported query parameters")
			return
		}

		w.Header().Set("Content-Type", "image/x-icon")
		cacheHeaders(w, options.cacheTime)
		w.Write(faviconBlob)
	}, nil
}
