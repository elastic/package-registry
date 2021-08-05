// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"log"
	"net/http"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/elastic/package-registry/util"
)

const artifactsRouterPath = "/epr/{packageName}/{packageName:[a-z0-9_]+}-{packageVersion}.zip"

var errArtifactNotFound = errors.New("artifact not found")

func artifactsHandler(indexer Indexer, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
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

		opts := util.PackageNameVersionFilter(packageName, packageVersion)
		packages, err := indexer.GetPackages(r.Context(), &opts)
		if err != nil {
			log.Printf("getting package path failed: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if len(packages) == 0 {
			notFoundError(w, errArtifactNotFound)
			return
		}

		cacheHeaders(w, cacheTime)
		util.ServePackage(w, r, packages[0])
	}
}
