// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"
	"go.elastic.co/apm/module/apmzap/v2"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/proxymode"
)

const (
	packageIndexRouterPath = "/package/{packageName:[a-z0-9_]+}/{packageVersion}/"
)

var errPackageRevisionNotFound = errors.New("package revision not found")

type packageIndexHandler struct {
	logger    *zap.Logger
	cacheTime time.Duration
	indexer   Indexer

	proxyMode *proxymode.ProxyMode
}

type packageIndexOption func(*packageIndexHandler)

func newPackageIndexHandler(logger *zap.Logger, indexer Indexer, cacheTime time.Duration, opts ...packageIndexOption) (*packageIndexHandler, error) {
	if indexer == nil {
		return nil, errors.New("indexer is required for package index handler")
	}
	if cacheTime <= 0 {
		return nil, errors.New("cache time must be greater than 0s")
	}

	h := &packageIndexHandler{
		logger:    logger,
		indexer:   indexer,
		cacheTime: cacheTime,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h, nil
}

func packageIndexWithProxy(pm *proxymode.ProxyMode) packageIndexOption {
	return func(h *packageIndexHandler) {
		h.proxyMode = pm
	}
}
func (h *packageIndexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	// Just this endpoint needs the full data, so we set it here.
	opts.FullData = true
	opts.IncludeDeprecatedNotice = true

	pkgs, err := h.indexer.Get(r.Context(), &opts)
	if err != nil {
		logger.Error("getting package path failed", zap.Error(err))
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
		notFoundError(w, errPackageRevisionNotFound)
		return
	}

	data, err := getPackageOutput(r.Context(), pkgs[0])
	if err != nil {
		logger.Error("marshaling package index failed",
			zap.String("package.path", pkgs[0].BasePath),
			zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	serveJSONResponse(r.Context(), w, h.cacheTime, data)
}

func getPackageOutput(ctx context.Context, pkg *packages.Package) ([]byte, error) {
	span, _ := apm.StartSpan(ctx, "Get Package Output", "app")
	defer span.End()

	return util.MarshalJSONPretty(pkg)
}
