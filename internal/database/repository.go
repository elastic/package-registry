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
