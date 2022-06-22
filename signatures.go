// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"net/http"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/util"
)

const signaturesRouterPath = "/epr/{packageName}/{packageName:[a-z0-9_]+}-{packageVersion}.zip.sig"

var errSignatureFileNotFound = errors.New("signature file not found")

func signaturesHandler(indexer Indexer, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	logger := util.Logger()
	return func(w http.ResponseWriter, r *http.Request) {
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
		packageList, err := indexer.Get(r.Context(), &opts)
		if err != nil {
			logger.Error("getting package path failed",
				zap.String("package.name", packageName),
				zap.String("package.version", packageVersion),
				zap.Error(err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if len(packageList) == 0 {
			notFoundError(w, errSignatureFileNotFound)
			return
		}

		cacheHeaders(w, cacheTime)
		packages.ServePackageLocation(w, r, packageList[0], packageList[0].BasePath+".sig")
	}
}
