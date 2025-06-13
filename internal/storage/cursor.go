// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	"github.com/goccy/go-json"

	"go.elastic.co/apm/v2"
	"go.uber.org/zap"
)

type cursor struct {
	Current string `json:"current"`
}

func (c *cursor) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func loadCursor(ctx context.Context, logger *zap.Logger, storageClient *storage.Client, bucketName, rootStoragePath string) (*cursor, error) {
	span, ctx := apm.StartSpan(ctx, "LoadCursor", "app")
	defer span.End()

	logger.Debug("load cursor file")

	rootedCursorStoragePath := joinObjectPaths(rootStoragePath, cursorStoragePath)
	objectReader, err := storageClient.Bucket(bucketName).Object(rootedCursorStoragePath).NewReader(ctx)
	if err == storage.ErrObjectNotExist {
		return nil, fmt.Errorf("cursor file doesn't exist, most likely a first run (bucketName: %s, path: %s): %w", bucketName, rootedCursorStoragePath, err)
	}
	if err != nil {
		return nil, fmt.Errorf("can't read the cursor file (path: %s): %w", rootedCursorStoragePath, err)
	}
	defer objectReader.Close()

	var c cursor
	err = json.NewDecoder(objectReader).Decode(&c)
	if err != nil {
		return nil, fmt.Errorf("can't decode the cursor file: %w", err)
	}

	logger.Debug("loaded cursor file", zap.String("cursor", c.String()))
	return &c, nil
}
