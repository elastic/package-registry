// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"fmt"
	"path/filepath"

	"cloud.google.com/go/storage"

	"github.com/elastic/package-registry/packages"
)

type RemotePackages struct {
	ctx           context.Context
	storageClient *storage.Client

	bucketName      string
	rootStoragePath string
}

type RemotePackagesOptions struct {
	StorageClient              *storage.Client
	PackageStorageBucketPublic string
}

func NewRemotePackages(ctx context.Context, options RemotePackagesOptions) (*RemotePackages, error) {
	bucketName, rootStoragePath, err := extractBucketNameFromURL(options.PackageStorageBucketPublic)
	if err != nil {
		return nil, fmt.Errorf("can't extract bucket name from URL (url: %s)", options.PackageStorageBucketPublic)
	}
	return &RemotePackages{
		ctx:             ctx,
		storageClient:   options.StorageClient,
		bucketName:      bucketName,
		rootStoragePath: rootStoragePath,
	}, nil
}

func (r RemotePackages) Stat(packagePath string) (packages.PackageInfo, error) {
	_, err := r.storageClient.Bucket(r.bucketName).Object(filepath.Join(r.rootStoragePath, artifactsPackagesStoragePath, packagePath)).Attrs(r.ctx)
	if err != nil {
		return nil, err
	}
	return NewRemotePackageInfo(), nil
}

var _ packages.PackageLocation = new(RemotePackages)

type RemotePackageInfo struct{}

func NewRemotePackageInfo() *RemotePackageInfo {
	return new(RemotePackageInfo)
}

func (r RemotePackageInfo) IsDir() bool {
	return false // GCP bucket doesn't contain directories, we use it to store files.
}

var _ packages.PackageInfo = new(RemotePackageInfo)
