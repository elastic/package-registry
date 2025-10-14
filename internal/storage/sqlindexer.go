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
	"github.com/Masterminds/semver/v3"
	"github.com/prometheus/client_golang/prometheus"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/internal/database"
	"github.com/elastic/package-registry/metrics"
	"github.com/elastic/package-registry/packages"
)

const (
	indexerGetDurationPrometheusLabel = "SQLStorageIndexer"
	defaultReadPackagesBatchSize      = 2000
)

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

	afterUpdateHook func(ctx context.Context)
}

type IndexerOptions struct {
	APMTracer                    *apm.Tracer
	PackageStorageBucketInternal string
	PackageStorageEndpoint       string
	WatchInterval                time.Duration
	Database                     database.Repository
	SwapDatabase                 database.Repository
	ReadPackagesBatchsize        int
	AfterUpdateIndexHook         func(ctx context.Context)
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
		cursor:                "init",
		afterUpdateHook:       options.AfterUpdateIndexHook,
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

	if i.afterUpdateHook != nil {
		i.afterUpdateHook(ctx)
	}

	metrics.StorageIndexerUpdateIndexSuccessTotal.Inc()
	metrics.NumberIndexedPackages.Set(float64(numPackages))
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

	kibanaVersion := ""
	if pkg.Conditions != nil && pkg.Conditions.Kibana != nil {
		kibanaVersion = pkg.Conditions.Kibana.Version
	}

	pkgVersionSemver, err := semver.NewVersion(pkg.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to create version from %q: %w", pkgVersionSemver, err)
	}

	formatVersionSemver, err := semver.NewVersion(pkg.FormatVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid format version '%s' for package %s-%s: %w", pkg.FormatVersion, pkg.Name, pkg.Version, err)
	}
	formatVersionMajorMinor := fmt.Sprintf("%d.%d.0", formatVersionSemver.Major(), formatVersionSemver.Minor())

	newPackage := database.Package{
		Cursor:                  cursor,
		Name:                    pkg.Name,
		Version:                 pkg.Version,
		VersionMajor:            int(pkgVersionSemver.Major()),
		VersionMinor:            int(pkgVersionSemver.Minor()),
		VersionPatch:            int(pkgVersionSemver.Patch()),
		VersionBuild:            pkgVersionSemver.Prerelease(),
		FormatVersion:           pkg.FormatVersion,
		FormatVersionMajorMinor: formatVersionMajorMinor,
		Path:                    fmt.Sprintf("%s-%s.zip", pkg.Name, pkg.Version),
		Type:                    pkg.Type,
		Release:                 pkg.Release,
		KibanaVersion:           kibanaVersion,
		Prerelease:              pkg.IsPrerelease(),
		Data:                    fullContents,
		BaseData:                baseContents,
	}

	return &newPackage, nil
}

// Get returns the list of packages from the indexer, optionally filtered by the provided options.
// If filter is nil, all packages are returned with the base data of the package.
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

		options := createDatabaseOptions(i.cursor, opts)
		queryJustLatestPackages := false
		if opts != nil && opts.Filter != nil {
			// Determine if we can use the optimized query to get just the latest packages.
			// We can use it when we are not filtering by version, not requesting all versions,
			// and not filtering by capabilities or discovery. As capabilities and discovery are not
			// supported at database level, we can only use the optimized query when they are not set.
			// If capabilities or discovery filters are added, it needs to be checked that they can be
			// applied when querying for the latest packages.
			queryJustLatestPackages = !opts.Filter.AllVersions && opts.Filter.PackageVersion == "" && len(opts.Filter.Capabilities) == 0 && opts.Filter.Discovery == nil
		}
		queryFunc := (*i.current).AllFunc
		if queryJustLatestPackages {
			queryFunc = (*i.current).LatestFunc
		}

		err := queryFunc(ctx, "packages", options, func(ctx context.Context, p *database.Package) error {
			pkg := &packages.Package{}
			var err error
			switch {
			case opts != nil && opts.SkipPackageData:
				// Set minimal package data.
				// There are some private fields of the package that are not set here (versionSemver, specMajorMinorSemver, etc.),
				// but they should not be needed when SkipPackageData is used.
				pkg.Name = p.Name
				pkg.Version = p.Version
				pkg.FormatVersion = p.FormatVersion
				pkg.Release = p.Release
				pkg.Path = p.Path
			case opts != nil && opts.FullData:
				err = json.Unmarshal(p.Data, pkg)
				if err != nil {
					return fmt.Errorf("failed to parse full package %s-%s: %w", p.Name, p.Version, err)
				}
			default:
				// BaseData is used for performance reasons, it contains only the fields that are needed for the search index.
				// FormatVersion needs to be set from database to ensure compatibility with the package structure.
				pkg.FormatVersion = p.FormatVersion
				err = json.Unmarshal(p.BaseData, pkg)
				if err != nil {
					return fmt.Errorf("failed to parse base package %s-%s: %w", p.Name, p.Version, err)
				}
			}
			pkg.BasePath = p.Path
			pkg.SetRemoteResolver(i.resolver)
			readPackages = append(readPackages, pkg)
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to obtain packages: %w", err)
		}
		i.logger.Debug("Number of packages read from database", zap.Int("num.packages", len(readPackages)))

		if opts != nil && opts.Filter != nil {
			if opts.Filter.PackageName != "" && opts.Filter.PackageVersion != "" {
				if len(readPackages) > 1 {
					return fmt.Errorf("expected at most one package when filtering by name and version, got %d", len(readPackages))
				}
				// If we are filtering by name and version at database level, there should be at most one package and it can be returned early.
				return nil
			}
			var err error
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

func createDatabaseOptions(cursor string, opts *packages.GetOptions) *database.SQLOptions {
	sqlOptions := &database.SQLOptions{
		CurrentCursor:   cursor,
		IncludeFullData: false,
		SkipPackageData: false,
	}
	if opts == nil {
		return sqlOptions
	}

	sqlOptions.IncludeFullData = opts.FullData
	sqlOptions.SkipPackageData = opts.SkipPackageData

	if opts.Filter == nil {
		return sqlOptions
	}

	if opts.Filter.Experimental {
		// Experimental is also used in endpoints like /package or /epr to get a specific package.
		// https://github.com/elastic/package-registry/blob/4b4eea9301902c15a75a8ef303c6e719f9ff6abd/packages/packages.go#L645

		// If experimental is set, then it should be applied the same filters as in the legacyApply function:
		// https://github.com/elastic/package-registry/blob/4b4eea9301902c15a75a8ef303c6e719f9ff6abd/packages/packages.go#L524
		sqlOptions.Filter = &database.FilterOptions{
			Type:    opts.Filter.PackageType,
			Name:    opts.Filter.PackageName,
			Version: opts.Filter.PackageVersion,
			// When experimental is set, prerelease should also be included.
			Prerelease: true,
		}

		if opts.Filter.KibanaVersion != nil {
			sqlOptions.Filter.KibanaVersion = opts.Filter.KibanaVersion.String()
		}

		return sqlOptions
	}

	// TODO: Add support to filter by discovery fields if possible.
	// TODO: Add support to filter by capabilities if possible, relates to https://github.com/elastic/package-registry/pull/1396/
	sqlOptions.Filter = &database.FilterOptions{
		Type:       opts.Filter.PackageType,
		Name:       opts.Filter.PackageName,
		Version:    opts.Filter.PackageVersion,
		Prerelease: opts.Filter.Prerelease,
	}
	if opts.Filter.KibanaVersion != nil {
		sqlOptions.Filter.KibanaVersion = opts.Filter.KibanaVersion.String()
	}
	if opts.Filter.SpecMin != nil {
		sqlOptions.Filter.SpecMin = opts.Filter.SpecMin.String()
	}
	if opts.Filter.SpecMax != nil {
		sqlOptions.Filter.SpecMax = opts.Filter.SpecMax.String()
	}

	return sqlOptions
}

func (i *SQLIndexer) Close(ctx context.Context) error {
	err := i.database.Close(ctx)
	errSwap := i.swapDatabase.Close(ctx)

	errors.Join(err, errSwap)
	return errors.Join(err, errSwap)
}
