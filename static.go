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

func staticHandler(logger *zap.Logger, indexer Indexer, cacheTime time.Duration) http.HandlerFunc {
	return staticHandlerWithProxyMode(logger, indexer, proxymode.NoProxy(logger), cacheTime)
}

func staticHandlerWithProxyMode(logger *zap.Logger, indexer Indexer, proxyMode *proxymode.ProxyMode, cacheTime time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := logger.With(apmzap.TraceContext(r.Context())...)

		params, err := staticParamsFromRequest(r)
		if err != nil {
			badRequest(w, err.Error())
			return
		}

		// Return error if any query parameter is present
		if len(r.URL.Query()) > 0 {
			badRequest(w, "unknown query parameters")
			return
		}

		opts := packages.NameVersionFilter(params.packageName, params.packageVersion)
		pkgs, err := indexer.Get(r.Context(), &opts)
		if err != nil {
			logger.Error("getting package path failed",
				zap.String("package.name", params.packageName),
				zap.String("package.version", params.packageVersion),
				zap.Error(err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if len(pkgs) == 0 && proxyMode.Enabled() {
			proxiedPackage, err := proxyMode.Package(r)
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

		cacheHeaders(w, cacheTime)
		packages.ServePackageResource(logger, w, r, pkgs[0], params.fileName)
	}
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
