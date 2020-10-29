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

var errResourceNotFound = errors.New("resource not found")

func notFoundError(w http.ResponseWriter, err error) {
	noCacheHeaders(w)
	http.Error(w, err.Error(), http.StatusNotFound)
}

func badRequest(w http.ResponseWriter, errorMessage string) {
	noCacheHeaders(w)
	http.Error(w, errorMessage, http.StatusBadRequest)
}

func cacheHeaders(w http.ResponseWriter, cacheTime time.Duration) {
	maxAge := fmt.Sprintf("max-age=%.0f", cacheTime.Seconds())
	w.Header().Add("Cache-Control", maxAge)
	w.Header().Add("Cache-Control", "public")
}

func noCacheHeaders(w http.ResponseWriter) {
	w.Header().Add("Cache-Control", "max-age=0")
	w.Header().Add("Cache-Control", "private, no store")
}

func jsonHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func catchAll(public http.FileSystem, cacheTime time.Duration) http.Handler {
	fileServer := http.FileServer(public)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, err := determineResourcePath(r, public)
		if err != nil {
			notFoundError(w, err)
			return
		}

		cacheHeaders(w, cacheTime)

		r.URL.Path = path
		fileServer.ServeHTTP(w, r)
	})
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

func getPackagePath(packagesBasePaths []string, packageName, packageVersion string) (string, error) {
	basePath, err := getPackageBasePath(packagesBasePaths, filepath.Join(packageName, packageVersion))
	if err != nil {
		return "", err
	}
	return filepath.Join(basePath, packageName, packageVersion), nil
}

func getPackageBasePath(packagesBasePaths []string, resourcePath string) (string, error) {
	for _, basePath := range packagesBasePaths {
		packagePath := filepath.Join(basePath, resourcePath)
		_, err := os.Stat(packagePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", errors.Wrapf(err, "stat file failed (path: %s)", packagePath)
		}
		return basePath, nil
	}
	return "", errResourceNotFound
}
