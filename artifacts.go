// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"
	"go.elastic.co/apm/module/apmzap/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/proxymode"
)

const artifactsRouterPath = "/epr/{packageName}/{packageName:[a-z0-9_]+}-{packageVersion}.zip"

var errArtifactNotFound = errors.New("artifact not found")

type artifactsHandler struct {
	logger    *zap.Logger
	indexer   Indexer
	cacheTime time.Duration

	proxyMode *proxymode.ProxyMode
}

type artifactsOption func(*artifactsHandler)

func newArtifactsHandler(logger *zap.Logger, indexer Indexer, cacheTime time.Duration, opts ...artifactsOption) (*artifactsHandler, error) {
	if indexer == nil {
		return nil, errors.New("indexer is required for artifacts handler")
	}
	if cacheTime <= 0 {
		return nil, errors.New("cache time must be greater than 0s")
	}

	a := &artifactsHandler{
		logger:    logger,
		indexer:   indexer,
		cacheTime: cacheTime,
	}

	for _, opt := range opts {
		opt(a)
	}

	return a, nil
}

func artifactsWithProxy(pm *proxymode.ProxyMode) artifactsOption {
	return func(h *artifactsHandler) {
		h.proxyMode = pm
	}
}

func (h *artifactsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(apmzap.TraceContext(r.Context())...)

	vars := mux.Vars(r)
	packageName, ok := vars["packageName"]
	if !ok {
		badRequest(w, "missing package name")
		return
	}

	packageVersion, ok := vars["packageVersion"]
	if !ok {
		badRequest(w, "missing package version")
		return
	}

	_, err := semver.StrictNewVersion(packageVersion)
	if err != nil {
		badRequest(w, "invalid package version")
		return
	}

	opts := packages.NameVersionFilter(packageName, packageVersion)
	opts.SkipJSON = true

	pkgs, err := h.indexer.Get(r.Context(), &opts)
	if err != nil {
		logger.Error("getting package path failed",
			zap.String("package.name", packageName),
			zap.String("package.version", packageVersion),
			zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(pkgs) == 0 && h.proxyMode.Enabled() {
		proxiedPackage, err := h.proxyMode.Package(r)
		if err != nil {
			logger.Error("proxy mode: package failed", zap.Error(err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if proxiedPackage != nil {
			pkgs = pkgs.Join(packages.Packages{proxiedPackage})
		}
	}
	if len(pkgs) == 0 {
		notFoundError(w, errArtifactNotFound)
		return
	}

	cacheHeaders(w, h.cacheTime)
	packages.ServePackage(logger, w, r, pkgs[0])
}
