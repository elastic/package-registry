// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"os"
)

type PackageLocation interface {
	Stat(packagePath string) (PackageInfo, error)
}

type PackageInfo interface {
	IsDir() bool
}

type LocalPackages struct{}

func NewLocalPackages() *LocalPackages {
	return new(LocalPackages)
}

func (l LocalPackages) Stat(packagePath string) (PackageInfo, error) {
	f, err := os.Stat(packagePath)
	if err != nil {
		return nil, err
	}
	return &localPackageInfo{
		fileInfo: f,
	}, nil
}

var _ PackageLocation = new(LocalPackages)

type localPackageInfo struct {
	fileInfo os.FileInfo
}

func (lpi localPackageInfo) IsDir() bool {
	return lpi.fileInfo.IsDir()
}

var _ PackageInfo = new(localPackageInfo)
