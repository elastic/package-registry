// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	ucfgYAML "github.com/elastic/go-ucfg/yaml"

	"github.com/gorilla/mux"
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

	router.HandleFunc("/search", searchHandler())
	router.PathPrefix("/").HandlerFunc(catchAll())

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
