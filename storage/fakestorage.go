// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"encoding/json"
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

func PrepareFakeServer(tb testing.TB, indexPath string) *fakestorage.Server {
	indexContent, err := os.ReadFile(indexPath)
	require.NoError(tb, err, "index file must be populated")

	const firstRevision = "1"
	serverObjects := prepareServerObjects(tb, firstRevision, indexContent)
	return fakestorage.NewServer(serverObjects)
}

func updateFakeServer(tb testing.TB, server *fakestorage.Server, revision, indexPath string) {
	indexContent, err := os.ReadFile(indexPath)
	require.NoError(tb, err, "index file must be populated")

	serverObjects := prepareServerObjects(tb, revision, indexContent)

	for _, so := range serverObjects {
		server.CreateObject(so)
	}
}

type searchIndexAll struct {
	Packages []packageIndex `json:"packages"`
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
