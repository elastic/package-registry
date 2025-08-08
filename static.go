// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"errors"
	"net/http"

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

func staticHandler(logger *zap.Logger, options handlerOptions) (http.HandlerFunc, error) {
	options.proxyMode = proxymode.NoProxy(logger)
	return staticHandlerWithProxyMode(logger, options)
}

func staticHandlerWithProxyMode(logger *zap.Logger, options handlerOptions) (http.HandlerFunc, error) {
	if options.proxyMode == nil {
		logger.Warn("packageIndexHandlerWithProxyMode called without proxy mode, defaulting to no proxy")
		options.proxyMode = proxymode.NoProxy(logger)
	}
	if options.cacheTime < 0 {
		return nil, errors.New("cache time must be non-negative for static handler")
	}
	if options.indexer == nil {
		return nil, errors.New("indexer is required for static handler")
	}

	return func(w http.ResponseWriter, r *http.Request) {
		logger := logger.With(apmzap.TraceContext(r.Context())...)

		params, err := staticParamsFromRequest(r)
		if err != nil {
			badRequest(w, err.Error())
			return
		}

		// Return error if any query parameter is present
		if !options.allowUnknownQueryParameters && len(r.URL.Query()) > 0 {
			badRequest(w, "not supported query parameters")
			return
		}

		opts := packages.NameVersionFilter(params.packageName, params.packageVersion)
		pkgs, err := options.indexer.Get(r.Context(), &opts)
		if err != nil {
			logger.Error("getting package path failed",
				zap.String("package.name", params.packageName),
				zap.String("package.version", params.packageVersion),
				zap.Error(err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if len(pkgs) == 0 && options.proxyMode.Enabled() {
			proxiedPackage, err := options.proxyMode.Package(r)
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

		cacheHeaders(w, options.cacheTime)
		packages.ServePackageResource(logger, w, r, pkgs[0], params.fileName)
	}, nil
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
