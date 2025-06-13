// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/stretchr/testify/require"
)

const fakePackageStorageBucketInternal = "fake-package-storage-internal"

var FakeIndexerOptions = IndexerOptions{
	PackageStorageBucketInternal: "gs://" + fakePackageStorageBucketInternal,
	WatchInterval:                0,
}

func RunFakeServerOnHostPort(indexPath, host string, port uint16) (*fakestorage.Server, error) {
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file %s: %w", indexPath, err)
	}

	const firstRevision = "1"
	serverObjects, _, err := prepareServerObjects(firstRevision, indexContent)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare server objects: %w", err)
	}
	return fakestorage.NewServerWithOptions(fakestorage.Options{
		InitialObjects: serverObjects,
		Host:           host,
		Port:           port,
		Scheme:         "http",
	})
}

func PrepareFakeServer(tb testing.TB, indexPath string) *fakestorage.Server {
	indexContent, err := os.ReadFile(indexPath)
	require.NoError(tb, err, "index file must be populated")

	const firstRevision = "1"
	serverObjects, numPackages, err := prepareServerObjects(firstRevision, indexContent)
	require.NoError(tb, err, "failed to prepare server objects")
	tb.Logf("Prepared %d packages with total %d server objects.", numPackages, len(serverObjects))

	return fakestorage.NewServer(serverObjects)
}

func updateFakeServer(tb testing.TB, server *fakestorage.Server, revision, indexPath string) {
	indexContent, err := os.ReadFile(indexPath)
	require.NoError(tb, err, "index file must be populated")

	serverObjects, numPackages, err := prepareServerObjects(revision, indexContent)
	require.NoError(tb, err, "failed to prepare server objects")
	tb.Logf("Prepared %d packages with total %d server objects.", numPackages, len(serverObjects))

	for _, so := range serverObjects {
		server.CreateObject(so)
	}
}

type searchIndexAll struct {
	Packages []packageIndex `json:"packages"`
}

func prepareServerObjects(revision string, indexContent []byte) ([]fakestorage.Object, int, error) {
	var index searchIndexAll
	err := json.Unmarshal(indexContent, &index)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal index content: %w", err)
	}
	if len(index.Packages) == 0 {
		return nil, 0, fmt.Errorf("index file must contain some package entries")
	}

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
	return serverObjects, len(index.Packages), nil
}
