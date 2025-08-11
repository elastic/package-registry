// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"net/http"
	"time"

	"github.com/elastic/package-registry/internal/util"
)

type indexData struct {
	ServiceName string `json:"service.name"`
	Version     string `json:"service.version"`
}

type indexHandler struct {
	cacheTime time.Duration
	body      []byte

	allowUnknownQueryParameters bool
}

type indexOption func(*indexHandler)

func newIndexHandler(cacheTime time.Duration, opts ...indexOption) (*indexHandler, error) {
	data := indexData{
		ServiceName: serviceName,
		Version:     version,
	}
	body, err := util.MarshalJSONPretty(&data)
	if err != nil {
		return nil, err
	}

	h := &indexHandler{
		cacheTime: cacheTime,
		body:      body,
	}
	for _, opt := range opts {
		opt(h)
	}

	return h, nil
}

func IndexWithAllowUnknownQueryParameters(allow bool) indexOption {
	return func(h *indexHandler) {
		h.allowUnknownQueryParameters = allow
	}
}

func (h *indexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Return error if any query parameter is present
	if !h.allowUnknownQueryParameters && len(r.URL.Query()) > 0 {
		badRequest(w, "not supported query parameters")
		return
	}

	serveJSONResponse(r.Context(), w, h.cacheTime, h.body)
}
