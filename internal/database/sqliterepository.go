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

const defaultMaxBulkAddBatch = 500

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
	{"type", "TEXT NOT NULL"},
	{"path", "TEXT NOT NULL"},
	{"data", "BLOB NOT NULL"},
	{"baseData", "BLOB NOT NULL"},
}

type SQLiteRepository struct {
	db                  *sql.DB
	path                string
	maxBulkAddBatchSize int
	maxTotalArgs        int
}

var _ Repository = new(SQLiteRepository)

type FileSQLDBOptions struct {
	Path             string
	BatchSizeInserts int
}

func NewFileSQLDB(options FileSQLDBOptions) (*SQLiteRepository, error) {
	// NOTE: Even using sqlcache (with Ristretto or Redis), data column needs to be processed (Unmarshalled)
	// again for all the Get queries performed, so there is no advantage in time of using sqlcache with SQLite
	// for our use case.

	db, err := sql.Open("sqlite", options.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	dbRepo, err := newSQLiteRepository(sqlDBOptions{
		db:               db,
		batchSizeInserts: options.BatchSizeInserts,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite repository: %w", err)
	}
	if err := dbRepo.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	dbRepo.path = options.Path

	return dbRepo, nil
}

type sqlDBOptions struct {
	db               *sql.DB
	batchSizeInserts int
}

func newSQLiteRepository(options sqlDBOptions) (*SQLiteRepository, error) {
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
	batchSize := defaultMaxBulkAddBatch
	if options.batchSizeInserts > 0 {
		batchSize = options.batchSizeInserts
	}
	return &SQLiteRepository{
		db:                  options.db,
		maxBulkAddBatchSize: batchSize,
		maxTotalArgs:        batchSize * len(keys),
	}, nil
}

func (r *SQLiteRepository) File(ctx context.Context) string {
	return r.path
}

func (r *SQLiteRepository) Ping(ctx context.Context) error {
	span, ctx := apm.StartSpan(ctx, "SQL: Ping", "app")
	span.Context.SetLabel("database.path", r.File(ctx))
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
	span.Context.SetLabel("database.path", r.File(ctx))
	defer span.End()
	createQuery := strings.Builder{}
	createQuery.WriteString("CREATE TABLE IF NOT EXISTS ")
	createQuery.WriteString("packages (")
	for _, i := range keys {
		createQuery.WriteString(fmt.Sprintf("%s %s, ", i.Name, i.SQLType))
	}
	createQuery.WriteString("PRIMARY KEY (name, version, cursor));")
	if _, err := r.db.ExecContext(ctx, createQuery.String()); err != nil {
		return err
	}
	// Not required to create an index for name and version as they are already part of the primary key
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
	span.Context.SetLabel("insert.batch.size", r.maxBulkAddBatchSize)
	span.Context.SetLabel("database.path", r.File(ctx))
	defer span.End()

	if len(pkgs) == 0 {
		return nil
	}

	totalProcessed := 0
	args := make([]any, 0, r.maxTotalArgs)
	for {
		read := 0
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

		endBatch := totalProcessed + r.maxBulkAddBatchSize
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

			args = append(args,
				pkgs[i].Cursor,
				pkgs[i].Name,
				pkgs[i].Version,
				pkgs[i].FormatVersion,
				pkgs[i].Release,
				pkgs[i].Prerelease,
				pkgs[i].KibanaVersion,
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

		// reuse args slice
		args = args[:0]
	}

	return nil
}

func (r *SQLiteRepository) All(ctx context.Context, database string, whereOptions WhereOptions) ([]*Package, error) {
	span, ctx := apm.StartSpan(ctx, "SQL: Get All", "app")
	span.Context.SetLabel("database.path", r.File(ctx))
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
	span.Context.SetLabel("database.path", r.File(ctx))
	defer span.End()

	useJSONFields := whereOptions == nil || !whereOptions.SkipJSONFields()
	useBaseData := whereOptions == nil || !whereOptions.UseFullData()

	var getKeys []string
	var query strings.Builder
	query.WriteString("SELECT ")
	for _, k := range keys {
		switch {
		case !useJSONFields && (k.Name == "data" || k.Name == "baseData"):
			continue
		case k.Name == "data" && useBaseData:
			continue
		case k.Name == "baseData" && !useBaseData:
			continue
		default:
			getKeys = append(getKeys, k.Name)
		}
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
	rows, err := r.db.QueryContext(ctx, query.String(), whereArgs...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Reuse pkg variable since all fields are scanned into it
	var pkg Package
	for rows.Next() {
		columns := []any{
			&pkg.Cursor,
			&pkg.Name,
			&pkg.Version,
			&pkg.FormatVersion,
			&pkg.Release,
			&pkg.Prerelease,
			&pkg.KibanaVersion,
			&pkg.Type,
			&pkg.Path,
		}
		if useJSONFields {
			// this variable will be assigned to BaseData if useBaseData is true
			// to avoid creating a new variable, we reuse pkg.Data
			columns = append(columns, &pkg.Data)
		}
		if err := rows.Scan(columns...); err != nil {
			return err
		}
		if useBaseData {
			pkg.BaseData = pkg.Data
			pkg.Data = []byte{}
		} else {
			pkg.BaseData = []byte{}
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
	span.Context.SetLabel("database.path", r.File(ctx))
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
	// It cannot be filtered by capabilities at database level, since it would be
	// complicated using SQL logic to ensure that all the capabilities defined in the package
	// are present in the query filter.

	// It cannot be filtered by categories at database level, since
	// the category filter is applied once all the others have been processed.
	// Therefore, it must be handled at the application level.
	// https://github.com/elastic/package-registry/blob/d0337e5ac884897bfef1f03e332c259018e00535/packages/packages.go#L516-L517
}

type SQLOptions struct {
	Filter *FilterOptions

	CurrentCursor string

	IncludeFullData bool // If true, the query will return the full data field instead of the base data field
	SkipPackageData bool // If true, no need to retrieve Data nor BaseData fields
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

func (o *SQLOptions) SkipJSONFields() bool {
	if o == nil {
		return false
	}
	return o.SkipPackageData
}
