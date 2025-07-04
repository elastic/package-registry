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

	_ "modernc.org/sqlite" // Import the SQLite driver

	"go.elastic.co/apm/v2"
)

var (
	ErrDuplicate    = errors.New("record already exists")
	ErrNotExists    = errors.New("row not exists")
	ErrUpdateFailed = errors.New("update failed")
	ErrDeleteFailed = errors.New("delete failed")
)

type keyDefinition struct {
	Name    string
	SQLType string
}

var keys = []keyDefinition{
	{"cursor", "TEXT NOT NULL"},
	{"name", "TEXT NOT NULL"},
	{"version", "TEXT NOT NULL"},
	{"formatVersion", "TEXT NOT NULL"},
	{"release", "TEXT NOT NULL"},
	{"prerelease", "INTEGER NOT NULL"},
	{"kibanaVersion", "TEXT NOT NULL"},
	{"categories", "TEXT NOT NULL"},
	{"capabilities", "TEXT NOT NULL"},
	{"discoveryFields", "TEXT NOT NULL"},
	{"type", "TEXT NOT NULL"},
	{"path", "TEXT NOT NULL"},
	{"data", "TEXT NOT NULL"},
	{"baseData", "TEXT NOT NULL"},
}

const defaultMaxBulkAddBatch = 2000

type SQLiteRepository struct {
	db              *sql.DB
	path            string
	maxBulkAddBatch int
	numberFields    int
}

var _ Repository = new(SQLiteRepository)

func NewFileSQLDB(path string) (*SQLiteRepository, error) {
	// NOTE: Even using sqlcache (with Ristretto or Redis), data column needs to be processed (Unmarshalled)
	// again for all the Get queries performed, so there is no advantage in time of using sqlcache with SQLite
	// for our use case.

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	dbRepo, err := newSQLiteRepository(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite repository: %w", err)
	}
	if err := dbRepo.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	dbRepo.path = path

	return dbRepo, nil
}

func newSQLiteRepository(db *sql.DB) (*SQLiteRepository, error) {
	// https://www.sqlite.org/pragma.html#pragma_journal_mode
	// Not observed any performance difference with WAL mode, so keeping it as DELETE mode for now.
	// if _, err = db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
	// 	return nil, fmt.Errorf("failed to update journal_mode: %w", err)
	// }

	// // https://www.sqlite.org/pragma.html#pragma_synchronous
	// // Default is FULL, which is the safest mode, but it can be slow.
	// if _, err = db.Exec("PRAGMA synchronous = NORMAL;"); err != nil {
	// 	return nil, fmt.Errorf("failed to update synchronous: %w", err)
	// }

	// https://www.sqlite.org/pragma.html#pragma_busy_timeout
	// Setting busy_timeout to 5000ms (5 seconds) as the time to wait for a lock to go away
	// before returning an error
	// if _, err = db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
	// 	return nil, fmt.Errorf("failed to update busy_timeout: %w", err)
	// }

	// https://www.sqlite.org/pragma.html#pragma_cache_size
	// By default, SQLite uses a 2MB cache size. We can increase it to 10MB.
	// if _, err := db.Exec("PRAGMA cache_size = -10000;"); err != nil {
	// 	return nil, fmt.Errorf("failed to update cache_size: %w", err)
	// }
	return &SQLiteRepository{
		db:              db,
		maxBulkAddBatch: defaultMaxBulkAddBatch,
		numberFields:    len(keys),
	}, nil
}

func (r *SQLiteRepository) File(ctx context.Context) string {
	return r.path
}

func (r *SQLiteRepository) Ping(ctx context.Context) error {
	span, ctx := apm.StartSpan(ctx, "SQL: Ping", "app")
	defer span.End()
	if r.db == nil {
		return errors.New("database is not initialized")
	}
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) Initialize(ctx context.Context) error {
	span, ctx := apm.StartSpan(ctx, "SQL: Initialize", "app")
	defer span.End()
	createQuery := strings.Builder{}
	createQuery.WriteString("CREATE TABLE IF NOT EXISTS ")
	createQuery.WriteString("packages (")
	for _, i := range keys {
		createQuery.WriteString(fmt.Sprintf("%s %s, ", i.Name, i.SQLType))
	}
	createQuery.WriteString("PRIMARY KEY (cursor, name, version));")
	if _, err := r.db.ExecContext(ctx, createQuery.String()); err != nil {
		return err
	}
	// Not required to create an index for name and version as they are already part of the primary key
	// NOt required to create an index for categories column, it is not used in the queries. Example:
	//  > "EXPLAIN QUERY PLAN SELECT name, version FROM packages WHERE categories LIKE '%,observability,%';"
	// QUERY PLAN
	// `--SCAN packages
	query := `
	CREATE INDEX IF NOT EXISTS idx_prerelease ON packages (prerelease);
	CREATE INDEX IF NOT EXISTS idx_type ON packages (type);
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
		sb.WriteString("(")
		for i, k := range keys {
			sb.WriteString(k.Name)
			if i < len(keys)-1 {
				sb.WriteString(", ")
			}
		}
		sb.WriteString(") values")

		endBatch := totalProcessed + r.maxBulkAddBatch
		for i := totalProcessed; i < endBatch && i < len(pkgs); i++ {
			sb.WriteString("(")
			for j := range keys {
				sb.WriteString("?")
				if j < len(keys)-1 {
					sb.WriteString(", ")
				}
			}
			sb.WriteString(")")

			if i < endBatch-1 && i < len(pkgs)-1 {
				sb.WriteString(",")
			}

			// Add commas to make it easier to search for these fields
			// in the SQL query
			categories := addCommasToString(pkgs[i].Categories)
			capabilities := addCommasToString(pkgs[i].Capabilities)
			discoveryFields := addCommasToString(pkgs[i].DiscoveryFields)

			args = append(args,
				pkgs[i].Cursor,
				pkgs[i].Name,
				pkgs[i].Version,
				pkgs[i].FormatVersion,
				pkgs[i].Release,
				pkgs[i].Prerelease,
				pkgs[i].KibanaVersion,
				categories,
				capabilities,
				discoveryFields,
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

func addCommasToString(s string) string {
	// Adding commas at the beginning and end of the string ensures that there is no
	// special case for the first and last elements of comma separated list when setting
	// the Where clause of the SQL query.
	// All elements can be searched as `column LIKE '%,element,%'`
	// Example: `categories LIKE '%,observability,%'`
	// And the categoories value could be like:
	// - `,observability,security,`
	// - `,observability,`
	// - `,security,observability,`
	if s != "" {
		s = fmt.Sprintf(",%s,", s)
	}
	return s
}

func (r *SQLiteRepository) All(ctx context.Context, database string, whereOptions WhereOptions) ([]*Package, error) {
	span, ctx := apm.StartSpan(ctx, "SQL: Get All", "app")
	defer span.End()

	var all []*Package
	r.AllFunc(ctx, database, whereOptions, func(ctx context.Context, pkg *Package) error {
		all = append(all, pkg)
		return nil
	})

	return all, nil
}

func (r *SQLiteRepository) AllFunc(ctx context.Context, database string, whereOptions WhereOptions, process func(ctx context.Context, pkg *Package) error) error {
	span, ctx := apm.StartSpan(ctx, "SQL: Get All (process each package)", "app")
	defer span.End()

	useBaseData := whereOptions == nil || !whereOptions.UseFullData()

	var getKeys []string
	var query strings.Builder
	query.WriteString("SELECT ")
	for _, k := range keys {
		if k.Name == "data" && useBaseData {
			continue
		}
		if k.Name == "baseData" && !useBaseData {
			continue
		}
		getKeys = append(getKeys, k.Name)
	}
	query.WriteString(strings.Join(getKeys, ", "))
	query.WriteString(" FROM ")
	query.WriteString(database)
	var whereArgs []any
	if whereOptions != nil {
		var clause string
		clause, whereArgs = whereOptions.Where()
		query.WriteString(clause)
	}
	// TODO: remove debug
	// fmt.Println("Query:", query.String())
	rows, err := r.db.QueryContext(ctx, query.String(), whereArgs...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Reuse pkg variable since all fields are scanned into it
	var pkg Package
	for rows.Next() {
		if err := rows.Scan(
			&pkg.Cursor,
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
			&pkg.Data, // this variable will be assigned to BaseData if useBaseData is true
			// to avoid creting a new variable, we reuse pkg.Data
		); err != nil {
			return err
		}
		if useBaseData {
			pkg.BaseData = pkg.Data
			pkg.Data = ""
		} else {
			pkg.BaseData = ""
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
	Type         string
	Name         string
	Version      string
	Prerelease   bool
	Category     string
	Capabilities []string
}

type SQLOptions struct {
	Filter *FilterOptions

	CurrentCursor string

	IncludeFullData bool // If true, the query will return the full data field instead of the base data field
}

func (o *SQLOptions) Where() (string, []any) {
	if o == nil {
		return "", nil
	}
	var sb strings.Builder
	var args []any
	// Always filter by cursor
	if o.CurrentCursor != "" {
		sb.WriteString("cursor = ?")
		args = append(args, o.CurrentCursor)
	}

	if o.Filter == nil {
		if sb.Len() == 0 {
			return "", nil
		}
		return fmt.Sprintf(" WHERE %s", sb.String()), args
	}

	if o.Filter.Type != "" {
		if sb.Len() > 0 {
			sb.WriteString(" AND ")
		}
		sb.WriteString("type = ?")
		args = append(args, o.Filter.Type)
	}

	if o.Filter.Name != "" {
		if sb.Len() > 0 {
			sb.WriteString(" AND ")
		}
		sb.WriteString("name = ?")
		args = append(args, o.Filter.Name)
	}

	if o.Filter.Version != "" {
		if sb.Len() > 0 {
			sb.WriteString(" AND ")
		}
		sb.WriteString("version = ?")
		args = append(args, o.Filter.Version)
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
		sb.WriteString("categories LIKE ?")
		args = append(args, fmt.Sprintf("%%,%s,%%", o.Filter.Category))
	}

	if len(o.Filter.Capabilities) > 0 {
		if sb.Len() > 0 {
			sb.WriteString(" AND ")
		}
		// If capabilities column value is empty, those packages are not filtered out
		sb.WriteString("( capabilities == '' OR (")
		for i, capability := range o.Filter.Capabilities {
			sb.WriteString("capabilities LIKE ?")
			args = append(args, fmt.Sprintf("%%,%s,%%", capability))
			if i < len(o.Filter.Capabilities)-1 {
				sb.WriteString(" AND ")
			}
		}
		sb.WriteString(") )")
	}

	if sb.String() == "" {
		return "", nil
	}
	return fmt.Sprintf(" WHERE %s", sb.String()), args
}

func (o *SQLOptions) UseFullData() bool {
	if o == nil {
		return false
	}
	return o.IncludeFullData
}
