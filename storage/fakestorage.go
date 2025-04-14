// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/elastic/package-registry/internal/database"
	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/stretchr/testify/require"
)

const fakePackageStorageBucketInternal = "fake-package-storage-internal"

func newSQLDBTest() (*database.SQLiteRepository, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	dbRepo := database.NewSQLiteRepository(db)
	if err := dbRepo.Migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	return dbRepo, nil
}

func CreateFakeIndexerOptions() (IndexerOptions, error) {
	db, err := newSQLDBTest()
	if err != nil {
		return IndexerOptions{}, err
	}
	fakeIndexerOptions := IndexerOptions{
		PackageStorageBucketInternal: "gs://" + fakePackageStorageBucketInternal,
		WatchInterval:                0,
		Database:                     db,
	}
	return fakeIndexerOptions, nil

}

func PrepareFakeServer(tb testing.TB, indexPath string) *fakestorage.Server {
	indexContent, err := os.ReadFile(indexPath)
	require.NoError(tb, err, "index file must be populated")

	const firstRevision = "1"
	serverObjects := prepareServerObjects(tb, firstRevision, indexContent)
	return fakestorage.NewServer(serverObjects)
}

func updateFakeServer(t *testing.T, server *fakestorage.Server, revision, indexPath string) {
	indexContent, err := os.ReadFile(indexPath)
	require.NoError(t, err, "index file must be populated")

	serverObjects := prepareServerObjects(t, revision, indexContent)

	for _, so := range serverObjects {
		server.CreateObject(so)
	}
}

func prepareServerObjects(tb testing.TB, revision string, indexContent []byte) []fakestorage.Object {
	var index searchIndexAll
	err := json.Unmarshal(indexContent, &index)
	require.NoError(tb, err, "index file must be valid")
	require.NotEmpty(tb, index.Packages, "index file must contain some package entries")

	var serverObjects []fakestorage.Object
	// Add cursor and index file
	serverObjects = append(serverObjects, fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName: fakePackageStorageBucketInternal, Name: cursorStoragePath,
		},
		Content: []byte(`{"current":"` + revision + `"}`),
	})
	serverObjects = append(serverObjects, fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName: fakePackageStorageBucketInternal, Name: joinObjectPaths(v2MetadataStoragePath, revision, searchIndexAllFile),
		},
		Content: indexContent,
	})
	tb.Logf("Prepared %d packages with total %d server objects.", len(index.Packages), len(serverObjects))
	return serverObjects
}
