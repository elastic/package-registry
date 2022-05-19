// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
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
	storageClient *storage.Client
	anIndex       searchIndexAll
}

func NewIndexer(storageClient *storage.Client) *Indexer {
	return &Indexer{
		storageClient: storageClient,
	}
}

func (i *Indexer) Init(ctx context.Context) error {
	go i.watchIndices(ctx)
	return nil
}

func (i *Indexer) watchIndices(ctx context.Context) {
	logger := util.Logger()

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
	i.anIndex = searchIndexAll{ // TODO
		Packages: []packageIndex{},
	}
	return nil
}

func (i *Indexer) Get(ctx context.Context, options *packages.GetOptions) (packages.Packages, error) {
	var pkgs packages.Packages


	panic("not implemented yet")
}
