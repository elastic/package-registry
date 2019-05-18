// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v2"
)

var (
	packagesPath string
	version      = "0.0.1"
)

func init() {
	packagesPath = *flag.String("packages-path", "./packages", "Path to integration packages directory.")
}

func main() {

	log.Println("Integrations registry started.")
	defer log.Println("Integrations registry stopped.")

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", infoHandler())
	router.HandleFunc("/list", listHandler())
	router.HandleFunc("/package/{name}", packageHandler())
	router.HandleFunc("/package/{name}/get", downloadHandler)

	log.Fatal(http.ListenAndServe("localhost:8080", router))
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	file := vars["name"]

	path := packagesPath + "/" + file + ".zip"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	d, err := ioutil.ReadFile(path)
	if err != nil {
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Description", "File Transfer")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file+".zip\"")
	w.Header().Set("Content-Transfer-Encoding", "binary")

	fmt.Fprint(w, string(d))
}

func infoHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version": "%s"}`, version)
	}
}

func packageHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		key := vars["name"]

		manifest, err := readManifest(key)
		if err != nil {
			log.Printf("Manifest not found: %s, %s", key, manifest)
			http.NotFound(w, r)
			return
		}

		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			log.Fatal(data)
		}

		fmt.Fprint(w, string(data))
	}
}

func listHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		integrations, err := getIntegrationPackages()
		if err != nil {
			http.NotFound(w, r)
			return
		}

		j, err := json.Marshal(integrations)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, string(j))
	}
}

// getIntegrationPackages returns list of available integration packages
func getIntegrationPackages() ([]string, error) {
	files, err := filepath.Glob(packagesPath + "/*.zip")
	if err != nil {
		return nil, err
	}

	var integrations []string
	for _, f := range files {
		file := filepath.Base(f)
		integration := strings.TrimSuffix(file, filepath.Ext(file))
		integrations = append(integrations, integration)
	}

	return integrations, nil
}

type Manifest struct {
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Requirement struct {
		Kibana struct {
			Min string `yaml:"min" json:"min"`
			Max string `yaml:"max" json:"max"`
		} `yaml:"kibana" json:"kibana"`
	} `yaml:"requirement" json:"requirement"`
}

func readManifest(p string) (*Manifest, error) {

	r, err := zip.OpenReader(packagesPath + "/" + p + ".zip")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == "manifest.yml" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}

			var data []byte
			buf := bytes.NewBuffer(data)
			_, err = io.Copy(buf, rc)
			if err != nil {
				return nil, err
			}
			rc.Close()

			var m = &Manifest{}
			err = yaml.Unmarshal(buf.Bytes(), m)
			if err != nil {
				return nil, err
			}

			return m, nil
		}
	}

	return nil, nil
}
