// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

func notFound(w http.ResponseWriter, err error) {
	errString := ""
	if err != nil {
		errString = err.Error()
	}
	http.Error(w, errString, http.StatusNotFound)
}

func catchAll(publicPath, cacheTime string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cacheHeaders(w, cacheTime)

		path := r.RequestURI

		file, err := os.Stat(publicPath + path)
		if os.IsNotExist(err) {
			notFound(w, fmt.Errorf("404 Page Not Found Error"))
			return
		}

		// Handles if it's a directory or last char is a / (also a directory)
		// It then opens index.json by default (if it exists)
		if len(path) == 0 {
			path = "/index.json"
		} else if path[len(path)-1:] == "/" {
			path = path + "index.json"
		} else if file.IsDir() {
			path = path + "/index.json"
		}

		file, err = os.Stat(publicPath + path)
		if os.IsNotExist(err) {
			notFound(w, fmt.Errorf("404 Page Not Found Error"))
			return
		}

		data, err := ioutil.ReadFile(publicPath + path)
		if err != nil {
			notFound(w, fmt.Errorf("404 Page Not Found Error"))
			return
		}
		sendHeader(w, r)
		w.Write(data)
	}
}

func jsonHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func sendHeader(w http.ResponseWriter, r *http.Request) {
	extension := filepath.Ext(r.RequestURI)

	switch extension {
	// No extension is always json
	case "":
		w.Header().Set("Content-Type", "application/json")
	case ".asciidoc":
		w.Header().Set("Content-Type", "text/asciidoc; charset=UTF-8")
	case ".gz":
		w.Header().Set("Content-Type", "application/gzip")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	case ".jpg":
	case ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".md":
		w.Header().Set("Content-Type", "text/markdown; charset=UTF-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".yml":
		w.Header().Set("Content-Type", "text/yaml; charset=UTF-8")
	default:
		// Using json as the default header
		w.Header().Set("Content-Type", "application/json")
	}
}

func cacheHeaders(w http.ResponseWriter, cacheTime string) {
	w.Header().Add("Cache-Control", "max-age="+cacheTime)
	w.Header().Add("Cache-Control", "public")
}
