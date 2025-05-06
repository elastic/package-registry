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

	database database.Repository

	logger *zap.Logger

	initializing bool
}
type IndexerOptions struct {
	APMTracer                    *apm.Tracer
	PackageStorageBucketInternal string
	PackageStorageEndpoint       string
	WatchInterval                time.Duration
	Database                     database.Repository
}

func NewIndexer(logger *zap.Logger, storageClient *storage.Client, options IndexerOptions) *Indexer {
	if options.APMTracer == nil {
		options.APMTracer = apm.DefaultTracer()
	}
	return &Indexer{
		storageClient: storageClient,
		options:       options,
		logger:        logger,
		database:      options.Database,
		label:         fmt.Sprintf("storage-%s", options.PackageStorageEndpoint),
		initializing:  true,
	}
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

	i.initializing = false
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
	defer metrics.StorageIndexerUpdateIndexDurationSeconds.Observe(time.Since(start).Seconds())

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

	allPackagesVisited := map[string]visitedPackage{}
	err = i.database.AllFunc(ctx, "packages", func(ctx context.Context, pkg *database.Package) error {
		key := fmt.Sprintf("%s-%s", pkg.Name, pkg.Version)
		allPackagesVisited[key] = visitedPackage{
			visited: false,
			id:      pkg.ID,
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to obtain all packages: %w", err)
	}
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

	err = func() error {
		i.m.Lock()
		defer i.m.Unlock()
		i.cursor = storageCursor.Current

		if i.initializing {
			i.logger.Info("Creating first data")
			err := i.addInitialDataToDatabase(ctx, anIndex)
			if err != nil {
				return fmt.Errorf("failed to add initial data to database: %w", err)
			}
		} else {
			i.logger.Info("Updating database")
			err := i.updateDatabase(ctx, anIndex, allPackagesVisited)
			if err != nil {
				return fmt.Errorf("failed to update database: %w", err)
			}
		}

		metrics.StorageIndexerUpdateIndexSuccessTotal.Inc()
		metrics.NumberIndexedPackages.Set(float64(len(*anIndex)))
		return nil
	}()
	if err != nil {
		metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
		return err
	}
	return nil
}

type visitedPackage struct {
	id      int64
	visited bool
}

func (i *Indexer) addInitialDataToDatabase(ctx context.Context, index *packages.Packages) error {
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
			Data:    string(contents),
		}

		dbPackages[index] = &newPackage
	}

	err := i.database.BulkCreate(ctx, "packages", dbPackages)
	if err != nil {
		return fmt.Errorf("failed to create all packages (bulk operation): %w", err)
	}

	return nil
}
func (i *Indexer) updateDatabase(ctx context.Context, index *packages.Packages, visited map[string]visitedPackage) error {
	for _, p := range *index {
		contents, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal package %s-%s: %w", p.Name, p.Version, err)
		}
		dbPackage, err := i.database.GetByNameAndVersion(ctx, "packages", p.Name, p.Version)
		if err != nil && !errors.Is(err, database.ErrNotExists) {
			return fmt.Errorf("failed to search for package %s-%s", p.Name, p.Version)
		}
		if err != nil {
			// Package does not exist, it requires to be created
			newPackage := database.Package{
				Name:    p.Name,
				Version: p.Version,
				Path:    p.BasePath,
				Data:    string(contents),
			}
			_, err = i.database.Create(ctx, "packages", &newPackage)
			if err != nil {
				return fmt.Errorf("failed to create package %s-%s: %w", p.Name, p.Version, err)
			}
			continue
		}
		key := fmt.Sprintf("%s-%s", dbPackage.Name, dbPackage.Version)
		visited[key] = visitedPackage{
			id:      visited[key].id,
			visited: true,
		}

		if dbPackage.Data != string(contents) {
			// Package differs in its contents, it requires to be updated
			dbPackage.Data = string(contents)
			_, err = i.database.Update(ctx, "packages", dbPackage.ID, dbPackage)
			if err != nil {
				return fmt.Errorf("failed to update package %s-%s: %w", p.Name, p.Version, err)
			}
			continue
		}
	}
	for k, v := range visited {
		if v.visited {
			continue
		}
		// Package not present anymore in the remote index, it requires to be deleted
		err := i.database.Delete(ctx, "packages", v.id)
		if err != nil {
			return fmt.Errorf("failed to delete package %s: %w", k, err)
		}
	}

	return nil
}

func (i *Indexer) Get(ctx context.Context, opts *packages.GetOptions) (packages.Packages, error) {
	start := time.Now()
	defer metrics.IndexerGetDurationSeconds.With(prometheus.Labels{"indexer": indexerGetDurationPrometheusLabel}).Observe(time.Since(start).Seconds())

	var readPackages packages.Packages
	err := func() error {
		i.m.RLock()
		defer i.m.RUnlock()
		err := i.database.AllFunc(ctx, "packages", func(ctx context.Context, p *database.Package) error {
			var pkg packages.Package
			err := json.Unmarshal([]byte(p.Data), &pkg)
			if err != nil {
				return fmt.Errorf("failed to parse package %s-%s: %w", p.Name, p.Version, err)
			}
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
	return i.database.Close(ctx)
}

func (i *Indexer) transformSearchIndexAllToPackages(packages *packages.Packages) {
	for _, m := range *packages {
		m.BasePath = fmt.Sprintf("%s-%s.zip", m.Name, m.Version)
		m.SetRemoteResolver(i.resolver)
	}
}
