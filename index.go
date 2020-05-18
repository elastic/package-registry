// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"encoding/json"
	"net/http"
	"time"
)

type indexData struct {
	ServiceName string `json:"service.name"`
	Version     string `json:"version"`
}

func indexHandler(cacheTime time.Duration) (func(w http.ResponseWriter, r *http.Request), error) {
	data := indexData{
		ServiceName: serviceName,
		Version:     version,
	}
	body, err := json.MarshalIndent(&data, "", " ")
	if err != nil {
		return nil, err
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cacheHeaders(w, cacheTime)
		w.Write(body)
	}, nil
}
