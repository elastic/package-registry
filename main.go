// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"io"
	"log"
	"net/http"
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

	router := getRouter()

	log.Fatal(http.ListenAndServe(address, router))
}

func getRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", infoHandler())
	router.HandleFunc("/list", listHandler())
	router.HandleFunc("/package/{name}", packageHandler())
	router.HandleFunc("/package/{name}/get", downloadHandler)
	router.HandleFunc("/img/{name}/{file}", imgHandler)

	return router
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

type Package struct {
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description" json:"description"`
	Icon        string `yaml:"icon" json:"icon"`
}

func (p *Package) getIcon() string {
	return "/img/" + p.Name + "-" + p.Version + "/icon.png"
}

type Manifest struct {
	Package     `yaml:",inline" json:",inline"`
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
