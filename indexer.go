// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"

	"github.com/elastic/package-registry/packages"
)

type Indexer interface {
	Init(context.Context) error
	Get(context.Context, *packages.GetOptions) (packages.Packages, error)
	Close(context.Context) error
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

	if opts != nil && opts.Filter != nil && !opts.Filter.AllVersions {
		return latestPackagesVersion(packages), nil
	}

	return packages, nil
}

func (c CombinedIndexer) Close(ctx context.Context) error {
	for _, indexer := range c {
		err := indexer.Close(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func latestPackagesVersion(source packages.Packages) (result packages.Packages) {
	packages.SortByNameVersion(source)

	current := ""
	for _, p := range source {
		if p.Name == current {
			continue
		}

		current = p.Name
		result = append(result, p)
	}

	return result
}
