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

type packageIndex struct {
	PackageManifest *packages.Package `json:"package_manifest"`
}

func loadSearchIndexAll(ctx context.Context, logger *zap.Logger, storageClient *storage.Client, bucketName, rootStoragePath string, aCursor cursor) (*packages.Packages, error) {
	span, ctx := apm.StartSpan(ctx, "LoadSearchIndexAll", "app")
	span.Context.SetLabel("load.method", "full")
	defer span.End()

	indexFile := searchIndexAllFile

	logger.Debug("load search-index-all index", zap.String("index.file", indexFile))

	rootedIndexStoragePath := buildIndexStoragePath(rootStoragePath, aCursor, indexFile)
	objectReader, err := storageClient.Bucket(bucketName).Object(rootedIndexStoragePath).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't read the index file (path: %s): %w", rootedIndexStoragePath, err)
	}
	defer objectReader.Close()

	// Using a decoder here as tokenizer to parse the list of packages as a stream
	// instead of needing the whole document in memory at the same time. This helps
	// reducing memory usage.
	// Using `Unmarshal(doc, &sia)` would require to read the whole document.
	// Using `dec.Decode(&sia)` would also make the decoder to keep the whole document
	// in memory.
	// `jsoniter` seemed to be slightly faster, but to use more memory for our use case,
	// and we are looking to optimize for memory use.
	var packages packages.Packages
	dec := json.NewDecoder(objectReader)
	for dec.More() {
		// Read everything till the "packages" key in the map.
		token, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("unexpected error while reading index file: %w", err)
		}
		if key, ok := token.(string); !ok || key != "packages" {
			continue
		}

		// Read the opening array now.
		token, err = dec.Token()
		if err != nil {
			return nil, fmt.Errorf("unexpected error while reading index file: %w", err)
		}
		if delim, ok := token.(json.Delim); !ok || delim != '[' {
			return nil, fmt.Errorf("expected opening array, found %v", token)
		}

		// Read the array of packages one by one.
		for dec.More() {
			var p packageIndex
			err = dec.Decode(&p)
			if err != nil {
				return nil, fmt.Errorf("unexpected error parsing package from index file (token: %v): %w", token, err)
			}
			packages = append(packages, p.PackageManifest)
		}

		// Read the closing array delimiter.
		token, err = dec.Token()
		if err != nil {
			return nil, fmt.Errorf("unexpected error while reading index file: %w", err)
		}
		if delim, ok := token.(json.Delim); !ok || delim != ']' {
			return nil, fmt.Errorf("expected closing array, found %v", token)
		}
	}
	return &packages, nil
}

func buildIndexStoragePath(rootStoragePath string, aCursor cursor, indexFile string) string {
	return joinObjectPaths(rootStoragePath, v2MetadataStoragePath, aCursor.Current, indexFile)
}

func LoadPackagesAndCursorFromIndex(ctx context.Context, logger *zap.Logger, storageClient *storage.Client, storageBucketInternal, currentCursor string) (*packages.Packages, string, error) {
	bucketName, rootStoragePath, err := extractBucketNameFromURL(storageBucketInternal)
	if err != nil {
		return nil, "", fmt.Errorf("can't extract bucket name from URL (url: %s): %w", storageBucketInternal, err)
	}

	storageCursor, err := loadCursor(ctx, logger, storageClient, bucketName, rootStoragePath)
	if err != nil {
		return nil, "", fmt.Errorf("can't load latest cursor: %w", err)
	}

	if storageCursor.Current == currentCursor {
		logger.Info("cursor is up-to-date", zap.String("cursor.current", currentCursor))
		return nil, currentCursor, nil
	}
	logger.Info("cursor will be updated", zap.String("cursor.current", currentCursor), zap.String("cursor.next", storageCursor.Current))

	anIndex, err := loadSearchIndexAll(ctx, logger, storageClient, bucketName, rootStoragePath, *storageCursor)
	if err != nil {
		return nil, "", fmt.Errorf("can't load the search-index-all index content: %w", err)
	}
	return anIndex, storageCursor.Current, nil
}

func loadSearchIndexAllBatches(ctx context.Context, logger *zap.Logger, storageClient *storage.Client, bucketName, rootStoragePath string, aCursor cursor, batchSize int,
	process func(context.Context, packages.Packages, string) error) error {
	span, ctx := apm.StartSpan(ctx, "LoadSearchIndexAll", "app")
	span.Context.SetLabel("load.method", "batches")
	span.Context.SetLabel("load.batch.size", batchSize)
	defer span.End()

	indexFile := searchIndexAllFile

	logger.Debug("load search-index-all index", zap.String("index.file", indexFile))

	rootedIndexStoragePath := buildIndexStoragePath(rootStoragePath, aCursor, indexFile)
	objectReader, err := storageClient.Bucket(bucketName).Object(rootedIndexStoragePath).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("can't read the index file (path: %s): %w", rootedIndexStoragePath, err)
	}
	defer objectReader.Close()

	// Using a decoder here as tokenizer to parse the list of packages as a stream
	// instead of needing the whole document in memory at the same time. This helps
	// reducing memory usage.
	// Using `Unmarshal(doc, &sia)` would require to read the whole document.
	// Using `dec.Decode(&sia)` would also make the decoder to keep the whole document
	// in memory.
	// `jsoniter` seemed to be slightly faster, but to use more memory for our use case,
	// and we are looking to optimize for memory use.
	dec := json.NewDecoder(objectReader)
	count := 0
	pkgs := make(packages.Packages, 0, batchSize)
	for dec.More() {
		// Read everything till the "packages" key in the map.
		token, err := dec.Token()
		if err != nil {
			return fmt.Errorf("unexpected error while reading index file: %w", err)
		}
		if key, ok := token.(string); !ok || key != "packages" {
			continue
		}

		// Read the opening array now.
		token, err = dec.Token()
		if err != nil {
			return fmt.Errorf("unexpected error while reading index file: %w", err)
		}
		if delim, ok := token.(json.Delim); !ok || delim != '[' {
			return fmt.Errorf("expected opening array, found %v", token)
		}

		// Read the array of packages one by one.
		for dec.More() {
			var p packageIndex
			err = dec.Decode(&p)
			if err != nil {
				return fmt.Errorf("unexpected error parsing package from index file (token: %v): %w", token, err)
			}
			pkgs = append(pkgs, p.PackageManifest)
			count++

			if count >= batchSize {
				err = process(ctx, pkgs, aCursor.Current)
				if err != nil {
					return fmt.Errorf("error processing batch of packages: %w", err)
				}
				count = 0
				pkgs = pkgs[:0] // Reset the slice to reuse the memory
			}
		}

		// Read the closing array delimiter.
		token, err = dec.Token()
		if err != nil {
			return fmt.Errorf("unexpected error while reading index file: %w", err)
		}
		if delim, ok := token.(json.Delim); !ok || delim != ']' {
			return fmt.Errorf("expected closing array, found %v", token)
		}
	}
	if len(pkgs) > 0 {
		err = process(ctx, pkgs, aCursor.Current)
		if err != nil {
			return fmt.Errorf("error processing final batch of packages: %w", err)
		}
	}
	return nil
}

func LoadPackagesAndCursorFromIndexBatches(ctx context.Context, logger *zap.Logger, storageClient *storage.Client, storageBucketInternal, currentCursor string, batchSize int,
	process func(context.Context, packages.Packages, string) error) (string, error) {
	bucketName, rootStoragePath, err := extractBucketNameFromURL(storageBucketInternal)
	if err != nil {
		return "", fmt.Errorf("can't extract bucket name from URL (url: %s): %w", storageBucketInternal, err)
	}

	storageCursor, err := loadCursor(ctx, logger, storageClient, bucketName, rootStoragePath)
	if err != nil {
		return "", fmt.Errorf("can't load latest cursor: %w", err)
	}

	if storageCursor.Current == currentCursor {
		logger.Info("cursor is up-to-date", zap.String("cursor.current", currentCursor))
		return currentCursor, nil
	}
	logger.Info("cursor will be updated", zap.String("cursor.current", currentCursor), zap.String("cursor.next", storageCursor.Current))

	err = loadSearchIndexAllBatches(ctx, logger, storageClient, bucketName, rootStoragePath, *storageCursor, batchSize, process)
	if err != nil {
		return "", fmt.Errorf("can't load the search-index-all index content: %w", err)
	}
	return storageCursor.Current, nil
}
