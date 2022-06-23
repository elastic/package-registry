// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/util"
)

type cursor struct {
	Current string `json:"current"`
}

func (c *cursor) String() string {
	b, err := json.MarshalIndent(c, " ", " ")
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func loadCursor(ctx context.Context, storageClient *storage.Client, bucketName, rootStoragePath string) (*cursor, error) {
	logger := util.Logger()
	logger.Debug("load cursor file")

	rootedCursorStoragePath := joinObjectPaths(rootStoragePath, cursorStoragePath)
	objectReader, err := storageClient.Bucket(bucketName).Object(rootedCursorStoragePath).NewReader(ctx)
	if err == storage.ErrObjectNotExist {
		return nil, errors.Wrapf(err, "cursor file doesn't exist, most likely a first run (bucketName: %s, path: %s)", bucketName, rootedCursorStoragePath)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the cursor file (path: %s)", rootedCursorStoragePath)
	}
	defer objectReader.Close()

	b, err := ioutil.ReadAll(objectReader)
	if err != nil {
		return nil, errors.Wrapf(err, "ioutil.ReadAll failed")
	}

	var c cursor
	err = json.Unmarshal(b, &c)
	if err != nil {
		return nil, errors.Wrapf(err, "can't unmarshal the cursor file")
	}

	logger.Debug("loaded cursor file", zap.String("cursor", c.String()))
	return &c, nil
}
