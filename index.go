// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"errors"
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
}

func newIndexHandler(cacheTime time.Duration) (*indexHandler, error) {
	if cacheTime <= 0 {
		return nil, errors.New("cache time must be greater than 0s")
	}
	data := indexData{
		ServiceName: serviceName,
		Version:     version,
	}
	body, err := util.MarshalJSONPretty(&data)
	if err != nil {
		return nil, err
	}

	return &indexHandler{
		cacheTime: cacheTime,
		body:      body,
	}, nil
}

func (h *indexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serveJSONResponse(r.Context(), w, h.cacheTime, h.body)
}
