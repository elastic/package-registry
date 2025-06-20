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
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"

	"github.com/prometheus/client_golang/prometheus"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/metrics"
	"github.com/elastic/package-registry/packages"

	internalStorage "github.com/elastic/package-registry/internal/storage"
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

	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				// Connect timeout.
				Timeout: 20 * time.Second,
			}).DialContext,
		},
	}

	i.resolver = internalStorage.NewStorageResolver(&httpClient, baseURL)
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

	anIndex, currentCursor, err := internalStorage.LoadPackagesAndCursorFromIndex(ctx, i.logger, i.storageClient, i.options.PackageStorageBucketInternal, i.cursor)
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

	i.m.Lock()
	defer i.m.Unlock()
	i.cursor = currentCursor
	i.packageList = *anIndex
	metrics.StorageIndexerUpdateIndexSuccessTotal.Inc()
	metrics.NumberIndexedPackages.Set(float64(len(i.packageList)))
	return nil
}

func (i *Indexer) Get(ctx context.Context, opts *packages.GetOptions) (packages.Packages, error) {
	start := time.Now()
	defer func() {
		metrics.IndexerGetDurationSeconds.With(prometheus.Labels{"indexer": indexerGetDurationPrometheusLabel}).Observe(time.Since(start).Seconds())
	}()

	span, ctx := apm.StartSpan(ctx, "GetStorageIndexer", "app")
	defer span.End()

	i.m.RLock()
	defer i.m.RUnlock()

	if opts != nil && opts.Filter != nil {
		return opts.Filter.Apply(ctx, i.packageList)
	}
	return i.packageList, nil
}

func (i *Indexer) transformSearchIndexAllToPackages(packages *packages.Packages) {
	for _, m := range *packages {
		m.BasePath = fmt.Sprintf("%s-%s.zip", m.Name, m.Version)
		m.SetRemoteResolver(i.resolver)
	}
}

func (i *Indexer) Close(ctx context.Context) error {
	return nil
}
