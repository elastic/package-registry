// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package database

import "context"

type Repository interface {
	Migrate(ctx context.Context) error
	Create(ctx context.Context, pkg Package) (*Package, error)
	All(ctx context.Context) ([]Package, error)
	GetByName(ctx context.Context, name string) (*Package, error)
	GetByIndexer(ctx context.Context, indexer string) ([]Package, error)
	Update(ctx context.Context, id int64, updated Package) (*Package, error)
	Delete(ctx context.Context, id int64) error
}
