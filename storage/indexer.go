// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"

	"cloud.google.com/go/storage"

	"github.com/elastic/package-registry/packages"
)

type Indexer struct {
	storageClient *storage.Client
}

func NewIndexer(storageClient *storage.Client) *Indexer {
	return &Indexer{
		storageClient: storageClient,
	}
}

func (i *Indexer) Init(context.Context) error {
	return nil
}

func (i *Indexer) Get(context.Context, *packages.GetOptions) (packages.Packages, error) {
	panic("not implemented yet")
}
