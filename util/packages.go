// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"go.elastic.co/apm"
)

// PackageValidationDisabled is a flag which can disable package content validation (package, data streams, assets, etc.).
var PackageValidationDisabled bool

type Packages []Package

// FilesystemIndexer indexes packages from the filesystem.
type FilesystemIndexer struct {
	paths       []string
	packageList Packages
}

func NewFilesystemIndexer(paths []string) *FilesystemIndexer {
	return &FilesystemIndexer{
		paths: paths,
	}
}

// GetPackages returns a slice with all existing packages.
// The list is stored in memory and on the second request directly served from memory.
// This assumes changes to packages only happen on restart (unless development mode is enabled).
// Caching the packages request many file reads every time this method is called.
func (i *FilesystemIndexer) GetPackages(ctx context.Context) (Packages, error) {
	if i.packageList != nil {
		return i.packageList, nil
	}

	var err error
	i.packageList, err = getPackagesFromFilesystem(ctx, i.paths)
	if err != nil {
		return nil, errors.Wrapf(err, "reading packages from filesystem failed")
	}
	return i.packageList, nil
}

func getPackagesFromFilesystem(ctx context.Context, packagesBasePaths []string) (Packages, error) {
	span, ctx := apm.StartSpan(ctx, "GetPackagesFromFilesystem", "app")
	defer span.End()

	var pList Packages
	for _, basePath := range packagesBasePaths {
		packagePaths, err := getPackagePaths(basePath)
		if err != nil {
			return nil, err
		}

		log.Printf("Packages in %s:", basePath)
		for _, path := range packagePaths {
			p, err := NewPackage(path)
			if err != nil {
				return nil, errors.Wrapf(err, "loading package failed (path: %s)", path)
			}

			log.Printf("%-20s\t%10s\t%s", p.Name, p.Version, p.BasePath)

			pList = append(pList, *p)
		}
	}
	return pList, nil
}

// getPackagePaths returns list of available packages, one for each version.
func getPackagePaths(packagesPath string) ([]string, error) {
	var foundPaths []string
	isZip := func(name string) bool {
		// TODO: Make this safer.
		return strings.HasSuffix(name, ".zip")
	}
	err := filepath.Walk(packagesPath, func(path string, info os.FileInfo, err error) error {
		if isZip(path) {
			foundPaths = append(foundPaths, path)
			return nil
		}

		relativePath, err := filepath.Rel(packagesPath, path)
		if err != nil {
			return err
		}

		dirs := strings.Split(relativePath, string(filepath.Separator))
		if len(dirs) < 2 {
			return nil // need to go to the package version level
		}

		if info.IsDir() {
			versionDir := dirs[1]
			_, err := semver.StrictNewVersion(versionDir)
			if err != nil {
				log.Printf("warning: unexpected directory: %s, ignoring", path)
			} else {
				foundPaths = append(foundPaths, path)
			}
			return filepath.SkipDir
		}
		// Unexpected file, return nil in order to continue processing sibling directories
		// Fixes an annoying problem when the .DS_Store file is left behind and the package
		// is not loading without any error information
		log.Printf("warning: unexpected file: %s, ignoring", path)
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "listing packages failed (path: %s)", packagesPath)
	}
	return foundPaths, nil
}
