// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

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
		formatVersion TEXT NOT NULL,
		prerelease INTEGER NOT NULL,
		release TEXT NOT NULL,
		kibanaVersion TEXT NOT NULL,
		Capabilities TEXT NOT NULL,
		type TEXT NOT NULL,
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
	args := make([]any, 0, maxBatch*5)
	for {
		read := 0
		// reuse args slice
		args = args[:0]
		var sb strings.Builder
		sb.WriteString("INSERT INTO ")
		sb.WriteString(database)
		sb.WriteString("(name, version, formatVersion, release, prerelease, kibanaVersion, capabilities, specMajorMinorSemver, type, path, data) values ")
		endBatch := totalProcessed + maxBatch
		for i := totalProcessed; i < endBatch && i < len(pkgs); i++ {
			sb.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			if i < endBatch-1 && i < len(pkgs)-1 {
				sb.WriteString(",")
			}
			args = append(args, pkgs[i].Name, pkgs[i].Version, pkgs[i].FormatVersion, pkgs[i].Release, pkgs[i].Prerelease, pkgs[i].KibanaVersion, pkgs[i].Capabilities, pkgs[i].Type, pkgs[i].Path, pkgs[i].Data)
			read++
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
		if totalProcessed >= len(pkgs) {
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
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.FormatVersion, &pkg.Release, &pkg.Prerelease, &pkg.KibanaVersion, &pkg.Capabilities, &pkg.Type, &pkg.Path, &pkg.Data); err != nil {
			return nil, err
		}
		all = append(all, pkg)
	}
	return all, nil
}

func (r *SQLiteRepository) AllFunc(ctx context.Context, database string, whereOptions WhereOptions, process func(ctx context.Context, pkg *Package) error) error {
	var query strings.Builder
	query.WriteString("SELECT * FROM ")
	query.WriteString(database)
	if whereOptions != nil {
		query.WriteString(whereOptions.Where())
	}
	fmt.Println(query.String())
	rows, err := r.db.QueryContext(ctx, query.String())
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var pkg Package
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.FormatVersion, &pkg.Release, &pkg.Prerelease, &pkg.KibanaVersion, &pkg.Capabilities, &pkg.Type, &pkg.Path, &pkg.Data); err != nil {
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

type FilterOptions struct {
	Type       string
	Name       string
	Version    string
	Prerelease bool
}

type SQLOptions struct {
	Filter *FilterOptions
}

func (o *SQLOptions) Where() string {
	if o == nil || o.Filter == nil {
		return ""
	}
	var sb strings.Builder
	if o.Filter.Type != "" {
		sb.WriteString("type = '")
		sb.WriteString(o.Filter.Type)
		sb.WriteString("'")
	}

	if o.Filter.Name != "" {
		if sb.Len() > 0 {
			sb.WriteString(" AND ")
		}
		sb.WriteString("name = '")
		sb.WriteString(o.Filter.Name)
		sb.WriteString("'")
	}

	if o.Filter.Version != "" {
		if sb.Len() > 0 {
			sb.WriteString(" AND ")
		}
		sb.WriteString("version = '")
		sb.WriteString(o.Filter.Version)
		sb.WriteString("'")
	}

	if !o.Filter.Prerelease {
		if sb.Len() > 0 {
			sb.WriteString(" AND ")
		}
		sb.WriteString("prerelease = 0")
	}

	clause := sb.String()
	if clause == "" {
		return ""
	}
	return fmt.Sprintf(" WHERE %s", clause)
}
