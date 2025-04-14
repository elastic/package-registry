// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package database

import (
	"context"
	"database/sql"
	"fmt"
)

func NewMemorySQLDB() (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	dbRepo := NewSQLiteRepository(db)
	if err := dbRepo.Migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	return dbRepo, nil
}
