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

	"go.elastic.co/apm/v2"
	_ "modernc.org/sqlite"
)

var (
	ErrDuplicate    = errors.New("record already exists")
	ErrNotExists    = errors.New("row not exists")
	ErrUpdateFailed = errors.New("update failed")
	ErrDeleteFailed = errors.New("delete failed")
)

const defaultMaxBulkAddBatch = 2000

type SQLiteRepository struct {
	db              *sql.DB
	path            string
	maxBulkAddBatch int
	numberFields    int
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
		db:              db,
		maxBulkAddBatch: defaultMaxBulkAddBatch,
		numberFields:    13,
	}
}

func (r *SQLiteRepository) File(ctx context.Context) string {
	return r.path
}

func (r *SQLiteRepository) Migrate(ctx context.Context) error {
	span, ctx := apm.StartSpan(ctx, "SQL: Migrate", "app")
	defer span.End()
	// TODO : Set name and version as primary keys ?
	query := `
    CREATE TABLE IF NOT EXISTS %s (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
		version TEXT NOT NULL,
		formatVersion TEXT NOT NULL,
		release TEXT NOT NULL,
		prerelease INTEGER NOT NULL,
		kibanaVersion TEXT NOT NULL,
		categories TEXT NOT NULL,
		capabilities TEXT NOT NULL,
		discoveryFields TEXT NOT NULL,
		type TEXT NOT NULL,
		path TEXT NOT NULL,
		data TEXT NOT NULL,
		baseData TEXT NOT NULL
    );
	`
	if _, err := r.db.ExecContext(ctx, fmt.Sprintf(query, "packages")); err != nil {
		return err
	}
	// TODO: review if category index is needed
	query = `
	CREATE INDEX idx_prerelease ON packages (prerelease);
	CREATE INDEX idx_name_version ON packages ( name, version);
	CREATE INDEX idx_type ON packages (type);
	CREATE INDEX idx_category ON packages (categories);
	`
	if _, err := r.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to create indices: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) BulkAdd(ctx context.Context, database string, pkgs []*Package) error {
	span, ctx := apm.StartSpan(ctx, "SQL: Insert batches", "app")
	defer span.End()

	if len(pkgs) == 0 {
		return nil
	}

	totalProcessed := 0
	args := make([]any, 0, r.maxBulkAddBatch*r.numberFields)
	for {
		read := 0
		// reuse args slice
		args = args[:0]
		var sb strings.Builder
		sb.WriteString("INSERT INTO ")
		sb.WriteString(database)
		sb.WriteString("(name, version, formatVersion, release, prerelease, kibanaVersion, ")
		sb.WriteString("categories, capabilities, discoveryFields, type, path, data, baseData) ")
		sb.WriteString(" values ")
		endBatch := totalProcessed + r.maxBulkAddBatch
		for i := totalProcessed; i < endBatch && i < len(pkgs); i++ {
			sb.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			if i < endBatch-1 && i < len(pkgs)-1 {
				sb.WriteString(",")
			}

			// Add commas to categories to make it easier to search for
			// categories in the SQL query
			categories := pkgs[i].Categories
			if categories != "" {
				categories = fmt.Sprintf(",%s,", categories)
			}

			args = append(args,
				pkgs[i].Name,
				pkgs[i].Version,
				pkgs[i].FormatVersion,
				pkgs[i].Release,
				pkgs[i].Prerelease,
				pkgs[i].KibanaVersion,
				categories,
				pkgs[i].Capabilities,
				pkgs[i].DiscoveryFields,
				pkgs[i].Type,
				pkgs[i].Path,
				pkgs[i].Data,
				pkgs[i].BaseData,
			)
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

func (r *SQLiteRepository) All(ctx context.Context, database string, whereOptions WhereOptions) ([]Package, error) {
	span, ctx := apm.StartSpan(ctx, "SQL: Get All", "app")
	defer span.End()

	var query strings.Builder
	query.WriteString("SELECT * FROM ")
	query.WriteString(database)
	if whereOptions != nil {
		query.WriteString(whereOptions.Where())
	}
	// TODO: remove debug
	fmt.Println(query.String())
	rows, err := r.db.QueryContext(ctx, query.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []Package
	for rows.Next() {
		var pkg Package
		if err := rows.Scan(
			&pkg.ID,
			&pkg.Name,
			&pkg.Version,
			&pkg.FormatVersion,
			&pkg.Release,
			&pkg.Prerelease,
			&pkg.KibanaVersion,
			&pkg.Categories,
			&pkg.Capabilities,
			&pkg.DiscoveryFields,
			&pkg.Type,
			&pkg.Path,
			&pkg.Data,
			&pkg.BaseData,
		); err != nil {
			return nil, err
		}
		all = append(all, pkg)
	}
	return all, nil
}

func (r *SQLiteRepository) AllFunc(ctx context.Context, database string, whereOptions WhereOptions, process func(ctx context.Context, pkg *Package) error) error {
	span, ctx := apm.StartSpan(ctx, "SQL: Get All (process each package)", "app")
	defer span.End()

	// TODO: return data or baseData column depending on the query required
	var query strings.Builder
	query.WriteString("SELECT * FROM ")
	query.WriteString(database)
	if whereOptions != nil {
		query.WriteString(whereOptions.Where())
	}
	// TODO: remove debug
	fmt.Println(query.String())
	rows, err := r.db.QueryContext(ctx, query.String())
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var pkg Package
		if err := rows.Scan(
			&pkg.ID,
			&pkg.Name,
			&pkg.Version,
			&pkg.FormatVersion,
			&pkg.Release,
			&pkg.Prerelease,
			&pkg.KibanaVersion,
			&pkg.Categories,
			&pkg.Capabilities,
			&pkg.DiscoveryFields,
			&pkg.Type,
			&pkg.Path,
			&pkg.Data,
			&pkg.BaseData,
		); err != nil {
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
	span, ctx := apm.StartSpan(ctx, "SQL: Drop", "app")
	defer span.End()
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
	Category   string
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

	if o.Filter.Category != "" {
		if sb.Len() > 0 {
			sb.WriteString(" AND ")
		}
		sb.WriteString("categories LIKE '%,")
		sb.WriteString(o.Filter.Category)
		sb.WriteString(",%'")
	}

	clause := sb.String()
	if clause == "" {
		return ""
	}
	return fmt.Sprintf(" WHERE %s", clause)
}
