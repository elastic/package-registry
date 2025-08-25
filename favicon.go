// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	_ "embed"
	"errors"
	"net/http"
	"time"
)

//go:embed img/favicon.ico
var faviconBlob []byte

type faviconHandler struct {
	cacheTime time.Duration
}

func newFaviconHandler(cacheTime time.Duration) (*faviconHandler, error) {
	if cacheTime <= 0 {
		return nil, errors.New("cache time must be greater than 0s")
	}

	return &faviconHandler{
		cacheTime: cacheTime,
	}, nil
}

func (h *faviconHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	cacheHeaders(w, h.cacheTime)
	w.Write(faviconBlob)
}
