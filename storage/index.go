// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"

	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/packages"
)

type searchIndexAll struct {
	Packages []packageIndex `json:"packages"`
}

type packageIndex struct {
	PackageManifest packages.Package `json:"package_manifest"`
}

func loadReaderSearchIndex(ctx context.Context, logger *zap.Logger, storageClient *storage.Client, bucketName, rootStoragePath string, aCursor cursor) (*storage.Reader, error) {
	span, ctx := apm.StartSpan(ctx, "LoadReaderSearchIndexAll", "app")
	defer span.End()

	indexFile := searchIndexAllFile

	logger.Debug("load search-index-all index", zap.String("index.file", indexFile))

	rootedIndexStoragePath := buildIndexStoragePath(rootStoragePath, aCursor, indexFile)
	objectReader, err := storageClient.Bucket(bucketName).Object(rootedIndexStoragePath).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't read the index file (path: %s): %w", rootedIndexStoragePath, err)
	}
	return objectReader, nil
}

func buildIndexStoragePath(rootStoragePath string, aCursor cursor, indexFile string) string {
	return joinObjectPaths(rootStoragePath, v2MetadataStoragePath, aCursor.Current, indexFile)
}
