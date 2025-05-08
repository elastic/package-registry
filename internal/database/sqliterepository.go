// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

var (
	ErrDuplicate    = errors.New("record already exists")
	ErrNotExists    = errors.New("row not exists")
	ErrUpdateFailed = errors.New("update failed")
	ErrDeleteFailed = errors.New("delete failed")
)

type SQLiteRepository struct {
	db   *sql.DB
	path string
}

var _ Repository = new(SQLiteRepository)

func NewFileSQLDB(path string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	dbRepo := newSQLiteRepository(db)
	if err := dbRepo.Migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	dbRepo.path = path
	return dbRepo, nil
}

func newSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{
		db: db,
	}
}

func (r *SQLiteRepository) File(ctx context.Context) string {
	return r.path
}

func (r *SQLiteRepository) Migrate(ctx context.Context) error {
	// TODO : Set name and version as primary keys ?
	query := `
    CREATE TABLE IF NOT EXISTS %s (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
		version TEXT NOT NULL,
		path TEXT NOT NULL,
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

func (r *SQLiteRepository) BulkAdd(ctx context.Context, database string, pkgs []*Package) error {
	totalProcessed := 0
	maxBatch := 2000
	for {
		read := 0
		args := []any{} // make([]any, len(pkgs)*4)
		var sb strings.Builder
		sb.WriteString("INSERT INTO ")
		sb.WriteString(database)
		sb.WriteString("(name, version, path, data) values ")
		endBatch := totalProcessed + maxBatch
		for i := totalProcessed; i < endBatch && i < len(pkgs); i++ {
			sb.WriteString("(?, ?, ?, ?)")
			if i < endBatch-1 && i < len(pkgs)-1 {
				sb.WriteString(",")
			}
			args = append(args, pkgs[i].Name, pkgs[i].Version, pkgs[i].Path, pkgs[i].Data)
			read += 1
		}
		query := sb.String()

		_, err := r.db.ExecContext(ctx, query, args...)
		if err != nil {
			// From github.com/mattn/go-sqlite3
			// var sqliteErr sqlite3.Error
			// if errors.As(err, &sqliteErr) {
			// 	if errors.Is(sqliteErr.ExtendedCode, sqlite.ErrConstraintUnique) {
			// 		return nil, ErrDuplicate
			// 	}
			// }
			return err
		}

		totalProcessed += read
		if totalProcessed == len(pkgs) {
			break
		}
	}

	return nil
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
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Path, &pkg.Data); err != nil {
			return nil, err
		}
		all = append(all, pkg)
	}
	return all, nil
}

func (r *SQLiteRepository) AllFunc(ctx context.Context, database string, process func(ctx context.Context, pkg *Package) error) error {
	query := fmt.Sprintf("SELECT * FROM %s", database)
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var pkg Package
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Path, &pkg.Data); err != nil {
			return err
		}
		err = process(ctx, &pkg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) Drop(ctx context.Context, table string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", table)
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) Close(ctx context.Context) error {
	return r.db.Close()
}
