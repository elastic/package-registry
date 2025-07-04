// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	gstorage "cloud.google.com/go/storage"
	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/gorilla/mux"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"google.golang.org/api/option"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.elastic.co/apm/module/apmgorilla/v2"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	ucfgYAML "github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/package-registry/internal/database"
	internalStorage "github.com/elastic/package-registry/internal/storage"
	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/metrics"
	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/proxymode"
	"github.com/elastic/package-registry/storage"
)

const (
	serviceName         = "package-registry"
	version             = "1.29.1"
	defaultInstanceName = "localhost"
)

var (
	address         string
	httpProfAddress string
	metricsAddress  string

	logLevel *zapcore.Level
	logType  string

	tlsCertFile string
	tlsKeyFile  string

	tlsMinVersionValue tlsVersionValue

	dryRun     bool
	configPath string

	printVersionInfo bool

	featureSQLStorageIndexer bool
	featureEnableSearchCache bool

	featureStorageIndexer        bool
	storageIndexerBucketInternal string
	storageEndpoint              string
	storageIndexerWatchInterval  time.Duration

	featureProxyMode bool
	proxyTo          string

	defaultConfig = Config{
		CacheTimeIndex:               10 * time.Second,
		CacheTimeSearch:              10 * time.Minute,
		CacheTimeCategories:          10 * time.Minute,
		CacheTimeCatchAll:            10 * time.Minute,
		SQLIndexerDatabaseFolderPath: "/tmp/", // TODO: Another default directory?
		SearchCacheSize:              100,
	}
)

func init() {
	flag.BoolVar(&printVersionInfo, "version", false, "Print Elastic Package Registry version")
	flag.StringVar(&address, "address", "localhost:8080", "Address of the package-registry service.")
	flag.StringVar(&metricsAddress, "metrics-address", "", "Address to expose the Prometheus metrics (experimental). ")

	logLevel = zap.LevelFlag("log-level", zap.InfoLevel, "log level (default \"info\")")
	flag.StringVar(&logType, "log-type", util.DefaultLoggerType, "log type (ecs, dev)")
	flag.StringVar(&tlsCertFile, "tls-cert", "", "Path of the TLS certificate.")
	flag.StringVar(&tlsKeyFile, "tls-key", "", "Path of the TLS key.")
	flag.Var(&tlsMinVersionValue, "tls-min-version", "Minimum version TLS supported.")
	flag.StringVar(&configPath, "config", "config.yml", "Path to the configuration file.")
	flag.StringVar(&httpProfAddress, "httpprof", "", "Enable HTTP profiler listening on the given address.")
	// This flag is experimental and might be removed in the future or renamed
	flag.BoolVar(&dryRun, "dry-run", false, "Runs a dry-run of the registry without starting the web service (experimental).")
	flag.BoolVar(&packages.ValidationDisabled, "disable-package-validation", false, "Disable package content validation.")
	// The following storage related flags are technical preview and might be removed in the future or renamed
	flag.BoolVar(&featureStorageIndexer, "feature-storage-indexer", false, "Enable storage indexer to include packages from Package Storage v2 (technical preview).")
	flag.BoolVar(&featureSQLStorageIndexer, "feature-sql-storage-indexer", false, "Enable SQL storage indexer to include packages from Package Storage v2 (technical preview).")
	flag.BoolVar(&featureEnableSearchCache, "feature-enable-search-cache", false, "Enable cache for search requests. Just supported with the SQL storage indexer. (technical preview).")
	flag.StringVar(&storageIndexerBucketInternal, "storage-indexer-bucket-internal", "", "Path to the internal Package Storage bucket (with gs:// prefix).")
	flag.StringVar(&storageEndpoint, "storage-endpoint", "https://package-storage.elastic.co/", "Package Storage public endpoint.")
	flag.DurationVar(&storageIndexerWatchInterval, "storage-indexer-watch-interval", 1*time.Minute, "Address of the package-registry service.")

	// The following proxy-indexer related flags are technical preview and might be removed in the future or renamed
	flag.BoolVar(&featureProxyMode, "feature-proxy-mode", false, "Enable proxy mode to include packages from other endpoint (technical preview).")
	flag.StringVar(&proxyTo, "proxy-to", "https://epr.elastic.co/", "Proxy-to endpoint")
}

type Config struct {
	PackagePaths                 []string      `config:"package_paths"`
	CacheTimeIndex               time.Duration `config:"cache_time.index"`
	CacheTimeSearch              time.Duration `config:"cache_time.search"`
	CacheTimeCategories          time.Duration `config:"cache_time.categories"`
	CacheTimeCatchAll            time.Duration `config:"cache_time.catch_all"`
	SQLIndexerDatabaseFolderPath string        `config:"sql_indexer.database_folder_path"`
	SearchCacheSize              int           `config:"search.cache_size"`
}

func main() {
	err := parseFlags()
	if err != nil {
		log.Fatal(err)
	}

	if tlsMinVersionValue > 0 {
		if tlsCertFile == "" || tlsKeyFile == "" {
			log.Fatalf("-tls-min-version set but missing TLS cert and key files (-tls-cert and -tls-key)")
		}
	}

	if featureStorageIndexer && featureSQLStorageIndexer {
		log.Fatal("Both feature-storage-indexer and feature-sql-storage-indexer are enabled. Please choose one.")
	}

	if featureEnableSearchCache && !featureSQLStorageIndexer {
		log.Fatal("feature-enable-search-cache is enabled, but feature-sql-storage-indexer is not enabled. Search cache is just supported with SQL Storage indexer.")
	}

	if printVersionInfo {
		fmt.Printf("Elastic Package Registry version %v\n", version)
		os.Exit(0)
	}

	apmTracer := initAPMTracer()
	defer apmTracer.Close()

	logger, err := util.NewLogger(util.LoggerOptions{
		APMTracer: apmTracer,
		Level:     logLevel,
		Type:      logType,
	})
	if err != nil {
		log.Fatalf("Failed to initialize logging: %v", err)
	}
	defer logger.Sync()

	apmTracer.SetLogger(&util.LoggerAdapter{logger.With(zap.String("log.logger", "apm"))})

	ctx := context.Background()

	config := mustLoadConfig(logger)
	if dryRun {
		logger.Info("Running dry-run mode")
		indexer := initIndexer(ctx, logger, apmTracer, config, nil)
		defer indexer.Close(ctx)
		os.Exit(0)
	}

	logger.Info("Package registry started")
	defer logger.Info("Package registry stopped")

	initHttpProf(logger)

	if indexPath := os.Getenv("EPR_EMULATOR_INDEX_PATH"); indexPath != "" {
		if !featureStorageIndexer && !featureSQLStorageIndexer {
			logger.Fatal("EPR_EMULATOR_INDEX_PATH environment variable is set, but feature-storage-indexer or feature-sql-storage-indexer are not enabled. Please enable one of them to use the fake GCS server.")
		}
		if storageIndexerBucketInternal != "" && storageIndexerBucketInternal != storage.FakeIndexerOptions.PackageStorageBucketInternal {
			logger.Fatal("EPR_EMULATOR_INDEX_PATH environment variable is set, but storage-indexer-bucket-internal is already set to a different value. Please remove the flag or set it to the fake GCS server bucket " + storage.FakeIndexerOptions.PackageStorageBucketInternal)
		}
		// In this mode, the internal bucket is set to the fake GCS server bucket.
		storageIndexerBucketInternal = storage.FakeIndexerOptions.PackageStorageBucketInternal
		fakeServer, err := initFakeGCSServer(logger, indexPath)
		if err != nil {
			logger.Fatal("failed to initialize fake GCS server", zap.Error(err))
		}
		defer fakeServer.Stop()
	}

	var searchCache *expirable.LRU[string, []byte]
	if featureSQLStorageIndexer && featureEnableSearchCache {
		searchCache = expirable.NewLRU[string, []byte](config.SearchCacheSize, nil, config.CacheTimeSearch)
	}

	indexer := initIndexer(ctx, logger, apmTracer, config, searchCache)
	defer indexer.Close(ctx)

	server := initServer(logger, apmTracer, config, indexer, searchCache)

	go func() {
		err := runServer(server)
		if err != nil && err != http.ErrServerClosed {
			logger.Fatal("error occurred while serving", zap.Error(err))
		}
	}()

	initMetricsServer(logger)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("error on shutdown", zap.Error(err))
	}
}

func initDatabase(ctx context.Context, logger *zap.Logger, databaseFolderPath, dbFileName string) (database.Repository, error) {
	span, _ := apm.StartSpan(ctx, "initDatabase", fmt.Sprintf("backend.init.%s", dbFileName))
	defer span.End()

	dbPath := filepath.Join(databaseFolderPath, dbFileName)

	logger.Debug("Creating database", zap.String("path", dbPath))
	exists := true
	_, err := os.Stat(dbPath)
	if errors.Is(err, os.ErrNotExist) {
		exists = false
	} else if err != nil {
		return nil, err
	}
	if exists {
		err = os.Remove(dbPath)
		if err != nil {
			logger.Fatal("failed to delete previous database", zap.String("path", dbPath), zap.Error(err))
			return nil, fmt.Errorf("failed to delete previous database (path %q): %w", dbPath, err)
		}
	}

	options := database.FileSQLDBOptions{
		Path: dbPath,
	}

	if os.Getenv("EPR_SQL_DB_INSERT_BATCH_SIZE") != "" {
		maxInsertBatchSize, err := strconv.Atoi(os.Getenv("EPR_SQL_DB_INSERT_BATCH_SIZE"))
		if err != nil {
			logger.Fatal("failed to parse EPR_SQL_DB_INSERT_BATCH_SIZE environment variable", zap.Error(err))
			return nil, fmt.Errorf("failed to parse EPR_SQL_DB_INSERT_BATCH_SIZE environment variable: %w", err)
		}
		options.BatchSizeInserts = maxInsertBatchSize
	}

	packageRepository, err := database.NewFileSQLDB(options)
	if err != nil {
		return nil, fmt.Errorf("failed to open database (path %q): %w", dbPath, err)
	}
	logger.Debug("Database created successfully", zap.String("path", dbPath))

	return packageRepository, nil
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

func initFakeGCSServer(logger *zap.Logger, indexPath string) (*fakestorage.Server, error) {
	var fakeServer *fakestorage.Server
	var err error
	emulatorHost := os.Getenv("STORAGE_EMULATOR_HOST")
	if emulatorHost != "" {
		logger.Info("Create fake GCS server based on STORAGE_EMULATOR_HOST environment variable", zap.String("STORAGE_EMULATOR_HOST", emulatorHost))
		host, port, err := net.SplitHostPort(emulatorHost)
		if err != nil {
			return nil, fmt.Errorf("failed to split host and port from STORAGE_EMULATOR_HOST: %w", err)
		}
		portInt, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("failed to convert port to integer from STORAGE_EMULATOR_HOST: %w", err)
		}
		fakeServer, err = internalStorage.RunFakeServerOnHostPort(indexPath, host, uint16(portInt))
		if err != nil {
			return nil, fmt.Errorf("failed to prepare fake storage server: %w", err)
		}
	} else {
		logger.Info("Create fake GCS server on random port")
		// let the fake server choose a random port
		fakeServer, err = internalStorage.RunFakeServerOnHostPort(indexPath, "localhost", 0)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare fake storage server: %w", err)
		}
		os.Setenv("STORAGE_EMULATOR_HOST", fakeServer.URL())
	}
	logger.Info("Using fake storage server for indexer", zap.String("URL", fakeServer.URL()))

	return fakeServer, nil
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return defaultInstanceName
	}
	return hostname
}

func initMetricsServer(logger *zap.Logger) {
	if metricsAddress == "" {
		return
	}

	hostname := getHostname()

	metrics.ServiceInfo.With(prometheus.Labels{"version": version, "instance": hostname}).Set(1)

	logger.Info("Starting http metrics in " + metricsAddress)
	go func() {
		router := http.NewServeMux()
		router.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(metricsAddress, router)
		if err != nil {
			logger.Fatal("failed to start Prometheus metrics endpoint", zap.Error(err))
		}
	}()
}

func initIndexer(ctx context.Context, logger *zap.Logger, apmTracer *apm.Tracer, config *Config, cache *expirable.LRU[string, []byte]) Indexer {
	tx := apmTracer.StartTransaction("initIndexer", "backend.init")
	defer tx.End()

	ctx = apm.ContextWithTransaction(ctx, tx)
	packagesBasePaths := getPackagesBasePaths(config)

	var combined CombinedIndexer

	switch {
	case featureSQLStorageIndexer:
		logger.Warn("Technical preview: SQL storage indexer is an experimental feature and it may be unstable.")
		indexer, err := initSQLStorageIndexer(ctx, logger, apmTracer, config, cache)
		if err != nil {
			logger.Fatal("failed to initialize SQL storage indexer", zap.Error(err))
		}
		combined = append(combined, indexer)
	case featureStorageIndexer:
		indexer, err := initStorageIndexer(ctx, logger, apmTracer, config)
		if err != nil {
			logger.Fatal("failed to initialize storage indexer", zap.Error(err))
		}
		combined = append(combined, indexer)
	}

	combined = append(combined,
		packages.NewZipFileSystemIndexer(logger, packagesBasePaths...),
		packages.NewFileSystemIndexer(logger, packagesBasePaths...),
	)
	ensurePackagesAvailable(ctx, logger, combined)
	return combined
}

func initStorageIndexer(ctx context.Context, logger *zap.Logger, apmTracer *apm.Tracer, config *Config) (*storage.Indexer, error) {
	storageClient, err := newStorageClient(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}
	return storage.NewIndexer(logger, storageClient, storage.IndexerOptions{
		APMTracer:                    apmTracer,
		PackageStorageBucketInternal: storageIndexerBucketInternal,
		PackageStorageEndpoint:       storageEndpoint,
		WatchInterval:                storageIndexerWatchInterval,
	}), nil
}

func initSQLStorageIndexer(ctx context.Context, logger *zap.Logger, apmTracer *apm.Tracer, config *Config, cache *expirable.LRU[string, []byte]) (*internalStorage.SQLIndexer, error) {
	storageClient, err := newStorageClient(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	storageDatabase, err := initDatabase(ctx, logger, config.SQLIndexerDatabaseFolderPath, "storage_packages.db")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage database: %w", err)
	}
	storageSwapDatabase, err := initDatabase(ctx, logger, config.SQLIndexerDatabaseFolderPath, "storage_packages_swap.db")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage backup database: %w", err)
	}

	options := internalStorage.IndexerOptions{
		APMTracer:                    apmTracer,
		PackageStorageBucketInternal: storageIndexerBucketInternal,
		PackageStorageEndpoint:       storageEndpoint,
		WatchInterval:                storageIndexerWatchInterval,
		Database:                     storageDatabase,
		SwapDatabase:                 storageSwapDatabase,
		Cache:                        cache,
	}

	if os.Getenv("EPR_SQL_INDEXER_READ_PACKAGES_BATCH_SIZE") != "" {
		readPackagesBatchSize, err := strconv.Atoi(os.Getenv("EPR_SQL_INDEXER_READ_PACKAGES_BATCH_SIZE"))
		if err != nil {
			logger.Fatal("failed to parse EPR_SQL_INDEXER_READ_PACKAGES_BATCH_SIZE environment variable", zap.Error(err))
			return nil, fmt.Errorf("failed to parse EPR_SQL_INDEXER_READ_PACKAGES_BATCH_SIZE environment variable: %w", err)
		}
		options.ReadPackagesBatchsize = readPackagesBatchSize
	}

	return internalStorage.NewIndexer(logger, storageClient, options), nil
}

func newStorageClient(ctx context.Context, logger *zap.Logger) (*gstorage.Client, error) {
	opts := []option.ClientOption{}
	if os.Getenv("STORAGE_EMULATOR_HOST") != "" {
		// https://pkg.go.dev/cloud.google.com/go/storage#hdr-Creating_a_Client
		logger.Info("Using local development setup for storage indexer", zap.String("STORAGE_EMULATOR_HOST", os.Getenv("STORAGE_EMULATOR_HOST")))
		// Required to add this option when using STORAGE_EMULATOR_HOST
		// Related to https://github.com/fsouza/fake-gcs-server/issues/1202#issuecomment-1644877525
		opts = append(opts, gstorage.WithJSONReads())
	}
	return gstorage.NewClient(ctx, opts...)
}

func initServer(logger *zap.Logger, apmTracer *apm.Tracer, config *Config, indexer Indexer, cache *expirable.LRU[string, []byte]) *http.Server {
	router := mustLoadRouter(logger, config, indexer, cache)
	apmgorilla.Instrument(router, apmgorilla.WithTracer(apmTracer))

	var tlsConfig tls.Config
	if tlsMinVersionValue > 0 {
		tlsConfig.MinVersion = uint16(tlsMinVersionValue)
	}
	return &http.Server{Addr: address, Handler: router, TLSConfig: &tlsConfig}
}

func runServer(server *http.Server) error {
	if tlsCertFile != "" && tlsKeyFile != "" {
		return server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
	}
	return server.ListenAndServe()
}

func initAPMTracer() *apm.Tracer {
	apm.DefaultTracer().Close()
	if _, found := os.LookupEnv("ELASTIC_APM_SERVER_URL"); !found {
		// Don't report anything if the Server URL hasn't been configured.
		return apm.DefaultTracer()
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
		return nil, fmt.Errorf("reading config failed (path: %s): %w", configPath, err)
	}

	config := defaultConfig
	err = cfg.Unpack(&config)
	if err != nil {
		return nil, fmt.Errorf("unpacking config failed (path: %s): %w", configPath, err)
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
	logger.Info("Cache time for /: " + config.CacheTimeIndex.String())
	logger.Info("Cache time for /index.json: " + config.CacheTimeIndex.String())
	logger.Info("Cache time for /search: " + config.CacheTimeSearch.String())
	logger.Info("Cache time for /categories: " + config.CacheTimeCategories.String())
	logger.Info("Cache time for all others: " + config.CacheTimeCatchAll.String())
	logger.Info("Database path: " + config.SQLIndexerDatabaseFolderPath)
	logger.Info("LRU cache size (search requests): " + strconv.Itoa(config.SearchCacheSize))
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

	if len(packages) > 0 {
		logger.Info(fmt.Sprintf("%v local package manifests loaded.", len(packages)))
	} else if featureProxyMode {
		logger.Info("No local packages found, but the proxy mode can access remote ones.")
	} else {
		logger.Fatal("No local packages found.")
	}
	metrics.NumberIndexedPackages.Set(float64(len(packages)))
}

func mustLoadRouter(logger *zap.Logger, config *Config, indexer Indexer, cache *expirable.LRU[string, []byte]) *mux.Router {
	router, err := getRouter(logger, config, indexer, cache)
	if err != nil {
		logger.Fatal("failed go configure router", zap.Error(err))
	}
	return router
}

func getRouter(logger *zap.Logger, config *Config, indexer Indexer, cache *expirable.LRU[string, []byte]) (*mux.Router, error) {
	if featureProxyMode {
		logger.Info("Technical preview: Proxy mode is an experimental feature and it may be unstable.")
	}
	proxyMode, err := proxymode.NewProxyMode(logger, proxymode.ProxyOptions{
		Enabled: featureProxyMode,
		ProxyTo: proxyTo,
	})
	if err != nil {
		return nil, fmt.Errorf("can't create proxy mode: %w", err)
	}
	artifactsHandler := artifactsHandlerWithProxyMode(logger, indexer, proxyMode, config.CacheTimeCatchAll)
	signaturesHandler := signaturesHandlerWithProxyMode(logger, indexer, proxyMode, config.CacheTimeCatchAll)
	faviconHandleFunc, err := faviconHandler(config.CacheTimeCatchAll)
	if err != nil {
		return nil, err
	}
	indexHandlerFunc, err := indexHandler(config.CacheTimeIndex)
	if err != nil {
		return nil, err
	}

	categoriesHandler := categoriesHandlerWithProxyMode(logger, indexer, proxyMode, config.CacheTimeCategories)
	packageIndexHandler := packageIndexHandlerWithProxyMode(logger, indexer, proxyMode, config.CacheTimeCatchAll)
	searchHandler := searchHandlerWithProxyMode(logger, indexer, proxyMode, config.CacheTimeSearch, cache)
	staticHandler := staticHandlerWithProxyMode(logger, indexer, proxyMode, config.CacheTimeCatchAll)

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", indexHandlerFunc)
	router.HandleFunc("/index.json", indexHandlerFunc)
	router.HandleFunc("/search", searchHandler)
	router.HandleFunc("/categories", categoriesHandler)
	router.HandleFunc("/health", healthHandler)
	router.HandleFunc("/favicon.ico", faviconHandleFunc)
	router.HandleFunc(artifactsRouterPath, artifactsHandler)
	router.HandleFunc(signaturesRouterPath, signaturesHandler)
	router.HandleFunc(packageIndexRouterPath, packageIndexHandler)
	router.HandleFunc(staticRouterPath, staticHandler)
	router.Use(util.LoggingMiddleware(logger))
	router.Use(util.CORSMiddleware())
	if metricsAddress != "" {
		router.Use(metrics.MetricsMiddleware())
	}
	router.NotFoundHandler = notFoundHandler(fmt.Errorf("404 page not found"))
	return router, nil
}

// healthHandler is used for Docker/K8s deployments. It returns 200 if the service is live
// In addition ?ready=true can be used for a ready request. Currently both are identical.
func healthHandler(w http.ResponseWriter, r *http.Request) {}
