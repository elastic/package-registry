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

const signaturesRouterPath = "/epr/{packageName}/{packageName:[a-z0-9_]+}-{packageVersion}.zip.sig"

var errSignatureFileNotFound = errors.New("signature file not found")

func signaturesHandler(logger *zap.Logger, indexer Indexer, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return signaturesHandlerWithProxyMode(logger, indexer, proxymode.NoProxy(logger), cacheTime)
}

func signaturesHandlerWithProxyMode(logger *zap.Logger, indexer Indexer, proxyMode *proxymode.ProxyMode, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := logger.With(apmzap.TraceContext(r.Context())...)

		// Return error if any query parameter is present
		if len(r.URL.Query()) > 0 {
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
		pkgs, err := indexer.Get(r.Context(), &opts)
		if err != nil {
			logger.Error("getting package path failed",
				zap.String("package.name", packageName),
				zap.String("package.version", packageVersion),
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
			notFoundError(w, errSignatureFileNotFound)
			return
		}

		cacheHeaders(w, cacheTime)
		packages.ServePackageSignature(logger, w, r, pkgs[0])
	}
}
