// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/storage"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/packages"
)

// packageIndexEntry mirrors the package_manifest wrapper used by package-storage-infra
// when writing added/updated entries to the delta file.
type packageIndexEntry struct {
	PackageManifest packages.Package `json:"package_manifest"`
}

// SearchIndexDelta holds the changes between two consecutive index revisions.
// Field names match the JSON produced by package-storage-infra:
//   - added/updated entries are wrapped in {"package_manifest": {...}}
//   - removed entries use the key "removed" (not "deleted")
type SearchIndexDelta struct {
	Added   []packageIndexEntry `json:"added"`
	Updated []packageIndexEntry `json:"updated"`
	Removed []removedPackageRef `json:"removed"`
}

// removedPackageRef identifies a package to remove by name and version.
type removedPackageRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func loadSearchIndexDelta(ctx context.Context, logger *zap.Logger, storageClient *storage.Client, bucketName, rootStoragePath string, aCursor cursor) (*SearchIndexDelta, error) {
	span, ctx := apm.StartSpan(ctx, "LoadSearchIndexDelta", "app")
	defer span.End()

	logger.Debug("load search-index-delta", zap.String("delta.file", searchIndexDeltaFile))

	rootedPath := buildIndexStoragePath(rootStoragePath, aCursor, searchIndexDeltaFile)
	objectReader, err := storageClient.Bucket(bucketName).Object(rootedPath).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't read the delta file (path: %s): %w", rootedPath, err)
	}
	defer objectReader.Close()

	var delta SearchIndexDelta
	if err := json.NewDecoder(objectReader).Decode(&delta); err != nil {
		return nil, fmt.Errorf("can't decode the delta file: %w", err)
	}
	return &delta, nil
}

// LoadSearchIndexDelta reads and parses the delta file for the given cursor value.
func LoadSearchIndexDelta(ctx context.Context, logger *zap.Logger, storageClient *storage.Client, storageBucketInternal, cursorValue string) (*SearchIndexDelta, error) {
	bucketName, rootStoragePath, err := extractBucketNameFromURL(storageBucketInternal)
	if err != nil {
		return nil, fmt.Errorf("can't extract bucket name from URL: %w", err)
	}
	return loadSearchIndexDelta(ctx, logger, storageClient, bucketName, rootStoragePath, cursor{Current: cursorValue})
}
