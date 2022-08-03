// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package proxy

import (
	"context"
	"net/http"
	"time"

	"github.com/elastic/package-registry/packages"
)

type Indexer struct {
	options IndexerOptions

	httpClient *http.Client
}

func NewIndexer(options IndexerOptions) *Indexer {
	return &Indexer{
		options: options,
	}
}

type IndexerOptions struct {
	ProxyTo string
}

func (i *Indexer) Init(ctx context.Context) error {
	i.httpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	return nil
}

func (i *Indexer) Get(ctx context.Context, opts *packages.GetOptions) (packages.Packages, error) {
	panic("Get: not implemented yet")
}
