// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package database

import "context"

type Repository interface {
	Migrate(ctx context.Context) error
	Create(ctx context.Context, database string, pkg *Package) (*Package, error)
	All(ctx context.Context, database string) ([]Package, error)
	AllFunc(ctx context.Context, database string, process func(ctx context.Context, pkg *Package) error) error
	GetByName(ctx context.Context, database, name string) (*Package, error)
	Update(ctx context.Context, database string, id int64, updated *Package) (*Package, error)
	Delete(ctx context.Context, database string, id int64) error
	Drop(ctx context.Context, table string) error
	Rename(ctx context.Context, from, to string) error
	Close(ctx context.Context) error
}
