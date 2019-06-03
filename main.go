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
	address      string
	version      = "0.0.1"
)

func init() {
	flag.StringVar(&packagesPath, "packages-path", "./packages", "Path to integration packages directory.")
	flag.StringVar(&address, "address", "localhost:8080", "Address of the integrations-registry service.")
}

func main() {
	flag.Parse()
	log.Println("Integrations registry started.")
	defer log.Println("Integrations registry stopped.")

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", infoHandler())
	router.HandleFunc("/list", listHandler())
	router.HandleFunc("/package/{name}", packageHandler())
	router.HandleFunc("/package/{name}/get", downloadHandler)
	router.HandleFunc("/img/{name}/{file}", imgHandler)

	log.Fatal(http.ListenAndServe(address, router))
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
				http.NotFound(w, r)
				return
			}
		} else {
			http.NotFound(w, r)
			return
		}
	}

	// Safety check for too short paths
	if len(file) < 3 {
		http.NotFound(w, r)
		return
	}

	suffix := file[len(file)-3:]

	// Only .png and .jpg are supported at the moment
	if suffix == "png" {
		w.Header().Set("Content-Type", "image/png")
	} else if suffix == "jpg" {
		w.Header().Set("Content-Type", "image/jpeg")
	} else {
		http.NotFound(w, r)
		return
	}

	fmt.Fprint(w, string(img))
}

func listHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		integrations, err := getIntegrationPackages()
		if err != nil {
			http.NotFound(w, r)
			return
		}

		var output []map[string]string
		for _, i := range integrations {
			m, err := readManifest(i)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			data := map[string]string{
				"name":        m.Name,
				"description": m.Description,
				"version":     m.Version,
				"icon":        "/img/" + m.Name + "-" + m.Version + "/icon.png",
			}
			output = append(output, data)
		}
		j, err := json.MarshalIndent(output, "", "  ")
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
	Description string `yaml:"description" json:"description"`
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

func readImage(p, file string) ([]byte, error) {

	r, err := zip.OpenReader(packagesPath + "/" + p + ".zip")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {

		// Skip all files not in the img directory
		if filepath.Base(filepath.Dir(f.Name)) != "img" {
			continue
		}

		if filepath.Base(f.Name) == file {
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

			return buf.Bytes(), nil
		}
	}

	// Means package exists but no image found
	return nil, nil
}
