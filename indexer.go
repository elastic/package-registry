// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"

	"github.com/elastic/package-registry/packages"
)

type Indexer interface {
	Init(context.Context) error
	Get(context.Context, *packages.GetOptions) (packages.Packages, error)
}

type CombinedIndexer []Indexer

func NewCombinedIndexer(indexers ...Indexer) CombinedIndexer {
	return indexers
}

func (c CombinedIndexer) Init(ctx context.Context) error {
	for _, indexer := range c {
		err := indexer.Init(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c CombinedIndexer) Get(ctx context.Context, opts *packages.GetOptions) (packages.Packages, error) {
	var packages packages.Packages
	for _, indexer := range c {
		p, err := indexer.Get(ctx, opts)
		if err != nil {
			return nil, err
		}
		packages = packages.Join(p)
	}
	return packages, nil
}
