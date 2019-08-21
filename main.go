// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	ucfgYAML "github.com/elastic/go-ucfg/yaml"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v2"
)

var (
	packagesPath string
	address      string
	version      = "0.0.1"
	configPath   = "config.yml"
)

func init() {
	flag.StringVar(&address, "address", "localhost:8080", "Address of the integrations-registry service.")
}

type Config struct {
	PackagesPath string `config:"packages.path"`
}

func main() {
	flag.Parse()
	log.Println("Integrations registry started.")
	defer log.Println("Integrations registry stopped.")

	config, err := getConfig()
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	packagesPath = config.PackagesPath

	// Unzip all packages
	err = setup(packagesPath)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	server := &http.Server{Addr: address, Handler: getRouter()}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			log.Printf("Error serving: %s", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx := context.TODO()
	if err := server.Shutdown(ctx); err != nil {
		log.Print(err)
	}
}

func setup(packagePath string) error {
	log.Println("Extracting packages")
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(wd)
	err = os.Chdir(packagePath)
	if err != nil {
		return err
	}
	files, err := ioutil.ReadDir(".")
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			if err := os.RemoveAll(f.Name()); err != nil {
				return err
			}
		}
	}

	ff, err := filepath.Glob("*.tar.gz")
	if err != nil {
		return err
	}

	for _, f := range ff {
		cmd := exec.Command("tar", "xvfz", f)
		cmd.Run()
	}

	return nil
}

func getConfig() (*Config, error) {
	cfg, err := ucfgYAML.NewConfigWithFile(configPath)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = cfg.Unpack(config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func getRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", infoHandler())
	router.HandleFunc("/search", searchHandler())
	router.HandleFunc("/package/{name}.tar.gz", targzDownloadHandler)
	router.HandleFunc("/package/{name}", packageHandler())
	router.HandleFunc("/img/{name}/{file}", imgHandler)

	return router
}

// getIntegrationPackages returns list of available integration packages
func getIntegrationPackages() ([]string, error) {

	files, err := ioutil.ReadDir(packagesPath)
	if err != nil {
		return nil, err
	}

	var packages []string
	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		packages = append(packages, f.Name())
	}

	return packages, nil
}

type Package struct {
	Name        string  `yaml:"name" json:"name"`
	Title       *string `yaml:"title" json:"title"`
	Version     string  `yaml:"version" json:"version"`
	Description string  `yaml:"description" json:"description"`
	Icon        string  `yaml:"icon" json:"icon"`
}

func (p *Package) getIcon() string {
	return "/img/" + p.Name + "-" + p.Version + "/icon.png"
}

type Manifest struct {
	Package     `yaml:",inline" json:",inline"`
	Requirement struct {
		Kibana struct {
			Min string `yaml:"version.min" json:"version.min"`
			Max string `yaml:"version.max" json:"version.max"`
		} `yaml:"kibana" json:"kibana"`
	} `yaml:"requirement" json:"requirement"`
	Screenshots []Screenshot `yaml:"screenshots" json:"screenshots,omitempty"`
}

type Screenshot struct {
	Src   string `yaml:"src" json:"src,omitempty"`
	Title string `yaml:"title" json:"title,omitempty"`
	Size  string `yaml:"size" json:"size,omitempty"`
	Type  string `yaml:"type" json:"type,omitempty"`
}

func readManifest(p string) (*Manifest, error) {

	manifest, err := ioutil.ReadFile(packagesPath + "/" + p + "/manifest.yml")
	if err != nil {
		return nil, err
	}

	var m = &Manifest{}
	err = yaml.Unmarshal(manifest, m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func readImage(p, file string) ([]byte, error) {
	// Make sure no relative paths are inserted
	if strings.Contains(file, "..") {
		return nil, fmt.Errorf("no relative paths allowed")
	}
	return ioutil.ReadFile(packagesPath + "/" + p + "/img/" + file)
}
