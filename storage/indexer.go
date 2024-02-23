// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"

	"github.com/prometheus/client_golang/prometheus"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/metrics"
	"github.com/elastic/package-registry/packages"
)

const indexerGetDurationPrometheusLabel = "StorageIndexer"

type Indexer struct {
	options       IndexerOptions
	storageClient *storage.Client

	cursor      string
	packageList packages.Packages

	m sync.RWMutex

	resolver packages.RemoteResolver

	logger *zap.Logger
}

type IndexerOptions struct {
	APMTracer                    *apm.Tracer
	PackageStorageBucketInternal string
	PackageStorageEndpoint       string
	WatchInterval                time.Duration
}

func NewIndexer(logger *zap.Logger, storageClient *storage.Client, options IndexerOptions) *Indexer {
	if options.APMTracer == nil {
		options.APMTracer = apm.DefaultTracer()
	}
	return &Indexer{
		storageClient: storageClient,
		options:       options,
		logger:        logger,
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
	err = i.updateIndex(ctx)
	if err != nil {
		return fmt.Errorf("can't update index file: %w", err)
	}

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

	i.resolver = storageResolver{
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

	anIndex, err := loadSearchIndexAll(ctx, i.logger, i.storageClient, bucketName, rootStoragePath, *storageCursor)
	if err != nil {
		metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
		return fmt.Errorf("can't load the search-index-all index content: %w", err)
	}
	i.logger.Info("Downloaded new search-index-all index", zap.String("index.packages.size", fmt.Sprintf("%d", len(anIndex.Packages))))

	refreshedList, err := i.transformSearchIndexAllToPackages(*anIndex)
	if err != nil {
		metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
		return fmt.Errorf("can't transform the search-index-all: %w", err)
	}

	i.m.Lock()
	defer i.m.Unlock()
	i.cursor = storageCursor.Current
	i.packageList = refreshedList
	metrics.StorageIndexerUpdateIndexSuccessTotal.Inc()
	metrics.NumberIndexedPackages.Set(float64(len(i.packageList)))
	return nil
}

func (i *Indexer) Get(ctx context.Context, opts *packages.GetOptions) (packages.Packages, error) {
	start := time.Now()
	defer metrics.IndexerGetDurationSeconds.With(prometheus.Labels{"indexer": indexerGetDurationPrometheusLabel}).Observe(time.Since(start).Seconds())

	i.m.RLock()
	defer i.m.RUnlock()

	if opts != nil && opts.Filter != nil {
		return opts.Filter.Apply(ctx, i.packageList)
	}
	return i.packageList, nil
}

func (i *Indexer) transformSearchIndexAllToPackages(sia searchIndexAll) (packages.Packages, error) {
	var transformedPackages packages.Packages
	for j := range sia.Packages {
		m := sia.Packages[j].PackageManifest
		m.BasePath = fmt.Sprintf("%s-%s.zip", m.Name, m.Version)
		m.SetRemoteResolver(i.resolver)
		transformedPackages = append(transformedPackages, &m)
	}
	return transformedPackages, nil
}
