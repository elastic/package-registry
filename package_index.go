// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"

	"github.com/elastic/package-registry/util"
)

const (
	packageIndexRouterPath = "/package/{packageName:[a-z_]+}/{packageVersion}/"
)

var errPackageRevisionNotFound = errors.New("package revision not found")

func packageIndexHandler(packagesBasePaths []string, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
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

		packagePath, err := getPackagePath(packagesBasePaths, packageName, packageVersion)
		if err == errResourceNotFound {
			notFoundError(w, errPackageRevisionNotFound)
			return
		}
		if err != nil {
			log.Printf("stat package path '%s' failed: %v", packagePath, err)

			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		cacheHeaders(w, cacheTime)

		p, err := util.NewPackage(packagePath)
		if err != nil {
			log.Printf("loading package from path '%s' failed: %v", packagePath, err)

			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		body, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			log.Printf("marshaling package index failed (path '%s'): %v", packagePath, err)

			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Write(body)
	}
}
