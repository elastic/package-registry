// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package database

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3" // Import the SQLite driver
)

type MemorySQLDBOptions struct {
	Path string
}

func NewMemorySQLDB(options MemorySQLDBOptions) (*SQLiteRepository, error) {
	if !CGOEnabled {
		return nil, fmt.Errorf("cgo is not enabled, cannot create in-memory SQLite database")
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	dbRepo, err := newSQLiteRepository(sqlDBOptions{db: db})
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite repository: %w", err)
	}

	if err := dbRepo.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	dbRepo.path = "memory-" + options.Path
	return dbRepo, nil
}
