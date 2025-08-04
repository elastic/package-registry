// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package database

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // Import the SQLite driver
)

type MemorySQLDBOptions struct {
	Path             string
	BatchSizeInserts int
}

func NewMemorySQLDB(options MemorySQLDBOptions) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	dbRepo, err := newSQLiteRepository(sqlDBOptions{db: db, batchSizeInserts: options.BatchSizeInserts})
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite repository: %w", err)
	}

	if err := dbRepo.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	dbRepo.path = "memory-" + options.Path
	return dbRepo, nil
}
