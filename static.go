// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"log"
	"net/http"
	"time"
)

const staticHandlerPrefix = "/package"

func staticHandler(packagesBasePaths []string, prefix string, cacheTime time.Duration) http.HandlerFunc {
	fileServers := map[string]http.Handler{}
	for _, packagesBasePath := range packagesBasePaths {
		fileServers[packagesBasePath] = catchAll(http.Dir(packagesBasePath), cacheTime)
	}
	return http.StripPrefix(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		basePath, err := getPackageBasePath(packagesBasePaths, r.URL.Path)
		if err == errResourceNotFound {
			notFoundError(w, err)
			return
		}
		if err != nil {
			log.Printf("stat package path '%s' failed: %v", r.URL.Path, err)

			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		fileServers[basePath].ServeHTTP(w, r)
	})).ServeHTTP
}
