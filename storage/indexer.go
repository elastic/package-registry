// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/util"
)

type Indexer struct {
	options       IndexerOptions
	storageClient *storage.Client

	cursor      string
	packageList packages.Packages

	m sync.RWMutex
}

type IndexerOptions struct {
	PackageStorageBucketInternal string
	PackageStorageBucketPublic   string
	WatchInterval                time.Duration
}

func NewIndexer(storageClient *storage.Client, options IndexerOptions) *Indexer {
	return &Indexer{
		storageClient: storageClient,
		options:       options,
	}
}

func (i *Indexer) Init(ctx context.Context) error {
	logger := util.Logger()
	logger.Debug("Initialize storage indexer")

	err := validateIndexerOptions(i.options)
	if err != nil {
		return errors.Wrapf(err, "validation failed")
	}

	// Populate index file for the first time.
	err = i.updateIndex(ctx)
	if err != nil {
		logger.Error("can't update index file", zap.Error(err))
	}

	go i.watchIndices(ctx)
	return nil
}

func validateIndexerOptions(options IndexerOptions) error {
	if !strings.HasPrefix(options.PackageStorageBucketInternal, "gs://") {
		return errors.New("missing or invalid options.PackageStorageBucketInternal")
	}
	if !strings.HasPrefix(options.PackageStorageBucketPublic, "gs://") {
		return errors.New("missing or invalid options.PackageStorageBucketPublic")
	}
	if options.WatchInterval < 0 {
		return errors.New("options.WatchInterval must be greater than or equal to 0")
	}
	return nil
}

func (i *Indexer) watchIndices(ctx context.Context) {
	logger := util.Logger()
	logger.Debug("Watch indices for changes")
	if i.options.WatchInterval == 0 {
		logger.Debug("No watcher configured, indices will not be updated (use only for testing purposes)")
		return
	}

	var err error
	t := time.NewTicker(i.options.WatchInterval)
	defer t.Stop()
	for {
		logger.Debug("watchIndices: start")

		err = i.updateIndex(ctx)
		if err != nil {
			logger.Error("can't update index file", zap.Error(err))
		}

		logger.Debug("watchIndices: finished")
		select {
		case <-ctx.Done():
			logger.Debug("watchIndices: quit")
			return
		case <-t.C:
		}
	}
}

func (i *Indexer) updateIndex(ctx context.Context) error {
	logger := util.Logger()
	logger.Debug("Update indices")

	bucketName, rootStoragePath, err := extractBucketNameFromURL(i.options.PackageStorageBucketInternal)
	if err != nil {
		return errors.Wrapf(err, "can't extract bucket name from URL (url: %s)", i.options.PackageStorageBucketInternal)
	}

	storageCursor, err := loadCursor(ctx, i.storageClient, bucketName, rootStoragePath)
	if err != nil {
		return errors.Wrap(err, "can't load latest cursor")
	}

	if storageCursor.Current == i.cursor {
		logger.Info("cursor is up-to-date", zap.String("cursor.current", i.cursor))
		return nil
	}
	logger.Info("cursor will be updated", zap.String("cursor.current", i.cursor), zap.String("cursor.next", storageCursor.Current))

	anIndex, err := loadSearchIndexAll(ctx, i.storageClient, bucketName, rootStoragePath, *storageCursor)
	if err != nil {
		return errors.Wrapf(err, "can't load the search-index-all index content")
	}
	logger.Info("Downloaded new search-index-all index", zap.String("index.packages.size", fmt.Sprintf("%d", len(anIndex.Packages))))

	refreshedList, err := transformSearchIndexAllToPackages(*anIndex)
	if err != nil {
		return errors.Wrap(err, "can't transform the search-index-all")
	}

	i.m.Lock()
	defer i.m.Unlock()
	i.packageList = refreshedList
	return nil
}

func (i *Indexer) Get(ctx context.Context, opts *packages.GetOptions) (packages.Packages, error) {
	i.m.RLock()
	defer i.m.RUnlock()

	if opts.Filter != nil {
		return opts.Filter.Apply(ctx, i.packageList), nil
	}
	return i.packageList, nil
}
