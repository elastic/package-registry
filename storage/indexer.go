// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/util"
)

const (
	watchInterval = 1 * time.Minute
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

	// TODO validate options

	// Populate index file for the first time.
	err := i.updateIndex()
	if err != nil {
		logger.Error("can't update index file", zap.Error(err))
	}

	go i.watchIndices(ctx)
	return nil
}

func (i *Indexer) watchIndices(ctx context.Context) {
	logger := util.Logger()
	logger.Debug("Watch indices for changes")

	var err error
	t := time.NewTicker(watchInterval)
	defer t.Stop()
	for {
		logger.Debug("watchIndices: start")

		err = i.updateIndex()
		if err != nil {
			logger.Error("can't update index file", zap.Error(err))
		}

		logger.Debug("watchIndices: finished")
		select {
		case <-ctx.Done():
			logger.Debug("watchIndices: quit")
			break
		case <-t.C:
		}
	}
}

func (i *Indexer) updateIndex() error {
	logger := util.Logger()
	logger.Debug("Update indices")

	// TODO Load cursor
	// TODO Check if cursor moved
	// TODO Rebuild package list

	i.m.Lock()
	defer i.m.Unlock()

	i.packageList = packages.Packages{}
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
