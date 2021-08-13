// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"archive/zip"
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

// Packages is a list of packages.
type Packages []*Package

func (p Packages) Len() int      { return len(p) }
func (p Packages) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p Packages) Less(i, j int) bool {
	if p[i].Name == p[j].Name {
		return p[i].Version < p[j].Version
	}
	return p[i].Name < p[j].Name
}

// Join returns a set of packages that combines both sets.
func (p1 Packages) Join(p2 Packages) Packages {
	// TODO: Avoid duplications?
	return append(p1, p2...)
}

// GetPackagesOptions can be used to pass options to GetPackages.
type GetPackagesOptions struct {
	// Filter to apply when querying for packages. If the filter is nil,
	// all packages are returned. This is different to a zero-object filter,
	// where internal and experimental packages are filtered by default.
	Filter *PackageFilter
}

// FileSystemIndexer indexes packages from the filesystem.
type FileSystemIndexer struct {
	paths       []string
	packageList Packages

	// Label used for APM instrumentation.
	label string

	// Walker function used to look for files.
	walkerFn func(basePath, path string, info os.FileInfo, err error) error

	// Builder to access the files of a package in this indexer.
	fsBuilder FileSystemBuilder
}

var walkerIndexFile = errors.New("file should be indexed")

// NewFileSystemIndexer creates a new FileSystemIndexer for the given paths.
func NewFileSystemIndexer(paths ...string) *FileSystemIndexer {
	walkerFn := func(basePath, path string, info os.FileInfo, err error) error {
		relativePath, err := filepath.Rel(basePath, path)
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
				return filepath.SkipDir
			}
			return walkerIndexFile
		}
		// Unexpected file, return nil in order to continue processing sibling directories
		// Fixes an annoying problem when the .DS_Store file is left behind and the package
		// is not loading without any error information
		log.Printf("warning: unexpected file: %s, ignoring", path)
		return nil
	}
	fsBuilder := func(p *Package) (PackageFileSystem, error) {
		return NewExtractedPackageFileSystem(p)
	}
	return &FileSystemIndexer{
		paths:     paths,
		label:     "FileSystemIndexer",
		walkerFn:  walkerFn,
		fsBuilder: fsBuilder,
	}
}

// NewZipFileSystemIndexer creates a new ZipFileSystemIndexer for the given paths.
func NewZipFileSystemIndexer(paths ...string) *FileSystemIndexer {
	walkerFn := func(basePath, path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".zip") {
			return nil
		}

		// Check if the file is actually a zip file.
		r, err := zip.OpenReader(path)
		if err != nil {
			log.Printf("warning: zip file cannot be opened as zip: %s, ignoring: %v", path, err)
			return nil
		}
		defer r.Close()

		return walkerIndexFile
	}
	fsBuilder := func(p *Package) (PackageFileSystem, error) {
		return NewZipPackageFileSystem(p)
	}
	return &FileSystemIndexer{
		paths:     paths,
		label:     "ZipFileSystemIndexer",
		walkerFn:  walkerFn,
		fsBuilder: fsBuilder,
	}
}

// GetPackages returns a slice with packages.
// Options can be used to filter the returned list of packages. When no options are passed
// or they don't contain any filter, no filtering is done.
// The list is stored in memory and on the second request directly served from memory.
// This assumes changes to packages only happen on restart (unless development mode is enabled).
// Caching the packages request many file reads every time this method is called.
func (i *FileSystemIndexer) GetPackages(ctx context.Context, opts *GetPackagesOptions) (Packages, error) {
	if i.packageList == nil {
		var err error
		i.packageList, err = i.getPackagesFromFileSystem(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "reading packages from filesystem failed")
		}
	}

	if opts == nil {
		return i.packageList, nil
	}

	if opts.Filter != nil {
		return opts.Filter.Apply(ctx, i.packageList), nil
	}

	return i.packageList, nil
}

func (i *FileSystemIndexer) getPackagesFromFileSystem(ctx context.Context) (Packages, error) {
	span, ctx := apm.StartSpan(ctx, "GetPackagesFromFileSystem", "app")
	span.Context.SetLabel("indexer", i.label)
	defer span.End()

	var pList Packages
	for _, basePath := range i.paths {
		packagePaths, err := i.getPackagePaths(basePath)
		if err != nil {
			return nil, err
		}

		log.Printf("Packages in %s:", basePath)
		for _, path := range packagePaths {
			p, err := NewPackage(path, i.fsBuilder)
			if err != nil {
				return nil, errors.Wrapf(err, "loading package failed (path: %s)", path)
			}

			log.Printf("%-20s\t%10s\t%s", p.Name, p.Version, p.BasePath)

			pList = append(pList, p)
		}
	}
	return pList, nil
}

// getPackagePaths returns list of available packages, one for each version.
func (i *FileSystemIndexer) getPackagePaths(packagesPath string) ([]string, error) {
	var foundPaths []string
	err := filepath.Walk(packagesPath, func(path string, info os.FileInfo, err error) error {
		err = i.walkerFn(packagesPath, path, info, err)
		if err != walkerIndexFile {
			return err
		}
		foundPaths = append(foundPaths, path)
		if info.IsDir() {
			// If a directory is being added, consider all its contents part of
			// the package and continue.
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "listing packages failed (path: %s)", packagesPath)
	}
	return foundPaths, nil
}

// PackageFilter can be used to filter a list of packages.
type PackageFilter struct {
	AllVersions    bool
	Category       string
	Experimental   bool
	Internal       bool
	KibanaVersion  *semver.Version
	PackageName    string
	PackageVersion string
}

// Apply applies the filter to the list of packages, if the filter is nil, no filtering is done.
func (f *PackageFilter) Apply(ctx context.Context, packages Packages) Packages {
	if f == nil {
		return packages
	}

	span, ctx := apm.StartSpan(ctx, "FilterPackages", "app")
	defer span.End()

	// Checks that only the most recent version of an integration is added to the list
	var packagesList Packages
	for _, p := range packages {
		// Skip internal packages by default
		if p.Internal && !f.Internal {
			continue
		}

		// Skip experimental packages if flag is not specified
		if p.Release == ReleaseExperimental && !f.Experimental {
			continue
		}

		// Filter by category first as this could heavily reduce the number of packages
		// It must happen before the version filtering as there only the newest version
		// is exposed and there could be an older package with more versions.
		if f.Category != "" && !p.HasCategory(f.Category) {
			continue
		}

		if f.KibanaVersion != nil {
			if valid := p.HasKibanaVersion(f.KibanaVersion); !valid {
				continue
			}
		}

		if f.PackageName != "" && f.PackageName != p.Name {
			continue
		}

		if f.PackageVersion != "" && f.PackageVersion != p.Version {
			continue
		}

		addPackage := true
		if !f.AllVersions {
			// Check if the version exists and if it should be added or not.
			for i, current := range packagesList {
				if current.Name != p.Name {
					continue
				}

				addPackage = false

				// If the package in the list is newer or equal, do nothing.
				if current.IsNewerOrEqual(p) {
					continue
				}

				// Otherwise replace it.
				packagesList[i] = p
			}
		}

		if addPackage {
			packagesList = append(packagesList, p)
		}
	}

	return packagesList
}

// PackageNameVersionFilter is a helper to initialize a PackageFilter with the usual
// options to look per name and version along all packages indexed.
func PackageNameVersionFilter(name, version string) GetPackagesOptions {
	return GetPackagesOptions{
		Filter: &PackageFilter{
			Experimental:   true,
			Internal:       true,
			PackageName:    name,
			PackageVersion: version,
		},
	}
}
