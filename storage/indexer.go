// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"

	"cloud.google.com/go/storage"

	"github.com/prometheus/client_golang/prometheus"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/internal/database"
	"github.com/elastic/package-registry/metrics"
	"github.com/elastic/package-registry/packages"
)

const indexerGetDurationPrometheusLabel = "StorageIndexer"

type Indexer struct {
	options       IndexerOptions
	storageClient *storage.Client

	cursor string

	label string

	m sync.RWMutex

	resolver packages.RemoteResolver

	database     database.Repository
	swapDatabase database.Repository

	current *database.Repository
	backup  *database.Repository

	logger *zap.Logger
}

type IndexerOptions struct {
	APMTracer                    *apm.Tracer
	PackageStorageBucketInternal string
	PackageStorageEndpoint       string
	WatchInterval                time.Duration
	Database                     database.Repository
	SwapDatabase                 database.Repository
}

func NewIndexer(logger *zap.Logger, storageClient *storage.Client, options IndexerOptions) *Indexer {
	if options.APMTracer == nil {
		options.APMTracer = apm.DefaultTracer()
	}

	indexer := &Indexer{
		storageClient: storageClient,
		options:       options,
		logger:        logger,
		database:      options.Database,
		swapDatabase:  options.SwapDatabase,
		label:         fmt.Sprintf("storage-%s", options.PackageStorageEndpoint),
	}

	indexer.current = &indexer.database
	indexer.backup = &indexer.swapDatabase

	return indexer
}

func (i *Indexer) Init(ctx context.Context) error {
	i.logger.Debug("Initialize storage indexer")

	err := validateIndexerOptions(i.options)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	err = i.setupResolver()
	if err != nil {
		return fmt.Errorf("can't setup remote resolver: %w", err)
	}

	// Populate index file for the first time.
	start := time.Now()
	err = i.updateIndex(ctx)
	if err != nil {
		return fmt.Errorf("can't update index file: %w", err)
	}
	i.logger.Info("Elapsed time to init database", zap.Duration("duration", time.Since(start)))

	go i.watchIndices(apm.ContextWithTransaction(ctx, nil))
	return nil
}

func validateIndexerOptions(options IndexerOptions) error {
	if !strings.HasPrefix(options.PackageStorageBucketInternal, "gs://") {
		return errors.New("missing or invalid options.PackageStorageBucketInternal")
	}
	_, err := url.Parse(options.PackageStorageEndpoint)
	if err != nil {
		return fmt.Errorf("invalid options.PackageStorageEndpoint, URL expected: %w", err)
	}
	if options.WatchInterval < 0 {
		return errors.New("options.WatchInterval must be greater than or equal to 0")
	}
	return nil
}

func (i *Indexer) setupResolver() error {
	baseURL, err := url.Parse(i.options.PackageStorageEndpoint)
	if err != nil {
		return err
	}

	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				// Connect timeout.
				Timeout: 20 * time.Second,
			}).DialContext,
		},
	}

	i.resolver = storageResolver{
		client:               &httpClient,
		artifactsPackagesURL: *baseURL.ResolveReference(&url.URL{Path: artifactsPackagesStoragePath + "/"}),
		artifactsStaticURL:   *baseURL.ResolveReference(&url.URL{Path: artifactsStaticStoragePath + "/"}),
	}
	return nil
}

func (i *Indexer) watchIndices(ctx context.Context) {
	i.logger.Debug("Watch indices for changes")
	if i.options.WatchInterval == 0 {
		i.logger.Debug("No watcher configured, indices will not be updated (use only for testing purposes)")
		return
	}

	var err error
	t := time.NewTicker(i.options.WatchInterval)
	defer t.Stop()
	for {
		i.logger.Debug("watchIndices: start")

		func() {
			tx := i.options.APMTracer.StartTransaction("updateIndex", "backend.watcher")
			defer tx.End()

			err = i.updateIndex(apm.ContextWithTransaction(ctx, tx))
			if err != nil {
				i.logger.Error("can't update index file", zap.Error(err))
			}
		}()

		i.logger.Debug("watchIndices: finished")
		select {
		case <-ctx.Done():
			i.logger.Debug("watchIndices: quit")
			return
		case <-t.C:
		}
	}
}

func (i *Indexer) updateIndex(ctx context.Context) error {
	span, ctx := apm.StartSpan(ctx, "UpdateIndex", "app")
	defer span.End()

	i.logger.Debug("Update indices")
	start := time.Now()
	defer func() {
		metrics.StorageIndexerUpdateIndexDurationSeconds.Observe(time.Since(start).Seconds())
	}()

	bucketName, rootStoragePath, err := extractBucketNameFromURL(i.options.PackageStorageBucketInternal)
	if err != nil {
		metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
		return fmt.Errorf("can't extract bucket name from URL (url: %s): %w", i.options.PackageStorageBucketInternal, err)
	}

	storageCursor, err := loadCursor(ctx, i.logger, i.storageClient, bucketName, rootStoragePath)
	if err != nil {
		metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
		return fmt.Errorf("can't load latest cursor: %w", err)
	}

	if storageCursor.Current == i.cursor {
		i.logger.Info("cursor is up-to-date", zap.String("cursor.current", i.cursor))
		return nil
	}
	i.logger.Info("cursor will be updated", zap.String("cursor.current", i.cursor), zap.String("cursor.next", storageCursor.Current))

	anIndex, err := loadSearchIndexAll(ctx, i.logger, i.storageClient, bucketName, rootStoragePath, *storageCursor)
	if err != nil {
		metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
		return fmt.Errorf("can't load the search-index-all index content: %w", err)
	}

	if anIndex == nil {
		i.logger.Info("Downloaded new search-index-all index. No packages found.")
		return nil
	}
	i.logger.Info("Downloaded new search-index-all index", zap.String("index.packages.size", fmt.Sprintf("%d", len(*anIndex))))

	i.transformSearchIndexAllToPackages(anIndex)

	i.logger.Info("Updating database")
	err = i.updateDatabase(ctx, anIndex)
	if err != nil {
		return fmt.Errorf("failed to update database: %w", err)
	}

	startLock := time.Now()
	err = func() error {
		i.m.Lock()
		defer i.m.Unlock()
		i.cursor = storageCursor.Current

		// swap databases
		i.current, i.backup = i.backup, i.current
		i.logger.Debug("Current database changed", zap.String("current.database.path", (*i.current).File(ctx)), zap.String("previous.database.path", (*i.backup).File(ctx)))

		metrics.StorageIndexerUpdateIndexSuccessTotal.Inc()
		metrics.NumberIndexedPackages.Set(float64(len(*anIndex)))
		return nil
	}()
	i.logger.Info("Elapsed time in lock for updating index database", zap.Duration("lock.duration", time.Since(startLock)))

	if err != nil {
		metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
		return err
	}
	return nil
}

func (i *Indexer) updateDatabase(ctx context.Context, index *packages.Packages) error {
	span, ctx := apm.StartSpan(ctx, "updateDatabase", "app")
	defer span.End()

	err := (*i.backup).Drop(ctx, "packages")
	if err != nil {
		return fmt.Errorf("failed to drop packages table: %w", err)
	}

	err = (*i.backup).Migrate(ctx)
	if err != nil {
		return fmt.Errorf("failed to create schema in backup database: %w", err)
	}

	totalProcessed := 0
	maxBatch := 500
	dbPackages := make([]*database.Package, 0, maxBatch)
	for {
		read := 0
		// reuse slice to avoid allocations
		dbPackages = dbPackages[:0]
		endBatch := totalProcessed + maxBatch
		for i := totalProcessed; i < endBatch && i < len(*index); i++ {
			fullContents, err := json.Marshal((*index)[i])
			if err != nil {
				return fmt.Errorf("failed to marshal package %s-%s: %w", (*index)[i].Name, (*index)[i].Version, err)
			}
			baseContents, err := json.Marshal((*index)[i].BasePackage)
			if err != nil {
				return fmt.Errorf("failed to marshal base package %s-%s: %w", (*index)[i].Name, (*index)[i].Version, err)
			}

			discoveryFields := strings.Builder{}
			if (*index)[i].Discovery != nil {
				for i, field := range (*index)[i].Discovery.Fields {
					discoveryFields.WriteString(field.Name)
					if i < len((*index)[i].Discovery.Fields)-1 {
						discoveryFields.WriteString(",")
					}
				}
			}

			kibanaVersion := ""
			if (*index)[i].Conditions != nil && (*index)[i].Conditions.Kibana != nil {
				kibanaVersion = (*index)[i].Conditions.Kibana.Version
			}

			capabilities := ""
			if (*index)[i].Conditions != nil && (*index)[i].Conditions.Elastic != nil {
				capabilities = strings.Join((*index)[i].Conditions.Elastic.Capabilities, ",")
			}

			categories := strings.Join((*index)[i].Categories, ",")
			for _, policyTemplate := range (*index)[i].PolicyTemplates {
				if len(policyTemplate.Categories) == 0 {
					continue
				}
				categories += fmt.Sprintf(",%s", strings.Join(policyTemplate.Categories, ","))
			}

			if (*index)[i].Conditions != nil && (*index)[i].Conditions.Elastic != nil {
				categories = strings.Join((*index)[i].Conditions.Elastic.Capabilities, ",")
			}

			newPackage := database.Package{
				Name:            (*index)[i].Name,
				Version:         (*index)[i].Version,
				FormatVersion:   (*index)[i].FormatVersion,
				Path:            (*index)[i].BasePath,
				Type:            (*index)[i].Type,
				Release:         (*index)[i].Release,
				KibanaVersion:   kibanaVersion,
				Categories:      categories,
				Capabilities:    capabilities,
				DiscoveryFields: discoveryFields.String(),
				Prerelease:      (*index)[i].IsPrerelease(),
				Data:            string(fullContents),
				BaseData:        string(baseContents),
			}

			dbPackages = append(dbPackages, &newPackage)
			read++
		}
		err = (*i.backup).BulkAdd(ctx, "packages", dbPackages)
		if err != nil {
			return fmt.Errorf("failed to create all packages (bulk operation): %w", err)
		}
		totalProcessed += read
		if totalProcessed >= len(*index) {
			break
		}
		printMemUsage()
	}

	return nil
}

func (i *Indexer) Get(ctx context.Context, opts *packages.GetOptions) (packages.Packages, error) {
	start := time.Now()
	defer func() {
		metrics.IndexerGetDurationSeconds.With(prometheus.Labels{"indexer": indexerGetDurationPrometheusLabel}).Observe(time.Since(start).Seconds())
	}()
	span, ctx := apm.StartSpan(ctx, "GetPackages-StorageIndexer", "app")
	defer span.End()

	// TODO: To be removed
	profBaseName := "get-preprocess-columns-all-basedata-fast-json.prof"
	// f, err := os.Create("cpu-" + profBaseName)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create CPU profile: %w", err)
	// }
	// defer f.Close()

	// if err := pprof.StartCPUProfile(f); err != nil {
	// 	return nil, fmt.Errorf("failed to start CPU profile: %w", err)
	// }
	// defer pprof.StopCPUProfile()

	var readPackages packages.Packages
	err := func() error {
		i.m.RLock()
		defer i.m.RUnlock()

		options := &database.SQLOptions{}
		if opts != nil && opts.Filter != nil {
			options.Filter = &database.FilterOptions{
				Type:       opts.Filter.PackageType,
				Name:       opts.Filter.PackageName,
				Version:    opts.Filter.PackageVersion,
				Prerelease: opts.Filter.Prerelease,
			}
			if opts.Filter.Experimental {
				options.Filter.Prerelease = true
			}
		}

		numPackages := 0
		err := (*i.current).AllFunc(ctx, "packages", options, func(ctx context.Context, p *database.Package) error {

			pkg, err := packages.NewPackageWithOptions(
				packages.WithName(p.Name),
				packages.WithVersion(p.Version),
				packages.WithFormatVersion(p.FormatVersion),
				packages.WithRelease(p.Release),
				packages.WithKibanaVersion(p.KibanaVersion),
				packages.WithCapabilities(p.Capabilities),
				packages.WithCategories(p.Categories),
				packages.WithType(p.Type),
				packages.WithDiscoveryFields(p.DiscoveryFields),
			)
			if err != nil {
				return fmt.Errorf("failed to create package %s-%s: %w", p.Name, p.Version, err)
			}

			numPackages++
			// First phase filtering packages
			if opts != nil && opts.Filter != nil {
				pkgs, err := opts.Filter.Apply(ctx, packages.Packages{pkg})
				if err != nil {
					return err
				}
				if len(pkgs) == 0 {
					return nil
				}
			}
			if opts.FullData {
				err = json.Unmarshal([]byte(p.Data), pkg)
			} else {
				err = json.Unmarshal([]byte(p.BaseData), pkg)
			}
			if err != nil {
				return fmt.Errorf("failed to parse package %s-%s: %w", p.Name, p.Version, err)
			}
			pkg.SetRemoteResolver(i.resolver)
			readPackages = append(readPackages, pkg)
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to obtain all packages: %w", err)
		}
		i.logger.Debug("Number of packages read from database", zap.Int("num.packages", numPackages))

		// Required to filter packages again if condition `all=false`
		if opts != nil && opts.Filter != nil {
			readPackages, err = opts.Filter.Apply(ctx, readPackages)
			if err != nil {
				return fmt.Errorf("failed to filter packages: %w", err)
			}
			return nil
		}

		return nil
	}()
	if err != nil {
		return nil, err
	}

	// TODO: To be removed
	mf, err := os.Create("mem-" + profBaseName)
	if err != nil {
		return nil, fmt.Errorf("could not create memory profile: %w", err)
	}
	defer mf.Close()
	runtime.GC() // get up-to-date statistics

	if err := pprof.Lookup("heap").WriteTo(mf, 0); err != nil {
		return nil, fmt.Errorf("could not write memory profile: %w", err)
	}

	return readPackages, nil
}

func (i *Indexer) Close(ctx context.Context) error {
	// Try to close all databases
	err := i.database.Close(ctx)
	errSwap := i.swapDatabase.Close(ctx)

	errors.Join(err, errSwap)
	return errors.Join(err, errSwap)
}

func (i *Indexer) transformSearchIndexAllToPackages(packages *packages.Packages) {
	for _, m := range *packages {
		m.BasePath = fmt.Sprintf("%s-%s.zip", m.Name, m.Version)
		m.SetRemoteResolver(i.resolver)
	}
}

func printMemUsage() {
	// var m runtime.MemStats
	// runtime.GC()
	// runtime.ReadMemStats(&m)
	// fmt.Printf("Alloc = %v MiB", m.Alloc/1024/1024)
	// fmt.Printf("\tTotalAlloc = %v MiB", m.TotalAlloc/1024/1024)
	// fmt.Printf("\tSys = %v MiB", m.Sys/1024/1024)
	// fmt.Printf("\tNumGC = %v\n", m.NumGC)
}
