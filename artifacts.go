// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/elastic/package-registry/archiver"
)

const artifactsRouterPath = "/epr/{packageName}/{packageName:[a-z_]+}-{packageVersion}.tar.gz"

var errArtifactNotFound = errors.New("artifact not found")

func artifactsHandler(packagesBasePaths []string, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
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
			notFoundError(w, errArtifactNotFound)
			return
		}
		if err != nil {
			log.Printf("stat package path '%s' failed: %v", packagePath, err)

			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/gzip")
		cacheHeaders(w, cacheTime)

		properties := archiver.PackageProperties{
			Name:    packageName,
			Version: packageVersion,
			Path:    packagePath,
		}
		// TODO: find proper directory, make it a config option
		basePath := "/tmp/epr"
		path := basePath + "/" + properties.Name + "-" + properties.Version + ".tar.gz"

		// TODO: Add dev option to skipt caching part
		// The nice thing here is that it also works across restarts, meaning pre-building
		// is an option.
		_, err = os.Stat(path)
		// If file does not exists, it builds the tar.gz and throws it into a file so it can be served again later.
		if os.IsNotExist(err) {
			os.MkdirAll(filepath.Dir(path), 0755)
			f, err := os.Create(path)
			if err != nil {
				http.Error(w, "error creating cache file", http.StatusInternalServerError)
				return
			}

			err = archiver.ArchivePackage(f, properties)
			if err != nil {
				log.Printf("archiving package path '%s' failed: %v", packagePath, err)
				return
			}

			f.Sync()
		} else if err != nil {
			log.Printf("checking file path '%s' failed: %v", packagePath, err)
			return
		}

		// TODO: Good idea to load it all into memory?
		content, err := ioutil.ReadFile(path)
		w.Write(content)
	}
}
