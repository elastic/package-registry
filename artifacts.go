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

const artifactsRouterPath = "/epr/{packageName}/{packageName:[a-z0-9_]+}-{packageVersion}.zip"

var errArtifactNotFound = errors.New("artifact not found")

func artifactsHandler(logger *zap.Logger, options handlerOptions) (func(w http.ResponseWriter, r *http.Request), error) {
	options.proxyMode = proxymode.NoProxy(logger)
	return artifactsHandlerWithProxyMode(logger, options)
}

func artifactsHandlerWithProxyMode(logger *zap.Logger, options handlerOptions) (func(w http.ResponseWriter, r *http.Request), error) {
	if options.proxyMode == nil {
		logger.Warn("artifactsHandlerWithProxyMode called without proxy mode, defaulting to no proxy")
		options.proxyMode = proxymode.NoProxy(logger)
	}
	if options.cacheTime < 0 {
		return nil, errors.New("cache time must be non-negative for artifacts handler")
	}
	if options.indexer == nil {
		return nil, errors.New("indexer is required for artifacts handler")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		logger := logger.With(apmzap.TraceContext(r.Context())...)

		// Return error if any query parameter is present
		if !options.allowUnknownQueryParameters && len(r.URL.Query()) > 0 {
			badRequest(w, "not supported query parameters")
			return
		}

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
		pkgs, err := options.indexer.Get(r.Context(), &opts)
		if err != nil {
			logger.Error("getting package path failed",
				zap.String("package.name", packageName),
				zap.String("package.version", packageVersion),
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
			notFoundError(w, errArtifactNotFound)
			return
		}

		cacheHeaders(w, options.cacheTime)
		packages.ServePackage(logger, w, r, pkgs[0])
	}, nil
}
