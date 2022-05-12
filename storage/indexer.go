// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"

	"github.com/elastic/package-registry/packages"
)

type Indexer struct{}

func NewIndexer() *Indexer {
	return new(Indexer)
}

func (i *Indexer) Init(context.Context) error {
	return nil
}

func (i *Indexer) Get(context.Context, *packages.GetOptions) (packages.Packages, error) {
	panic("not implemented yet")
}
