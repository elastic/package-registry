// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"log"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/util"
)

const (
	packageIndexRouterPath = "/package/{packageName:[a-z0-9_]+}/{packageVersion}/"
)

var errPackageRevisionNotFound = errors.New("package revision not found")

func packageIndexHandler(indexer Indexer, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
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
		packages, err := indexer.Get(r.Context(), &opts)
		if err != nil {
			log.Printf("getting package path failed: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if len(packages) == 0 {
			notFoundError(w, errPackageRevisionNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		cacheHeaders(w, cacheTime)

		err = util.WriteJSONPretty(w, packages[0])
		if err != nil {
			log.Printf("marshaling package index failed (path '%s'): %v", packages[0].BasePath, err)

			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}
}
