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

const staticRouterPath = "/package/{packageName}/{packageVersion}/{name:.*}"

type staticParams struct {
	packageName    string
	packageVersion string
	fileName       string
}

type staticHandler struct {
	logger    *zap.Logger
	indexer   Indexer
	cacheTime time.Duration

	proxyMode *proxymode.ProxyMode
}

type staticOption func(*staticHandler)

func newStaticHandler(logger *zap.Logger, indexer Indexer, cacheTime time.Duration, opts ...staticOption) (*staticHandler, error) {
	if indexer == nil {
		return nil, errors.New("indexer is required for static handler")
	}
	if cacheTime <= 0 {
		return nil, errors.New("cache time must be greater than 0s")
	}

	s := &staticHandler{
		logger:    logger,
		indexer:   indexer,
		cacheTime: cacheTime,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

func staticWithProxy(pm *proxymode.ProxyMode) staticOption {
	return func(h *staticHandler) {
		h.proxyMode = pm
	}
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(apmzap.TraceContext(r.Context())...)

	params, err := staticParamsFromRequest(r)
	if err != nil {
		badRequest(w, err.Error())
		return
	}

	opts := packages.NameVersionFilter(params.packageName, params.packageVersion)
	opts.SkipPackageData = true

	pkgs, err := h.indexer.Get(r.Context(), &opts)
	if err != nil {
		logger.Error("getting package path failed",
			zap.String("package.name", params.packageName),
			zap.String("package.version", params.packageVersion),
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
		notFoundError(w, errPackageRevisionNotFound)
		return
	}

	cacheHeaders(w, h.cacheTime)
	packages.ServePackageResource(logger, w, r, pkgs[0], params.fileName)
}

func staticParamsFromRequest(r *http.Request) (*staticParams, error) {
	vars := mux.Vars(r)
	packageName, ok := vars["packageName"]
	if !ok {
		return nil, errors.New("missing package name")
	}

	packageVersion, ok := vars["packageVersion"]
	if !ok {
		return nil, errors.New("missing package version")
	}

	_, err := semver.StrictNewVersion(packageVersion)
	if err != nil {
		return nil, errors.New("invalid package version")
	}

	fileName, ok := vars["name"]
	if !ok {
		return nil, errors.New("missing file name")
	}

	params := staticParams{
		packageName:    packageName,
		packageVersion: packageVersion,
		fileName:       fileName,
	}
	return &params, nil
}
