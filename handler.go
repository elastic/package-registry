// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/elastic/integrations-registry/p"

	"github.com/blang/semver"
)

func searchHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		integrations, err := getIntegrationPackages()
		if err != nil {
			notFound(w, err)
			return
		}

		integrationsList := map[string]*p.Manifest{}

		query := r.URL.Query()

		var kibanaVersion *semver.Version

		if len(query) > 0 {
			if v := query.Get("kibana"); v != "" {
				kibanaVersion, err = semver.New(v)
				if err != nil {
					// TODO: Add error that invalid version
					notFound(w, err)
					return
				}
			}
		}

		// Checks that only the most recent version of an integration is added to the list
		for _, i := range integrations {
			m, err := p.ReadManifest(packagesPath, i)
			if err != nil {
				notFound(w, err)
				return
			}

			if kibanaVersion != nil {
				if m.Requirement.Kibana.Max != "" {
					maxKibana, err := semver.Parse(m.Requirement.Kibana.Max)
					if err != nil {
						notFound(w, err)
						return
					}
					if kibanaVersion.GT(maxKibana) {
						continue
					}
				}

				if m.Requirement.Kibana.Min != "" {
					minKibana, err := semver.Parse(m.Requirement.Kibana.Min)
					if err != nil {
						notFound(w, err)
						return
					}
					if kibanaVersion.LT(minKibana) {
						continue
					}
				}
			}

			// Check if the version exists and if it should be added or not.
			if i, ok := integrationsList[m.Name]; ok {
				newVersion, _ := semver.Make(m.Version)
				oldVersion, _ := semver.Make(i.Version)

				// Skip addition of integration if only lower or equal
				if newVersion.LTE(oldVersion) {
					continue
				}
			}
			integrationsList[m.Name] = m

		}

		var output []map[string]interface{}

		for _, m := range integrationsList {
			data := map[string]interface{}{
				"name":        m.Name,
				"description": m.Description,
				"version":     m.Version,
				"download":    "/package/" + m.Name + "-" + m.Version + ".tar.gz",
			}
			if m.Title != nil {
				data["title"] = *m.Title
			}
			if m.Icons != nil {
				data["icons"] = m.Icons
			}
			output = append(output, data)
		}

		j, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			notFound(w, err)
			return
		}
		jsonHeader(w)
		fmt.Fprint(w, string(j))
	}
}

func notFound(w http.ResponseWriter, err error) {
	errString := ""
	if err != nil {
		errString = err.Error()
	}
	http.Error(w, errString, http.StatusNotFound)
}

func catchAll(publicPath string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

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
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".jpg":
	case ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	}
}
