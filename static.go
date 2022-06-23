// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/util"
)

const staticRouterPath = "/package/{packageName}/{packageVersion}/{name:.*}"

type staticParams struct {
	packageName    string
	packageVersion string
	fileName       string
}

func staticHandler(indexer Indexer, cacheTime time.Duration) http.HandlerFunc {
	logger := util.Logger()
	return func(w http.ResponseWriter, r *http.Request) {
		params, err := staticParamsFromRequest(r)
		if err != nil {
			badRequest(w, err.Error())
			return
		}

		opts := packages.NameVersionFilter(params.packageName, params.packageVersion)
		packageList, err := indexer.Get(r.Context(), &opts)
		if err != nil {
			logger.Error("getting package path failed",
				zap.String("package.name", params.packageName),
				zap.String("package.version", params.packageVersion),
				zap.Error(err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if len(packageList) == 0 {
			notFoundError(w, errPackageRevisionNotFound)
			return
		}

		cacheHeaders(w, cacheTime)
		packages.ServeLocalPackageResource(w, r, packageList[0], params.fileName)
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
