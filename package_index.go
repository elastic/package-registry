// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"errors"
	"net/http"
	"time"

	"go.elastic.co/apm/module/apmzap/v2"
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

func packageIndexHandler(logger *zap.Logger, indexer Indexer, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return packageIndexHandlerWithProxyMode(logger, indexer, proxymode.NoProxy(logger), cacheTime)
}

func packageIndexHandlerWithProxyMode(logger *zap.Logger, indexer Indexer, proxyMode *proxymode.ProxyMode, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := logger.With(apmzap.TraceContext(r.Context())...)

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
		pkgs, err := indexer.Get(r.Context(), &opts)
		if err != nil {
			logger.Error("getting package path failed", zap.Error(err))
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

		w.Header().Set("Content-Type", "application/json")
		cacheHeaders(w, cacheTime)

		err = util.WriteJSONPretty(w, pkgs[0])
		if err != nil {
			logger.Error("marshaling package index failed",
				zap.String("package.path", pkgs[0].BasePath),
				zap.Error(err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}
}
