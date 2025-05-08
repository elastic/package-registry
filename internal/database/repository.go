// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package database

import "context"

type Repository interface {
	Migrate(ctx context.Context) error
	BulkAdd(ctx context.Context, database string, pkgs []*Package) error
	All(ctx context.Context, database string) ([]Package, error)
	AllFunc(ctx context.Context, database string, process func(ctx context.Context, pkg *Package) error) error
	Drop(ctx context.Context, table string) error
	Close(ctx context.Context) error

	File(ctx context.Context) string
}
