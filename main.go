// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmgorilla"

	ucfgYAML "github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/storage"
	"github.com/elastic/package-registry/util"
)

const (
	serviceName = "package-registry"
	version     = "1.8.1"
)

var (
	address         string
	httpProfAddress string

	tlsCertFile string
	tlsKeyFile  string

	dryRun     bool
	configPath string

	featureStorageIndexer bool

	defaultConfig = Config{
		CacheTimeIndex:      10 * time.Second,
		CacheTimeSearch:     10 * time.Minute,
		CacheTimeCategories: 10 * time.Minute,
		CacheTimeCatchAll:   10 * time.Minute,
	}
)

func init() {
	flag.StringVar(&address, "address", "localhost:8080", "Address of the package-registry service.")
	flag.StringVar(&tlsCertFile, "tls-cert", "", "Path of the TLS certificate.")
	flag.StringVar(&tlsKeyFile, "tls-key", "", "Path of the TLS key.")
	flag.StringVar(&configPath, "config", "config.yml", "Path to the configuration file.")
	flag.StringVar(&httpProfAddress, "httpprof", "", "Enable HTTP profiler listening on the given address.")
	// This flag is experimental and might be removed in the future or renamed
	flag.BoolVar(&dryRun, "dry-run", false, "Runs a dry-run of the registry without starting the web service (experimental).")
	flag.BoolVar(&packages.ValidationDisabled, "disable-package-validation", false, "Disable package content validation.")
	// This flag is experimental and might be removed in the future or renamed
	flag.BoolVar(&featureStorageIndexer, "feature-storage-indexer", false, "Enable storage indexer to include packages from Package Storage v2 (experimental).")
}

type Config struct {
	PackagePaths        []string      `config:"package_paths"`
	CacheTimeIndex      time.Duration `config:"cache_time.index"`
	CacheTimeSearch     time.Duration `config:"cache_time.search"`
	CacheTimeCategories time.Duration `config:"cache_time.categories"`
	CacheTimeCatchAll   time.Duration `config:"cache_time.catch_all"`
}

func main() {
	parseFlags()

	logger := util.Logger()
	defer logger.Sync()

	logger.Info("Package registry started")
	defer logger.Info("Package registry stopped")

	initHttpProf(logger)

	server := initServer(logger)
	go func() {
		err := runServer(server)
		if err != nil && err != http.ErrServerClosed {
			logger.Fatal("error occurred while serving", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx := context.Background()
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("error on shutdown", zap.Error(err))
	}
}

func initHttpProf(logger *zap.Logger) {
	if httpProfAddress == "" {
		return
	}

	logger.Info("Starting http pprof in " + httpProfAddress)
	go func() {
		err := http.ListenAndServe(httpProfAddress, nil)
		if err != nil {
			logger.Fatal("failed to start HTTP profiler", zap.Error(err))
		}
	}()
}

func initServer(logger *zap.Logger) *http.Server {
	apmTracer := initAPMTracer(logger)
	tx := apmTracer.StartTransaction("initServer", "backend.init")
	defer tx.End()

	ctx := apm.ContextWithTransaction(context.TODO(), tx)

	config := mustLoadConfig(logger)
	packagesBasePaths := getPackagesBasePaths(config)

	var indexers []Indexer
	indexers = append(indexers, packages.NewFileSystemIndexer(packagesBasePaths...))
	indexers = append(indexers, packages.NewZipFileSystemIndexer(packagesBasePaths...))
	if featureStorageIndexer {
		indexers = append(indexers, storage.NewIndexer())
	}
	combinedIndexer := NewCombinedIndexer(indexers...)
	ensurePackagesAvailable(ctx, logger, combinedIndexer)

	// If -dry-run=true is set, service stops here after validation
	if dryRun {
		os.Exit(0)
	}

	router := mustLoadRouter(logger, config, combinedIndexer)
	apmgorilla.Instrument(router, apmgorilla.WithTracer(apmTracer))

	return &http.Server{Addr: address, Handler: router}
}

func runServer(server *http.Server) error {
	if tlsCertFile != "" && tlsKeyFile != "" {
		return server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
	}
	return server.ListenAndServe()
}

func initAPMTracer(logger *zap.Logger) *apm.Tracer {
	apm.DefaultTracer.Close()
	if _, found := os.LookupEnv("ELASTIC_APM_SERVER_URL"); !found {
		// Don't report anything if the Server URL hasn't been configured.
		return apm.DefaultTracer
	}

	tracer, err := apm.NewTracerOptions(apm.TracerOptions{
		ServiceName:    serviceName,
		ServiceVersion: version,
	})
	if err != nil {
		logger.Fatal("Failed to initialize APM agent", zap.Error(err))
	}
	return tracer
}

func mustLoadConfig(logger *zap.Logger) *Config {
	config, err := getConfig(logger)
	if err != nil {
		logger.Fatal("getting config", zap.Error(err))
	}
	printConfig(logger, config)
	return config
}

func getConfig(logger *zap.Logger) (*Config, error) {
	cfg, err := ucfgYAML.NewConfigWithFile(configPath)
	if os.IsNotExist(err) {
		logger.Fatal("Configuration file is not available: " + configPath)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "reading config failed (path: %s)", configPath)
	}

	config := defaultConfig
	err = cfg.Unpack(&config)
	if err != nil {
		return nil, errors.Wrapf(err, "unpacking config failed (path: %s)", configPath)
	}
	return &config, nil
}

func getPackagesBasePaths(config *Config) []string {
	var paths []string
	paths = append(paths, config.PackagePaths...)
	return paths
}

func printConfig(logger *zap.Logger, config *Config) {
	logger.Info("Packages paths: " + strings.Join(config.PackagePaths, ", "))
	logger.Info("Cache time for /search: " + config.CacheTimeSearch.String())
	logger.Info("Cache time for /categories: " + config.CacheTimeCategories.String())
	logger.Info("Cache time for all others: " + config.CacheTimeCatchAll.String())
}

func ensurePackagesAvailable(ctx context.Context, logger *zap.Logger, indexer Indexer) {
	err := indexer.Init(ctx)
	if err != nil {
		logger.Fatal("Init failed", zap.Error(err))
	}

	packages, err := indexer.Get(ctx, nil)
	if err != nil {
		logger.Fatal("Cannot get packages from indexer", zap.Error(err))
	}

	if len(packages) == 0 {
		logger.Fatal("No packages available")
	}

	logger.Info(fmt.Sprintf("%v package manifests loaded", len(packages)))
}

func mustLoadRouter(logger *zap.Logger, config *Config, indexer Indexer) *mux.Router {
	router, err := getRouter(logger, config, indexer)
	if err != nil {
		logger.Fatal("failed go configure router", zap.Error(err))
	}
	return router
}

func getRouter(logger *zap.Logger, config *Config, indexer Indexer) (*mux.Router, error) {
	artifactsHandler := artifactsHandler(indexer, config.CacheTimeCatchAll)
	signaturesHandler := signaturesHandler(indexer, config.CacheTimeCatchAll)
	faviconHandleFunc, err := faviconHandler(config.CacheTimeCatchAll)
	if err != nil {
		return nil, err
	}
	indexHandlerFunc, err := indexHandler(config.CacheTimeIndex)
	if err != nil {
		return nil, err
	}

	packageIndexHandler := packageIndexHandler(indexer, config.CacheTimeCatchAll)
	staticHandler := staticHandler(indexer, config.CacheTimeCatchAll)

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", indexHandlerFunc)
	router.HandleFunc("/index.json", indexHandlerFunc)
	router.HandleFunc("/search", searchHandler(indexer, config.CacheTimeSearch))
	router.HandleFunc("/categories", categoriesHandler(indexer, config.CacheTimeCategories))
	router.HandleFunc("/health", healthHandler)
	router.HandleFunc("/favicon.ico", faviconHandleFunc)
	router.HandleFunc(artifactsRouterPath, artifactsHandler)
	router.HandleFunc(signaturesRouterPath, signaturesHandler)
	router.HandleFunc(packageIndexRouterPath, packageIndexHandler)
	router.HandleFunc(staticRouterPath, staticHandler)
	router.Use(util.LoggingMiddleware(logger))
	router.NotFoundHandler = http.Handler(notFoundHandler(fmt.Errorf("404 page not found")))
	return router, nil
}

// healthHandler is used for Docker/K8s deployments. It returns 200 if the service is live
// In addition ?ready=true can be used for a ready request. Currently both are identical.
func healthHandler(w http.ResponseWriter, r *http.Request) {}
