// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/elastic/package-registry/util"

	ucfgYAML "github.com/elastic/go-ucfg/yaml"

	"github.com/gorilla/mux"
)

const (
	packageDir = "package"
)

var (
	packagesBasePath string
	address          string
	configPath       = "config.yml"

	defaultConfig = Config{
		PublicDir:           "config.yml",
		CacheTimeSearch:     10 * time.Minute,
		CacheTimeCategories: 10 * time.Minute,
		CacheTimeCatchAll:   10 * time.Minute,
	}
)

func init() {
	flag.StringVar(&address, "address", "localhost:8080", "Address of the package-registry service.")
}

type Config struct {
	PublicDir           string        `config:"public_dir"`
	CacheTimeSearch     time.Duration `config:"cache_time.search"`
	CacheTimeCategories time.Duration `config:"cache_time.categories"`
	CacheTimeCatchAll   time.Duration `config:"cache_time.catch_all"`
}

func main() {
	flag.Parse()
	log.Println("Package registry started.")
	defer log.Println("Package registry stopped.")

	config, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Cache time for /search: ", config.CacheTimeSearch)
	log.Println("Cache time for /categories: ", config.CacheTimeCategories)
	log.Println("Cache time for all others: ", config.CacheTimeCatchAll)

	packagesBasePath := filepath.Join(config.PublicDir, packageDir)
	packages, err := util.GetPackages(packagesBasePath)
	if err != nil {
		log.Fatal(err)
	}

	if len(packages) == 0 {
		log.Fatal("No packages available")
	}

	log.Printf("%v package manifests loaded into memory.\n", len(packages))

	router, err := getRouter(*config, packagesBasePath)
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{Addr: address, Handler: router}

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error occurred while serving: %s", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx := context.TODO()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}

func getConfig() (*Config, error) {
	cfg, err := ucfgYAML.NewConfigWithFile(configPath)
	if err != nil {
		return nil, err
	}

	config := defaultConfig
	err = cfg.Unpack(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func getRouter(config Config, packagesBasePath string) (*mux.Router, error) {
	faviconHandleFunc, err := faviconHandler(config.CacheTimeCatchAll)
	if err != nil {
		return nil, err
	}
	indexHandlerFunc, err := indexHandler(config.CacheTimeCatchAll)
	if err != nil {
		return nil, err
	}

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", indexHandlerFunc)
	router.HandleFunc("/index.json", indexHandlerFunc)
	router.HandleFunc("/search", searchHandler(packagesBasePath, config.CacheTimeSearch))
	router.HandleFunc("/categories", categoriesHandler(packagesBasePath, config.CacheTimeCategories))
	router.HandleFunc("/health", healthHandler)
	router.HandleFunc("/favicon.ico", faviconHandleFunc)
	router.PathPrefix("/epr").HandlerFunc(catchAll(http.Dir(config.PublicDir), config.CacheTimeCatchAll))
	router.PathPrefix("/package").HandlerFunc(catchAll(http.Dir(config.PublicDir), config.CacheTimeCatchAll))
	router.Use(loggingMiddleware)
	return router, nil
}

// healthHandler is used for Docker/K8s deployments. It returns 200 if the service is live
// In addition ?ready=true can be used for a ready request. Currently both are identical.
func healthHandler(w http.ResponseWriter, r *http.Request) {}

// logging middle to log all requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		next.ServeHTTP(w, r)
	})
}

// logRequest converts a request object into a proper logging event
func logRequest(r *http.Request) {
	// Do not log requests to the health endpoint
	if r.RequestURI == "/health" {
		return
	}
	log.Println(fmt.Sprintf("source.ip: %s, url.original: %s", r.RemoteAddr, r.RequestURI))
}
