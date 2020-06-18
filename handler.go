// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

var errPackageNotFound = errors.New("package not found")

func notFoundError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusNotFound)
}

func badRequest(w http.ResponseWriter, errorMessage string) {
	http.Error(w, errorMessage, http.StatusBadRequest)
}

func catchAll(public http.FileSystem, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	fileServer := http.FileServer(public)
	return func(w http.ResponseWriter, r *http.Request) {
		path, err := determineResourcePath(r, public)
		if err != nil {
			notFoundError(w, err)
			return
		}

		cacheHeaders(w, cacheTime)

		r.URL.Path = path
		fileServer.ServeHTTP(w, r)
	}
}

func determineResourcePath(r *http.Request, public http.FileSystem) (string, error) {
	path := r.URL.Path

	// Handles if it's a directory or last char is a / (also a directory)
	// It then opens index.json by default (if it exists)
	if len(path) == 0 || path == "/" {
		path = "index.json"
	} else if path[len(path)-1:] == "/" {
		path = filepath.Join(path, "index.json")
	} else {
		f, err := public.Open(path)
		if err != nil { // catch all errors, including "forbidden access"
			return "", errors.New("404 Page Not Found Error")
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			return "", errors.New("404 Page Not Found Error")
		}

		if stat.IsDir() {
			path = path + "/index.json"
			dirIndexFile, err := public.Open(path)
			if err != nil { // catch all errors, including "forbidden access"
				return "", errors.New("404 Page Not Found Error")
			}
			defer dirIndexFile.Close()
		}
	}
	return path, nil
}

func cacheHeaders(w http.ResponseWriter, cacheTime time.Duration) {
	maxAge := fmt.Sprintf("max-age=%.0f", cacheTime.Seconds())
	w.Header().Add("Cache-Control", maxAge)
	w.Header().Add("Cache-Control", "public")
}

func jsonHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func getPackagePath(packagesBasePaths []string, packageName, packageVersion string) (string, error) {
	for _, packagesBasePath := range packagesBasePaths {
		packagePath := filepath.Join(packagesBasePath, packageName, packageVersion)
		_, err := os.Stat(packagePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", errors.Wrapf(err, "stat file failed (path: %s)", packagePath)
		}
		return packagePath, nil
	}
	return "", errPackageNotFound
}
