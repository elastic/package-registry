// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/blang/semver"
	"github.com/gorilla/mux"
)

func zipDownloadHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	file := vars["name"]

	path := packagesPath + "/" + file + ".zip"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		notFound(w, err)
		return
	}

	d, err := ioutil.ReadFile(path)
	if err != nil {
		notFound(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Description", "File Transfer")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file+".zip\"")
	w.Header().Set("Content-Transfer-Encoding", "binary")

	fmt.Fprint(w, string(d))
}

func targzDownloadHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	file := vars["name"]

	path := packagesPath + "/" + file + ".tar.gz"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		notFound(w, err)
		return
	}

	d, err := ioutil.ReadFile(path)
	if err != nil {
		notFound(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Description", "File Transfer")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file+".tar.gz\"")
	w.Header().Set("Content-Transfer-Encoding", "binary")

	fmt.Fprint(w, string(d))
}

func infoHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonHeader(w)
		fmt.Fprintf(w, `{"version": "%s"}`, version)
	}
}

func packageHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		key := vars["name"]

		manifest, err := readManifest(key)
		if err != nil {
			notFound(w, fmt.Errorf("error reading manfiest: %s, %s", key, err))
			return
		}
		// It's not set by default, generate it
		manifest.Icon = manifest.getIcon()

		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			log.Fatal(data)
		}

		jsonHeader(w)
		fmt.Fprint(w, string(data))
	}
}

func imgHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	integration := vars["name"]
	file := vars["file"]

	img, err := readImage(integration, file)
	if err != nil {
		http.Error(w, "integration "+integration+" not found", 404)
		return
	}

	// Package exists but does not have an icon, so the default icon is shipped
	if img == nil {
		if file == "icon.png" {
			img, err = ioutil.ReadFile("./img/icon.png")
			if err != nil {
				notFound(w, err)
				return
			}
		} else {
			notFound(w, nil)
			return
		}
	}

	// Safety check for too short paths
	if len(file) < 3 {
		notFound(w, nil)
		return
	}

	suffix := file[len(file)-3:]

	// Only .png and .jpg are supported at the moment
	if suffix == "png" {
		w.Header().Set("Content-Type", "image/png")
	} else if suffix == "jpg" {
		w.Header().Set("Content-Type", "image/jpeg")
	} else {
		notFound(w, err)
		return
	}

	fmt.Fprint(w, string(img))
}

func listHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		integrations, err := getIntegrationPackages()
		if err != nil {
			notFound(w, err)
			return
		}

		integrationsList := map[string]*Manifest{}

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
			m, err := readManifest(i)
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

		var output []map[string]string

		for _, m := range integrationsList {
			data := map[string]string{
				"name":        m.Name,
				"description": m.Description,
				"version":     m.Version,
				"icon":        m.getIcon(),
				"download":    "/package/" + m.Name + "-" + m.Version + ".tar.gz",
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

func jsonHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}
