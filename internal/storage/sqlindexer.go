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

	"github.com/hashicorp/golang-lru/v2/expirable"

	"cloud.google.com/go/storage"

	"github.com/prometheus/client_golang/prometheus"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/categories"
	"github.com/elastic/package-registry/internal/database"
	"github.com/elastic/package-registry/metrics"
	"github.com/elastic/package-registry/packages"
)

const (
	indexerGetDurationPrometheusLabel = "SQLStorageIndexer"
	defaultReadPackagesBatchSize      = 2000
)

var allCategories = categories.DefaultCategories()

type SQLIndexer struct {
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

	readPackagesBatchSize int

	searchCache     *expirable.LRU[string, []byte] // Cache for search results
	categoriesCache *expirable.LRU[string, []byte] // Cache for categories results
}

type IndexerOptions struct {
	APMTracer                    *apm.Tracer
	PackageStorageBucketInternal string
	PackageStorageEndpoint       string
	WatchInterval                time.Duration
	Database                     database.Repository
	SwapDatabase                 database.Repository
	SearchCache                  *expirable.LRU[string, []byte] // Cache for search results
	CategoriesCache              *expirable.LRU[string, []byte] // Cache for categories results
	ReadPackagesBatchsize        int
}

func NewIndexer(logger *zap.Logger, storageClient *storage.Client, options IndexerOptions) *SQLIndexer {
	if options.APMTracer == nil {
		options.APMTracer = apm.DefaultTracer()
	}

	indexer := &SQLIndexer{
		storageClient:         storageClient,
		options:               options,
		logger:                logger,
		database:              options.Database,
		swapDatabase:          options.SwapDatabase,
		label:                 fmt.Sprintf("storage-%s", options.PackageStorageEndpoint),
		readPackagesBatchSize: defaultReadPackagesBatchSize,
		searchCache:           options.SearchCache,
		categoriesCache:       options.CategoriesCache,
		cursor:                "init",
	}

	indexer.current = &indexer.database
	indexer.backup = &indexer.swapDatabase

	if options.ReadPackagesBatchsize > 0 {
		indexer.readPackagesBatchSize = options.ReadPackagesBatchsize
	}

	return indexer
}

func (i *SQLIndexer) Init(ctx context.Context) error {
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

	if options.Database == nil || options.SwapDatabase == nil {
		return errors.New("options.Database and options.SwapDatabase must be set")
	}
	return nil
}

func (i *SQLIndexer) setupResolver() error {
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

	i.resolver = NewStorageResolver(&httpClient, baseURL)
	return nil
}

func (i *SQLIndexer) watchIndices(ctx context.Context) {
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

func (i *SQLIndexer) updateIndex(ctx context.Context) error {
	span, ctx := apm.StartSpan(ctx, "UpdateIndex", "app")
	span.Context.SetLabel("read.packages.batch.size", i.readPackagesBatchSize)
	defer span.End()

	i.logger.Debug("Update indices")
	start := time.Now()
	defer func() {
		metrics.StorageIndexerUpdateIndexDurationSeconds.Observe(time.Since(start).Seconds())
	}()

	defer func(initialCursor string) {
		if initialCursor == i.cursor {
			return
		}
		startClean := time.Now()
		if err := i.cleanBackupDatabase(ctx); err != nil {
			i.logger.Error("Failed to clean backup database", zap.Error(err))
		}
		startCleanDuration := time.Since(startClean)
		i.logger.Debug("Cleaned backup database", zap.Duration("elapsed.time", time.Since(startClean)), zap.String("elapsed.time.human", startCleanDuration.String()))
	}(i.cursor)

	numPackages := 0
	currentCursor, err := LoadPackagesAndCursorFromIndexBatches(ctx, i.logger, i.storageClient, i.options.PackageStorageBucketInternal, i.cursor, i.readPackagesBatchSize, func(ctx context.Context, pkgs packages.Packages, newCursor string) error {
		// This function is called for each batch of packages read from the index.
		startUpdate := time.Now()
		if err := i.updateDatabase(ctx, &pkgs, newCursor); err != nil {
			return fmt.Errorf("failed to update database: %w", err)
		}
		startDuration := time.Since(startUpdate)
		numPackages += len(pkgs)
		i.logger.Debug("Filled database with a batch of packages", zap.Duration("elapsed.time", startDuration), zap.String("elapsed.time.human", startDuration.String()), zap.Int("num.packages", len(pkgs)))
		return nil
	})
	if err != nil {
		metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
		return fmt.Errorf("can't load the search-index-all index content: %w", err)
	}
	if i.cursor == currentCursor {
		return nil
	}
	i.logger.Info("Downloaded new search-index-all index", zap.String("index.packages.size", fmt.Sprintf("%d", numPackages)))

	startLock := time.Now()
	i.swapDatabases(ctx, currentCursor, numPackages)
	i.logger.Debug("Elapsed time in lock for updating index database", zap.Duration("lock.duration", time.Since(startLock)))

	return nil
}

func (i *SQLIndexer) updateDatabase(ctx context.Context, index *packages.Packages, cursor string) error {
	span, ctx := apm.StartSpan(ctx, "updateDatabase", "app")
	defer span.End()

	totalProcessed := 0
	dbPackages := make([]*database.Package, 0, i.readPackagesBatchSize)
	for {
		endBatch := totalProcessed + i.readPackagesBatchSize
		for j := totalProcessed; j < endBatch && j < len(*index); j++ {

			newPackage, err := createDatabasePackage((*index)[j], cursor)
			if err != nil {
				return fmt.Errorf("failed to create database package %s-%s: %w", (*index)[j].Name, (*index)[j].Version, err)
			}

			dbPackages = append(dbPackages, newPackage)
		}
		err := (*i.backup).BulkAdd(ctx, "packages", dbPackages)
		if err != nil {
			return fmt.Errorf("failed to create all packages (bulk operation): %w", err)
		}
		totalProcessed += len(dbPackages)
		if totalProcessed >= len(*index) {
			break
		}
		// reuse slice to avoid allocations
		dbPackages = dbPackages[:0]
	}

	return nil
}

func (i *SQLIndexer) cleanBackupDatabase(ctx context.Context) error {
	span, ctx := apm.StartSpan(ctx, "cleanBackupDatabase", "app")
	defer span.End()

	err := (*i.backup).Drop(ctx, "packages")
	if err != nil {
		return fmt.Errorf("failed to drop packages table: %w", err)
	}

	err = (*i.backup).Initialize(ctx)
	if err != nil {
		return fmt.Errorf("failed to create schema in backup database: %w", err)
	}
	return nil
}

func (i *SQLIndexer) swapDatabases(ctx context.Context, currentCursor string, numPackages int) {
	i.m.Lock()
	defer i.m.Unlock()
	i.cursor = currentCursor

	i.current, i.backup = i.backup, i.current
	i.logger.Debug("Current database changed", zap.String("current.database.path", (*i.current).File(ctx)), zap.String("previous.database.path", (*i.backup).File(ctx)))

	i.purgeCaches()

	metrics.StorageIndexerUpdateIndexSuccessTotal.Inc()
	metrics.NumberIndexedPackages.Set(float64(numPackages))
}

func (i *SQLIndexer) purgeCaches() {
	// Purge the caches after updating the index
	// there could be new, updated or removed packages
	if i.searchCache != nil {
		i.searchCache.Purge()
	}
	if i.categoriesCache != nil {
		i.categoriesCache.Purge()
	}
}

func createDatabasePackage(pkg *packages.Package, cursor string) (*database.Package, error) {
	fullContents, err := json.Marshal(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal package %s-%s: %w", pkg.Name, pkg.Version, err)
	}
	baseContents, err := json.Marshal(pkg.BasePackage)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal base package %s-%s: %w", pkg.Name, pkg.Version, err)
	}

	discoveryFields := strings.Builder{}
	if pkg.Discovery != nil {
		for i, field := range pkg.Discovery.Fields {
			discoveryFields.WriteString(field.Name)
			if i < len(pkg.Discovery.Fields)-1 {
				discoveryFields.WriteString(",")
			}
		}
	}

	kibanaVersion := ""
	if pkg.Conditions != nil && pkg.Conditions.Kibana != nil {
		kibanaVersion = pkg.Conditions.Kibana.Version
	}

	capabilities := ""
	if pkg.Conditions != nil && pkg.Conditions.Elastic != nil {
		capabilities = strings.Join(pkg.Conditions.Elastic.Capabilities, ",")
	}

	pkgCategories := calculateAllCategories(pkg)

	newPackage := database.Package{
		Cursor:          cursor,
		Name:            pkg.Name,
		Version:         pkg.Version,
		FormatVersion:   pkg.FormatVersion,
		Path:            fmt.Sprintf("%s-%s.zip", pkg.Name, pkg.Version),
		Type:            pkg.Type,
		Release:         pkg.Release,
		KibanaVersion:   kibanaVersion,
		Categories:      strings.Join(pkgCategories, ","),
		Capabilities:    capabilities,
		DiscoveryFields: discoveryFields.String(),
		Prerelease:      pkg.IsPrerelease(),
		Data:            fullContents,
		BaseData:        baseContents,
	}

	return &newPackage, nil
}

// calculateAllCategories returns all categories for a given package, including those from policy templates and parent categories.
func calculateAllCategories(pkg *packages.Package) []string {
	pkgCategories := []string{}
	pkgCategories = append(pkgCategories, pkg.Categories...)
	for _, policyTemplate := range pkg.PolicyTemplates {
		if len(policyTemplate.Categories) == 0 {
			continue
		}
		pkgCategories = append(pkgCategories, policyTemplate.Categories...)
	}

	for _, category := range pkgCategories {
		if _, found := allCategories[category]; !found {
			continue
		}
		if allCategories[category].Parent == nil {
			continue
		}
		pkgCategories = append(pkgCategories, allCategories[category].Parent.Name)
	}
	return pkgCategories
}

func (i *SQLIndexer) Get(ctx context.Context, opts *packages.GetOptions) (packages.Packages, error) {
	start := time.Now()
	defer func() {
		metrics.IndexerGetDurationSeconds.With(prometheus.Labels{"indexer": indexerGetDurationPrometheusLabel}).Observe(time.Since(start).Seconds())
	}()
	span, ctx := apm.StartSpan(ctx, "GetStorageIndexer", "app")
	defer span.End()

	var readPackages packages.Packages
	err := func() error {
		i.m.RLock()
		defer i.m.RUnlock()

		options := &database.SQLOptions{
			CurrentCursor: i.cursor,
		}
		if opts != nil && opts.Filter != nil {
			// TODO: Add support to filter by discovery fields if possible.
			options.Filter = &database.FilterOptions{
				Type:         opts.Filter.PackageType,
				Name:         opts.Filter.PackageName,
				Version:      opts.Filter.PackageVersion,
				Prerelease:   opts.Filter.Prerelease,
				Category:     opts.Filter.Category,
				Capabilities: opts.Filter.Capabilities,
			}
			if opts.Filter.Experimental {
				options.Filter.Prerelease = true
			}
		}
		if opts != nil {
			options.IncludeFullData = opts.FullData
		}

		err := (*i.current).AllFunc(ctx, "packages", options, func(ctx context.Context, p *database.Package) error {
			var pkg packages.Package
			var err error
			if opts != nil && opts.FullData {
				err = json.Unmarshal(p.Data, &pkg)
			} else {
				// BaseData is used for performance reasons, it contains only the fields that are needed for the search index.
				// FormatVersion needs to be set from database to ensure compatibility with the package structure.
				pkg.FormatVersion = p.FormatVersion
				err = json.Unmarshal(p.BaseData, &pkg)
			}
			if err != nil {
				return fmt.Errorf("failed to parse package %s-%s: %w", p.Name, p.Version, err)
			}
			pkg.BasePath = p.Path
			pkg.SetRemoteResolver(i.resolver)
			readPackages = append(readPackages, &pkg)
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to obtain all packages: %w", err)
		}
		i.logger.Debug("Number of packages read from database", zap.Int("num.packages", len(readPackages)))

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

	return readPackages, nil
}

func (i *SQLIndexer) Close(ctx context.Context) error {
	err := i.database.Close(ctx)
	errSwap := i.swapDatabase.Close(ctx)

	errors.Join(err, errSwap)
	return errors.Join(err, errSwap)
}
