// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"os"
	"time"
)

type PackageLocation interface {
	Stat(packagePath string) (PackageInfo, error)
	Open(packagePath string) (PackageFile, error)
}

type PackageInfo interface {
	IsDir() bool
	ModTime() time.Time
}

type localPackages struct{}

var _ PackageLocation = new(localPackages)

type localPackageInfo struct {
	fileInfo os.FileInfo
}

var _ PackageInfo = new(localPackageInfo)

func newLocalPackages() *localPackages {
	return new(localPackages)
}

func (l localPackages) Open(packagePath string) (PackageFile, error) {
	return os.Open(packagePath)
}

func (l localPackages) Stat(packagePath string) (PackageInfo, error) {
	f, err := os.Stat(packagePath)
	if err != nil {
		return nil, err
	}
	return &localPackageInfo{
		fileInfo: f,
	}, nil
}

func (lpi localPackageInfo) IsDir() bool {
	return lpi.fileInfo.IsDir()
}

func (lpi localPackageInfo) ModTime() time.Time {
	return lpi.fileInfo.ModTime()
}
