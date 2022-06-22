// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"fmt"
	"path/filepath"
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
	storageClient              *storage.Client
	packageStorageBucketPublic string
}

type remotePackageInfo struct {
	attrs storage.ObjectAttrs
}

var _ packages.PackageInfo = new(remotePackageInfo)

func newRemotePackages(ctx context.Context, options remotePackagesOptions) (*remotePackages, error) {
	bucketName, rootStoragePath, err := extractBucketNameFromURL(options.packageStorageBucketPublic)
	if err != nil {
		return nil, fmt.Errorf("can't extract bucket name from URL (url: %s)", options.packageStorageBucketPublic)
	}
	return &remotePackages{
		ctx:             ctx,
		storageClient:   options.storageClient,
		bucketName:      bucketName,
		rootStoragePath: rootStoragePath,
	}, nil
}

func (r remotePackages) Open(packagePath string) (packages.PackageFile, error) {
	panic("not implemented yet")
}

func (r remotePackages) Stat(packagePath string) (packages.PackageInfo, error) {
	attrs, err := r.storageClient.Bucket(r.bucketName).Object(filepath.Join(r.rootStoragePath, artifactsPackagesStoragePath, packagePath)).Attrs(r.ctx)
	if err != nil {
		return nil, err
	}
	return &remotePackageInfo{
		attrs: *attrs,
	}, nil
}

func (r remotePackageInfo) IsDir() bool {
	return false // GCP bucket doesn't contain directories, we use it to store files.
}

func (r remotePackageInfo) ModTime() time.Time {
	return r.attrs.Updated
}
