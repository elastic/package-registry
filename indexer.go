// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"

	"github.com/elastic/package-registry/util"
)

type Indexer interface {
	GetPackages(context.Context, *util.GetPackagesOptions) (util.Packages, error)
}

type CombinedIndexer []Indexer

func NewCombinedIndexer(indexers ...Indexer) CombinedIndexer {
	return CombinedIndexer(indexers)
}

func (c CombinedIndexer) GetPackages(ctx context.Context, opts *util.GetPackagesOptions) (util.Packages, error) {
	var packages util.Packages
	for _, indexer := range c {
		p, err := indexer.GetPackages(ctx, opts)
		if err != nil {
			return nil, err
		}
		packages = packages.Join(p)
	}
	return packages, nil
}
