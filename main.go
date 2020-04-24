// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/elastic/package-registry/util"

	ucfgYAML "github.com/elastic/go-ucfg/yaml"

	"github.com/gorilla/mux"
	"go.elastic.co/ecszap"
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

	logger *zap.Logger
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

	encoderConfig := ecszap.NewDefaultEncoderConfig()
	core := ecszap.NewCore(encoderConfig, os.Stdout, zap.DebugLevel)
	logger = zap.New(core, zap.AddCaller())

	logger.Info("Package registry started.")
	defer logger.Info("Package registry stopped.")

	flag.Parse()
	config, err := getConfig()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("Cache time for /search: %s", config.CacheTimeSearch))
	logger.Info(fmt.Sprint("Cache time for /categories: %s", config.CacheTimeCategories))
	logger.Info(fmt.Sprint("Cache time for all others: %s", config.CacheTimeCatchAll))

	packagesBasePath := config.PublicDir + "/" + packageDir

	// Prefill the package cache
	packages, err := util.GetPackages(packagesBasePath)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	logger.Info(fmt.Sprintf("%v package manifests loaded into memory.", len(packages)))

	server := &http.Server{Addr: address, Handler: getRouter(*config, packagesBasePath)}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			logger.Error(fmt.Sprintf("Error serving: %s", err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx := context.TODO()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error(err.Error())
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

func getRouter(config Config, packagesBasePath string) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/search", searchHandler(packagesBasePath, config.CacheTimeSearch))
	router.HandleFunc("/categories", categoriesHandler(packagesBasePath, config.CacheTimeCategories))
	router.HandleFunc("/health", healthHandler)
	router.PathPrefix("/").HandlerFunc(catchAll(config.PublicDir, config.CacheTimeCatchAll))

	return router
}

// healthHandler is used for Docker/K8s deployments. It returns 200 if the service is live
// In addition ?ready=true can be used for a ready request. Currently both are identical.
func healthHandler(w http.ResponseWriter, r *http.Request) {}

// TODO: convert to a logging handler
func logRequest(r *http.Request) {
	// TOOD: log in go routine to not block
	logger.Info("some logging info",
		zap.String("source.ip", r.RemoteAddr),
		zap.String("url.original", r.RequestURI),
	)
}
