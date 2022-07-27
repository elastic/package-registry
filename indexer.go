// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"sort"

	"github.com/Masterminds/semver/v3"

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

	if !opts.Filter.AllVersions {
		return latestPackagesVersion(packages), nil
	}

	return packages, nil
}

func latestPackagesVersion(source packages.Packages) (result packages.Packages) {
	sort.Sort(byNameVersion(source))

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

type byNameVersion packages.Packages

func (p byNameVersion) Len() int      { return len(p) }
func (p byNameVersion) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p byNameVersion) Less(i, j int) bool {
	if p[i].Name != p[j].Name {
		return p[i].Name < p[j].Name
	}

	// Newer versions first.
	iSemVer, _ := semver.NewVersion(p[i].Version)
	jSemVer, _ := semver.NewVersion(p[j].Version)
	if iSemVer != nil && jSemVer != nil {
		return jSemVer.LessThan(iSemVer)
	}
	return p[j].Version < p[i].Version
}
