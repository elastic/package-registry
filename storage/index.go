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

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/util"
)

type searchIndexAll struct {
	Packages []packageIndex `json:"packages"`
}

type packageIndex struct {
	PackageManifest packages.Package `json:"package_manifest"`
}

func loadSearchIndexAll(ctx context.Context, storageClient *storage.Client, bucketName, rootStoragePath string, aCursor cursor) (*searchIndexAll, error) {
	indexFile := searchIndexAllFile

	logger := util.Logger()
	logger.Debug("load search-index-all index", zap.String("index.file", indexFile))

	content, err := loadIndexContent(ctx, storageClient, indexFile, bucketName, rootStoragePath, aCursor)
	if err != nil {
		return nil, errors.Wrap(err, "can't load search-index-all content")
	}

	var sia searchIndexAll
	if content == nil {
		return &sia, nil
	}

	err = json.Unmarshal(content, &sia)
	if err != nil {
		return nil, errors.Wrap(err, "can't unmarshal search-index-all")
	}
	return &sia, nil
}

func loadIndexContent(ctx context.Context, storageClient *storage.Client, indexFile, bucketName, rootStoragePath string, aCursor cursor) ([]byte, error) {
	logger := util.Logger()
	logger.Debug("load index content", zap.String("index.file", indexFile))

	rootedIndexStoragePath := buildIndexStoragePath(rootStoragePath, aCursor, indexFile)
	objectReader, err := storageClient.Bucket(bucketName).Object(rootedIndexStoragePath).NewReader(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the index file (path: %s)", rootedIndexStoragePath)
	}
	defer objectReader.Close()

	b, err := ioutil.ReadAll(objectReader)
	if err != nil {
		return nil, errors.Wrapf(err, "ioutil.ReadAll failed")
	}

	return b, nil
}

func buildIndexStoragePath(rootStoragePath string, aCursor cursor, indexFile string) string {
	return joinObjectPaths(rootStoragePath, v2MetadataStoragePath, aCursor.Current, indexFile)
}

func transformSearchIndexAllToPackages(sia searchIndexAll) (packages.Packages, error) {
	var transformedPackages packages.Packages
	for i := range sia.Packages {
		transformedPackages = append(transformedPackages, &sia.Packages[i].PackageManifest)
	}
	return transformedPackages, nil
}
