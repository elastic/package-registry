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
