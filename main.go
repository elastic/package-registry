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
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmgorilla"

	ucfgYAML "github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/package-registry/packages"
)

const (
	serviceName = "package-registry"
	version     = "1.4.2"
)

var (
	address         string
	httpProfAddress string

	tlsCertFile string
	tlsKeyFile  string

	dryRun     bool
	configPath string

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
	log.Println("Package registry started.")
	defer log.Println("Package registry stopped.")

	initHttpProf()

	server := initServer()
	go func() {
		err := runServer(server)
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

func initHttpProf() {
	if httpProfAddress == "" {
		return
	}

	log.Printf("Starting http pprof in %s", httpProfAddress)
	go func() {
		err := http.ListenAndServe(httpProfAddress, nil)
		if err != nil {
			log.Fatalf("failed to start HTTP profiler: %v", err)
		}
	}()
}

func initServer() *http.Server {
	apmTracer := initAPMTracer()
	tx := apmTracer.StartTransaction("initServer", "backend.init")
	defer tx.End()

	ctx := apm.ContextWithTransaction(context.TODO(), tx)

	config := mustLoadConfig()
	packagesBasePaths := getPackagesBasePaths(config)
	indexer := NewCombinedIndexer(
		packages.NewFileSystemIndexer(packagesBasePaths...),
		packages.NewZipFileSystemIndexer(packagesBasePaths...),
	)
	ensurePackagesAvailable(ctx, indexer)

	// If -dry-run=true is set, service stops here after validation
	if dryRun {
		os.Exit(0)
	}

	router := mustLoadRouter(config, indexer)
	apmgorilla.Instrument(router, apmgorilla.WithTracer(apmTracer))

	return &http.Server{Addr: address, Handler: router}
}

func runServer(server *http.Server) error {
	if tlsCertFile != "" && tlsKeyFile != "" {
		return server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
	}
	return server.ListenAndServe()
}

func initAPMTracer() *apm.Tracer {
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
		log.Fatalf("Failed to initialize APM agent: %v", err)
	}
	return tracer
}

func mustLoadConfig() *Config {
	config, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}
	printConfig(config)
	return config
}

func getConfig() (*Config, error) {
	cfg, err := ucfgYAML.NewConfigWithFile(configPath)
	if os.IsNotExist(err) {
		log.Printf(`Using default configuration options as "%s" is not available.`, configPath)
		return &defaultConfig, nil
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

func printConfig(config *Config) {
	log.Printf("Packages paths: %s\n", strings.Join(config.PackagePaths, ", "))
	log.Println("Cache time for /search: ", config.CacheTimeSearch)
	log.Println("Cache time for /categories: ", config.CacheTimeCategories)
	log.Println("Cache time for all others: ", config.CacheTimeCatchAll)
}

func ensurePackagesAvailable(ctx context.Context, indexer Indexer) {
	err := indexer.Init(ctx)
	if err != nil {
		log.Fatal(err)
	}

	packages, err := indexer.Get(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	if len(packages) == 0 {
		log.Fatal("No packages available")
	}

	log.Printf("%v package manifests loaded.\n", len(packages))
}

func mustLoadRouter(config *Config, indexer Indexer) *mux.Router {
	router, err := getRouter(config, indexer)
	if err != nil {
		log.Fatal(err)
	}
	return router
}

func getRouter(config *Config, indexer Indexer) (*mux.Router, error) {
	artifactsHandler := artifactsHandler(indexer, config.CacheTimeCatchAll)
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
	router.HandleFunc(packageIndexRouterPath, packageIndexHandler)
	router.HandleFunc(staticRouterPath, staticHandler)
	router.Use(loggingMiddleware)
	router.NotFoundHandler = http.Handler(notFoundHandler(fmt.Errorf("404 page not found")))
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
