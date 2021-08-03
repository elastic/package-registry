// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"go.elastic.co/apm"

	"github.com/elastic/package-registry/archiver"
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

		p, err := getPackageFromIndex(r.Context(), indexer, packageName, packageVersion)
		if err == errResourceNotFound {
			notFoundError(w, errArtifactNotFound)
			return
		}
		if err != nil {
			log.Printf("getting package path failed: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		packagePath := p.BasePath
		f, err := os.Stat(packagePath)
		if err != nil {
			log.Printf("stat package path '%s' failed: %v", packagePath, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/gzip")
		cacheHeaders(w, cacheTime)

		if f.IsDir() {
			err = archiver.ArchivePackage(w, archiver.PackageProperties{
				Name:    packageName,
				Version: packageVersion,
				Path:    packagePath,
			})
			if err != nil {
				log.Printf("archiving package path '%s' failed: %v", packagePath, err)
				return
			}
		} else {
			http.ServeFile(w, r, packagePath)
		}
	}
}

func getPackageFromIndex(ctx context.Context, indexer Indexer, name, version string) (*util.Package, error) {
	span, ctx := apm.StartSpan(ctx, "GetPackagePathFromIndex", "app")
	defer span.End()

	packages, err := indexer.GetPackages(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range packages {
		if p.Name == name && p.Version == version {
			return &p, nil
		}
	}

	return nil, errResourceNotFound
}
