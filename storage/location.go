// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"time"

	"cloud.google.com/go/storage"

	"github.com/elastic/package-registry/packages"
)

type remotePackages struct {
	ctx           context.Context
	storageClient *storage.Client

	bucketName      string
	rootStoragePath string
}

var _ packages.PackageLocation = new(remotePackages)

type remotePackagesOptions struct {
	storageClient   *storage.Client
	storageEndpoint string
}

type remotePackageInfo struct{}

var _ packages.PackageInfo = new(remotePackageInfo)

func newRemotePackages(options remotePackagesOptions) (*remotePackages, error) {
	return &remotePackages{
		storageClient:   options.storageClient,
		bucketName:      bucketName,
		rootStoragePath: rootStoragePath,
	}, nil
}

func (r remotePackages) Open(packagePath string) (packages.PackageFile, error) {
	panic("open: not implemented yet")
}

func (r remotePackages) Stat(packagePath string) (packages.PackageInfo, error) {
	panic("stat: not implemented yet")
}

func (r remotePackageInfo) IsDir() bool {
	return false // GCP bucket doesn't contain directories, we use it to store files.
}

func (r remotePackageInfo) ModTime() time.Time {
	return r.attrs.Updated
}
