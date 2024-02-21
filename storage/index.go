// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
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
		return nil, errors.Wrapf(err, "can't read the index file (path: %s)", rootedIndexStoragePath)
	}
	return objectReader, nil
}

func loadSearchIndexAll(ctx context.Context, logger *zap.Logger, storageClient *storage.Client, bucketName, rootStoragePath string, aCursor cursor) (*searchIndexAll, error) {
	span, ctx := apm.StartSpan(ctx, "LoadSearchIndexAll", "app")
	defer span.End()

	indexFile := searchIndexAllFile

	logger.Debug("load search-index-all index", zap.String("index.file", indexFile))

	rootedIndexStoragePath := buildIndexStoragePath(rootStoragePath, aCursor, indexFile)
	objectReader, err := storageClient.Bucket(bucketName).Object(rootedIndexStoragePath).NewReader(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the index file (path: %s)", rootedIndexStoragePath)
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
	var sia searchIndexAll
	dec := json.NewDecoder(objectReader)
	for dec.More() {
		// Read everything till the "packages" key in the map.
		token, err := dec.Token()
		if err != nil {
			return nil, errors.Wrapf(err, "unexpected error while reading index file")
		}
		if key, ok := token.(string); !ok || key != "packages" {
			continue
		}

		// Read the opening array now.
		token, err = dec.Token()
		if err != nil {
			return nil, errors.Wrapf(err, "unexpected error while reading index file")
		}
		if delim, ok := token.(json.Delim); !ok || delim != '[' {
			return nil, errors.Errorf("expected opening array, found %v", token)
		}

		// Read the array of packages one by one.
		for dec.More() {
			var p packageIndex
			err = dec.Decode(&p)
			if err != nil {
				return nil, errors.Wrapf(err, "unexpected error parsing package from index file (token: %v)", token)
			}
			sia.Packages = append(sia.Packages, p)
		}

		// Read the closing array delimiter.
		token, err = dec.Token()
		if err != nil {
			return nil, errors.Wrapf(err, "unexpected error while reading index file")
		}
		if delim, ok := token.(json.Delim); !ok || delim != ']' {
			return nil, errors.Errorf("expected closing array, found %v", token)
		}
	}
	return &sia, nil
}

func buildIndexStoragePath(rootStoragePath string, aCursor cursor, indexFile string) string {
	return joinObjectPaths(rootStoragePath, v2MetadataStoragePath, aCursor.Current, indexFile)
}
