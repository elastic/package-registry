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

type faviconOption func(*faviconHandler)

func newFaviconHandler(cacheTime time.Duration, opts ...faviconOption) (*faviconHandler, error) {
	if cacheTime <= 0 {
		return nil, errors.New("cache time must be greater than 0s")
	}

	h := &faviconHandler{
		cacheTime: cacheTime,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h, nil
}

func (h *faviconHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	cacheHeaders(w, h.cacheTime)
	w.Write(faviconBlob)
}
