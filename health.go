// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"fmt"
	"net/http"
)

// healthHandler is used for Docker/K8s deployments. It returns 200 if the service is live
// In addition ?ready=true can be used for a ready request. Currently both are identical.
type healthHandler struct {
	allowUnknownQueryParameters bool
}

type healthOption func(*healthHandler)

func newHealthHandler(opts ...func(*healthHandler)) *healthHandler {
	h := &healthHandler{}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func healthWithAllowUnknownQueryParameters(allow bool) healthOption {
	return func(h *healthHandler) {
		h.allowUnknownQueryParameters = allow
	}
}

func (h *healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for k := range r.URL.Query() {
		switch k {
		case "ready":
			// Ready check, currently same as live check
		default:
			if !h.allowUnknownQueryParameters {
				badRequest(w, fmt.Sprintf("unknown query parameter: %s", k))
				return
			}
		}
	}
}
