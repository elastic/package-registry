// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"errors"
	"net/http"

	"github.com/elastic/package-registry/internal/util"
)

type indexData struct {
	ServiceName string `json:"service.name"`
	Version     string `json:"service.version"`
}

func indexHandler(options handlerOptions) (func(w http.ResponseWriter, r *http.Request), error) {
	data := indexData{
		ServiceName: serviceName,
		Version:     version,
	}
	body, err := util.MarshalJSONPretty(&data)
	if err != nil {
		return nil, err
	}
	if options.cacheTime < 0 {
		return nil, errors.New("cache time must be non-negative for index handler")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// Return error if any query parameter is present
		if !options.allowUnknownQueryParameters && len(r.URL.Query()) > 0 {
			badRequest(w, "not supported query parameters")
			return
		}

		serveJSONResponse(r.Context(), w, options.cacheTime, body)
	}, nil
}
