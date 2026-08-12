// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// listCursorsBetween returns all timestamp folder names in v2/metadata/ that are
// strictly greater than since and less than or equal to until, in ascending order.
// Cursor values must be lexicographically comparable in chronological order (e.g.
// zero-padded or fixed-length strings such as Unix epoch seconds). Simple unpadded
// integer strings break ordering once they reach a second digit ("9" > "10").
func listCursorsBetween(ctx context.Context, storageClient *storage.Client, bucketName, rootStoragePath, since, until string) ([]string, error) {
	prefix := joinObjectPaths(rootStoragePath, v2MetadataStoragePath) + "/"

	query := &storage.Query{
		Prefix:    prefix,
		Delimiter: "/",
	}

	it := storageClient.Bucket(bucketName).Objects(ctx, query)

	var timestamps []string
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error listing cursor folders: %w", err)
		}
		if attrs.Prefix == "" {
			continue
		}
		token := strings.TrimSuffix(strings.TrimPrefix(attrs.Prefix, prefix), "/")
		if token > since && token <= until {
			timestamps = append(timestamps, token)
		}
	}

	sort.Strings(timestamps)
	return timestamps, nil
}

// ListCursorsBetween returns all timestamp folder names in v2/metadata/ strictly
// greater than since and less than or equal to until, in ascending order.
func ListCursorsBetween(ctx context.Context, storageClient *storage.Client, storageBucketInternal, since, until string) ([]string, error) {
	bucketName, rootStoragePath, err := extractBucketNameFromURL(storageBucketInternal)
	if err != nil {
		return nil, fmt.Errorf("can't extract bucket name from URL: %w", err)
	}
	return listCursorsBetween(ctx, storageClient, bucketName, rootStoragePath, since, until)
}
