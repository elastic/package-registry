// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

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

	i.logger.Debug("Elapsed time in lock for updating index database", zap.Duration("lock.duration", time.Since(startLock)))
	if err != nil {
		metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
		return err
	}
	return nil
}

func (i *Indexer) updateDatabase(ctx context.Context, index *packages.Packages) error {
	dbPackages := make([]*database.Package, len(*index))
	for index, pkg := range *index {
		contents, err := json.Marshal(pkg)
		if err != nil {
			return fmt.Errorf("failed to marshal package %s-%s: %w", pkg.Name, pkg.Version, err)
		}

		newPackage := database.Package{
			Name:    pkg.Name,
			Version: pkg.Version,
			Path:    pkg.BasePath,
			Type:    pkg.Type,
			Data:    string(contents),
		}

		dbPackages[index] = &newPackage
	}

	err := (*i.backup).Drop(ctx, "packages")
	if err != nil {
		return fmt.Errorf("failed to drop packages table: %w", err)
	}

	err = (*i.backup).Migrate(ctx)
	if err != nil {
		return fmt.Errorf("failed to create schema in backup database: %w", err)
	}

	err = (*i.backup).BulkAdd(ctx, "packages", dbPackages)
	if err != nil {
		return fmt.Errorf("failed to create all packages (bulk operation): %w", err)
	}
	return nil
}

func (i *Indexer) Get(ctx context.Context, opts *packages.GetOptions) (packages.Packages, error) {
	start := time.Now()
	defer func() {
		metrics.IndexerGetDurationSeconds.With(prometheus.Labels{"indexer": indexerGetDurationPrometheusLabel}).Observe(time.Since(start).Seconds())
	}()

	var readPackages packages.Packages
	err := func() error {
		i.m.RLock()
		defer i.m.RUnlock()

		options := database.AllOptions{}
		if opts != nil && opts.Filter != nil {
			options.Type = opts.Filter.PackageType
			options.Name = opts.Filter.PackageName
			options.Version = opts.Filter.PackageVersion
		}

		numPackages := 0
		err := (*i.current).AllFunc(ctx, "packages", &options, func(ctx context.Context, p *database.Package) error {

			var pkg packages.Package
			err := json.Unmarshal([]byte(p.Data), &pkg)
			if err != nil {
				return fmt.Errorf("failed to parse package %s-%s: %w", p.Name, p.Version, err)
			}
			numPackages++
			// First phase filtering packages
			if opts != nil && opts.Filter != nil {
				pkgs, err := opts.Filter.Apply(ctx, packages.Packages{&pkg})
				if err != nil {
					return err
				}
				if len(pkgs) == 0 {
					return nil
				}
			}
			pkg.SetRemoteResolver(i.resolver)
			readPackages = append(readPackages, &pkg)
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to obtain all packages: %w", err)
		}
		i.logger.Debug("Number of packages read from database", zap.Int("num.packages", numPackages))
		return nil
	}()
	if err != nil {
		return nil, err
	}

	// Required to filter packages again if condition `all=false`
	if opts != nil && opts.Filter != nil {
		pkgs, err := opts.Filter.Apply(ctx, readPackages)
		return pkgs, err
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
