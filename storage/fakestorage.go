// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"

	internalStorage "github.com/elastic/package-registry/internal/storage"
)

var FakeIndexerOptions = IndexerOptions{
	PackageStorageBucketInternal: "gs://" + internalStorage.FakePackageStorageBucketInternal,
	WatchInterval:                0,
}

func PrepareFakeServer(tb testing.TB, indexPath string) *fakestorage.Server {
	return internalStorage.PrepareFakeServer(tb, indexPath)
}
