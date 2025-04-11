package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/mattn/go-sqlite3"
)

const databaseName = "packages"

var (
	ErrDuplicate    = errors.New("record already exists")
	ErrNotExists    = errors.New("row not exists")
	ErrUpdateFailed = errors.New("update failed")
	ErrDeleteFailed = errors.New("delete failed")
)

type SQLiteRepository struct {
	db *sql.DB
}

var _ Repository = new(SQLiteRepository)

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{
		db: db,
	}
}

func (r *SQLiteRepository) Migrate() error {
	query := fmt.Sprintf(`
    CREATE TABLE IF NOT EXISTS %s (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
		version TEXT NOT NULL,
        data TEXT NOT NULL
    );
    `, databaseName)
	_, err := r.db.Exec(query)
	return err
}

func (r *SQLiteRepository) Create(pkg Package) (*Package, error) {
	query := fmt.Sprintf("INSERT INTO %s(name, version, data) values(?,?,?)", databaseName)
	res, err := r.db.Exec(query, pkg.Name, pkg.Version, pkg.Data)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) {
			if errors.Is(sqliteErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
				return nil, ErrDuplicate
			}
		}
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	pkg.ID = id

	return &pkg, nil
}

func (r *SQLiteRepository) All() ([]Package, error) {
	query := fmt.Sprintf("SELECT * FROM %s", databaseName)
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []Package
	for rows.Next() {
		var pkg Package
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Data); err != nil {
			return nil, err
		}
		all = append(all, pkg)
	}
	return all, nil
}

func (r *SQLiteRepository) GetByName(name string) (*Package, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE name = ?", name)
	row := r.db.QueryRow(query)

	var pkg Package
	if err := row.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotExists
		}
		return nil, err
	}
	return &pkg, nil
}
func (r *SQLiteRepository) Update(id int64, updated Package) (*Package, error) {
	if id == 0 {
		return nil, errors.New("invalid updated ID")
	}
	query := fmt.Sprintf("UPDATE %s SET name = ?, version = ?, data = ? WHERE id = ?", databaseName)
	res, err := r.db.Exec(query, updated.Name, updated.Version, updated.Data, id)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrUpdateFailed
	}

	return &updated, nil
}

func (r *SQLiteRepository) Delete(id int64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", databaseName)
	res, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrDeleteFailed
	}

	return err
}
