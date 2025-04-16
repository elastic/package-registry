// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
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

func NewFileSQLDB(path string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	dbRepo := NewSQLiteRepository(db)
	if err := dbRepo.Migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	return dbRepo, nil
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{
		db: db,
	}
}

func (r *SQLiteRepository) Migrate(ctx context.Context) error {
	// TODO : Set name and version as primary keys ?
	query := `
    CREATE TABLE IF NOT EXISTS %s (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
		version TEXT NOT NULL,
		path TEXT NOT NULL,
		indexer TEXT NOT NULL,
        data TEXT NOT NULL
    );
	`
	if _, err := r.db.ExecContext(ctx, fmt.Sprintf(query, "packages")); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, fmt.Sprintf(query, "packages_new")); err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) Create(ctx context.Context, database string, pkg *Package) (*Package, error) {
	query := fmt.Sprintf("INSERT INTO %s(name, version, path, indexer, data) values(?,?,?,?,?)", database)
	res, err := r.db.ExecContext(ctx, query, pkg.Name, pkg.Version, pkg.Path, pkg.Indexer, pkg.Data)
	if err != nil {
		// From github.com/mattn/go-sqlite3
		// var sqliteErr sqlite3.Error
		// if errors.As(err, &sqliteErr) {
		// 	if errors.Is(sqliteErr.ExtendedCode, sqlite.ErrConstraintUnique) {
		// 		return nil, ErrDuplicate
		// 	}
		// }
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	pkg.ID = id

	return pkg, nil
}

func (r *SQLiteRepository) All(ctx context.Context, database string) ([]Package, error) {
	query := fmt.Sprintf("SELECT * FROM %s", database)
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []Package
	for rows.Next() {
		var pkg Package
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Path, &pkg.Indexer, &pkg.Data); err != nil {
			return nil, err
		}
		all = append(all, pkg)
	}
	return all, nil
}

func (r *SQLiteRepository) GetByName(ctx context.Context, database, name string) (*Package, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE name = ?", database)
	row := r.db.QueryRowContext(ctx, query, name)

	var pkg Package
	if err := row.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Path, &pkg.Indexer, &pkg.Data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotExists
		}
		return nil, err
	}
	return &pkg, nil
}

func (r *SQLiteRepository) GetByIndexer(ctx context.Context, database, indexer string) ([]Package, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE indexer = ?", database)
	rows, err := r.db.QueryContext(ctx, query, indexer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []Package
	for rows.Next() {
		var pkg Package
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Path, &pkg.Indexer, &pkg.Data); err != nil {
			return nil, err
		}
		all = append(all, pkg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return all, nil
}

func (r *SQLiteRepository) GetByIndexerFunc(ctx context.Context, database, indexer string, process func(ctx context.Context, pkg *Package) error) error {
	query := fmt.Sprintf("SELECT * FROM %s WHERE indexer = ?", database)
	rows, err := r.db.QueryContext(ctx, query, indexer)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var pkg Package
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Path, &pkg.Indexer, &pkg.Data); err != nil {
			return err
		}
		err = process(ctx, &pkg)
		if err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) Update(ctx context.Context, database string, id int64, updated *Package) (*Package, error) {
	if id == 0 {
		return nil, errors.New("invalid updated ID")
	}
	query := fmt.Sprintf("UPDATE %s SET name = ?, version = ?, path = ? indexer = ? data = ? WHERE id = ?", database)
	res, err := r.db.ExecContext(ctx, query, updated.Name, updated.Version, updated.Path, updated.Indexer, updated.Data, id)
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

	return updated, nil
}

func (r *SQLiteRepository) Delete(ctx context.Context, database string, id int64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", database)
	res, err := r.db.ExecContext(ctx, query, id)
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

func (r *SQLiteRepository) Drop(ctx context.Context, table string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", table)
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) Rename(ctx context.Context, from, to string) error {
	query := fmt.Sprintf("ALTER TABLE %s RENAME TO %s", from, to)
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}
	return nil
}
