// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/blang/semver"
	"github.com/gorilla/mux"

	"github.com/elastic/package-registry/util"
)

const (
	packageIndexRouterPath1 = "/package/{packageName:[a-z_]+}/{packageVersion}/index.json"
	packageIndexRouterPath2 = "/package/{packageName:[a-z_]+}/{packageVersion}/"
)

var errPackageRevisionNotFound = errors.New("package revision not found")

func packageIndexHandler(packagesBasePath string, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
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

		_, err := semver.Parse(packageVersion)
		if err != nil {
			badRequest(w, "invalid package version")
			return
		}

		packagePath := filepath.Join(packagesBasePath, packageName, packageVersion)
		_, err = os.Stat(packagePath)
		if os.IsNotExist(err) {
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

		aPackage, err := util.NewPackage(packagePath)
		if err != nil {
			log.Printf("building package from path '%s' failed: %v", packagePath, err)

			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		body, err := json.MarshalIndent(aPackage, "", "  ")
		if err != nil {
			log.Printf("marshaling package index failed (path '%s'): %v", packagePath, err)

			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Write(body)
	}
}
