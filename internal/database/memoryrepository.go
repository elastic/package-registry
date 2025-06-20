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

func NewMemorySQLDB(path string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	dbRepo, err := newSQLiteRepository(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite repository: %w", err)
	}

	if err := dbRepo.Migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	dbRepo.path = "memory-" + path
	return dbRepo, nil
}
