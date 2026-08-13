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

	cursor             string
	packageList        packages.Packages
	deprecatedPackages packages.DeprecatedPackages

	m sync.RWMutex

	resolver packages.RemoteResolver

	logger *zap.Logger
}

type IndexerOptions struct {
	APMTracer                    *apm.Tracer
	PackageStorageBucketInternal string
	PackageStorageEndpoint       string
	WatchInterval                time.Duration
	IncrementalUpdates           bool
}

func NewIndexer(logger *zap.Logger, storageClient *storage.Client, options IndexerOptions) *Indexer {
	if options.APMTracer == nil {
		options.APMTracer = apm.DefaultTracer()
	}
	return &Indexer{
		storageClient:      storageClient,
		options:            options,
		logger:             logger,
		deprecatedPackages: make(packages.DeprecatedPackages),
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

	latestCursorValue, err := internalStorage.LoadLatestCursorValue(ctx, i.logger, i.storageClient, i.options.PackageStorageBucketInternal)
	if err != nil {
		metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
		return fmt.Errorf("can't load latest cursor: %w", err)
	}
	if i.cursor == latestCursorValue {
		return nil
	}

	if i.cursor == "" || !i.options.IncrementalUpdates {
		return i.fullSync(ctx, latestCursorValue)
	}
	return i.incrementalSync(ctx, latestCursorValue)
}

func (i *Indexer) fullSync(ctx context.Context, latestCursorValue string) error {
	anIndex, err := internalStorage.LoadSearchIndexAllForCursor(ctx, i.logger, i.storageClient, i.options.PackageStorageBucketInternal, latestCursorValue)
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
	i.cursor = latestCursorValue
	i.packageList = *anIndex
	metrics.StorageIndexerUpdateIndexSuccessTotal.Inc()
	metrics.NumberIndexedPackages.Set(float64(len(i.packageList)))

	packages.UpdateLatestDeprecatedPackagesMapByName(i.packageList, i.deprecatedPackages)
	packages.PropagateLatestDeprecatedInfoToPackageList(i.packageList, i.deprecatedPackages)

	return nil
}

func (i *Indexer) incrementalSync(ctx context.Context, latestCursorValue string) error {
	timestamps, err := internalStorage.ListCursorsBetween(ctx, i.storageClient, i.options.PackageStorageBucketInternal, i.cursor, latestCursorValue)
	if err != nil {
		metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
		return fmt.Errorf("can't list cursors between %s and %s: %w", i.cursor, latestCursorValue, err)
	}

	type revision struct {
		timestamp string
		delta     *internalStorage.SearchIndexDelta
		prepared  *preparedDelta
		fullIndex *packages.Packages
	}

	revisions := make([]revision, 0, len(timestamps))
	for _, ts := range timestamps {
		delta, err := internalStorage.LoadSearchIndexDelta(ctx, i.logger, i.storageClient, i.options.PackageStorageBucketInternal, ts)
		if err != nil {
			if errors.Is(err, storage.ErrObjectNotExist) {
				i.logger.Warn("delta file missing, falling back to full sync for timestamp", zap.String("cursor", ts))
				anIndex, err := internalStorage.LoadSearchIndexAllForCursor(ctx, i.logger, i.storageClient, i.options.PackageStorageBucketInternal, ts)
				if err != nil {
					metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
					return fmt.Errorf("can't load search-index-all for cursor %s: %w", ts, err)
				}
				// Full sync supersedes all prior deltas — reset to avoid holding multiple full copies in memory.
				revisions = revisions[:0]
				revisions = append(revisions, revision{timestamp: ts, fullIndex: anIndex})
			} else {
				metrics.StorageIndexerUpdateIndexErrorsTotal.Inc()
				return fmt.Errorf("can't load delta for cursor %s: %w", ts, err)
			}
		} else {
			revisions = append(revisions, revision{timestamp: ts, delta: delta})
		}
	}

	if len(revisions) == 0 {
		return nil
	}

	// Pre-process all revisions before acquiring the lock; i.resolver is read-only after init.
	for j := range revisions {
		if revisions[j].fullIndex != nil {
			i.transformSearchIndexAllToPackages(revisions[j].fullIndex)
		} else if revisions[j].delta != nil {
			pd := i.prepareDelta(revisions[j].delta)
			revisions[j].prepared = &pd
			revisions[j].delta = nil
		}
	}

	i.m.Lock()
	defer i.m.Unlock()

	for _, r := range revisions {
		if r.fullIndex != nil {
			i.packageList = *r.fullIndex
		} else if r.prepared != nil {
			i.applyDelta(*r.prepared)
		}
		i.cursor = r.timestamp
	}

	metrics.StorageIndexerUpdateIndexSuccessTotal.Inc()
	metrics.NumberIndexedPackages.Set(float64(len(i.packageList)))

	packages.UpdateLatestDeprecatedPackagesMapByName(i.packageList, i.deprecatedPackages)
	packages.PropagateLatestDeprecatedInfoToPackageList(i.packageList, i.deprecatedPackages)

	return nil
}

type preparedDelta struct {
	removeKeys map[string]struct{}
	updateMap  map[string]*packages.Package
	added      packages.Packages
}

// prepareDelta builds lookup structures from a raw delta. Safe to call without holding i.m
// because it only reads delta.* and i.resolver, both of which are read-only after init.
func (i *Indexer) prepareDelta(delta *internalStorage.SearchIndexDelta) preparedDelta {
	removeKeys := make(map[string]struct{}, len(delta.Removed))
	for _, ref := range delta.Removed {
		removeKeys[ref.Name+"-"+ref.Version] = struct{}{}
	}

	updateMap := make(map[string]*packages.Package, len(delta.Updated))
	for _, entry := range delta.Updated {
		p := entry.PackageManifest
		p.BasePath = fmt.Sprintf("%s-%s.zip", p.Name, p.Version)
		p.SetRemoteResolver(i.resolver)
		updateMap[p.Name+"-"+p.Version] = &p
	}

	added := make(packages.Packages, 0, len(delta.Added))
	for _, entry := range delta.Added {
		p := entry.PackageManifest
		p.BasePath = fmt.Sprintf("%s-%s.zip", p.Name, p.Version)
		p.SetRemoteResolver(i.resolver)
		added = append(added, &p)
	}

	return preparedDelta{removeKeys: removeKeys, updateMap: updateMap, added: added}
}

// applyDelta applies a pre-processed delta to i.packageList.
// Must be called with i.m held for writing.
func (i *Indexer) applyDelta(pd preparedDelta) {
	out := make(packages.Packages, 0, max(0, len(i.packageList)-len(pd.removeKeys))+len(pd.added))
	for _, p := range i.packageList {
		key := p.Name + "-" + p.Version
		if _, removed := pd.removeKeys[key]; removed {
			continue
		}
		if newer, ok := pd.updateMap[key]; ok {
			out = append(out, newer)
			delete(pd.updateMap, key)
		} else {
			out = append(out, p)
		}
	}
	out = append(out, pd.added...)
	i.packageList = out
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
