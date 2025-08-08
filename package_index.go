// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"errors"
	"net/http"

	"go.elastic.co/apm/module/apmzap/v2"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"

	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/proxymode"
)

const (
	packageIndexRouterPath = "/package/{packageName:[a-z0-9_]+}/{packageVersion}/"
)

var errPackageRevisionNotFound = errors.New("package revision not found")

func packageIndexHandler(logger *zap.Logger, options handlerOptions) (func(w http.ResponseWriter, r *http.Request), error) {
	if options.proxyMode == nil {
		logger.Warn("packageIndexHandlerWithProxyMode called without proxy mode, defaulting to no proxy")
		options.proxyMode = proxymode.NoProxy(logger)
	}
	if options.cacheTime < 0 {
		return nil, errors.New("cache time must be non-negative for package index handler")
	}
	if options.indexer == nil {
		return nil, errors.New("indexer is required for package index handler")
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
		// Just this endpoint needs the full data, so we set it here.
		opts.FullData = true

		pkgs, err := options.indexer.Get(r.Context(), &opts)
		if err != nil {
			logger.Error("getting package path failed", zap.Error(err))
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

		data, err := getPackageOutput(r.Context(), pkgs[0])
		if err != nil {
			logger.Error("marshaling package index failed",
				zap.String("package.path", pkgs[0].BasePath),
				zap.Error(err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		serveJSONResponse(r.Context(), w, options.cacheTime, data)
	}, nil
}

func getPackageOutput(ctx context.Context, pkg *packages.Package) ([]byte, error) {
	span, _ := apm.StartSpan(ctx, "Get Package Output", "app")
	defer span.End()

	return util.MarshalJSONPretty(pkg)
}
