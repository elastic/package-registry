package database

type Package struct {
	ID      int64
	Name    string
	Path    string
	Version string
	Indexer string
	Data    string
}
