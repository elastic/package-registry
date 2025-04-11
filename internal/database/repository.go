package database

type Repository interface {
	Migrate() error
	Create(Package) (*Package, error)
	All() ([]Package, error)
	GetByName(name string) (*Package, error)
	Update(id int64, updated Package) (*Package, error)
	Delete(id int64) error
}
