// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package filesystem

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-json"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"go.elastic.co/apm/v2"

	"github.com/elastic/package-registry/categories"
	"github.com/elastic/package-registry/internal/database"
	"github.com/elastic/package-registry/metrics"
	"github.com/elastic/package-registry/packages"
)

// ValidationDisabled is a flag which can disable package content validation (package, data streams, assets, etc.).
var (
	ValidationDisabled bool
	allCategories      = categories.DefaultCategories()
)

const defaultMaxBulkAddBatch = 2000

// FileSystemSQLIndexer indexes packages from the filesystem.
type FileSystemSQLIndexer struct {
	paths []string

	// Label used for APM instrumentation.
	label string

	// Walker function used to look for files, it returns true for paths that should be indexed.
	walkerFn func(basePath, path string, info os.DirEntry) (shouldIndex bool, err error)

	// Builder to access the files of a package in this indexer.
	fsBuilder packages.FileSystemBuilder

	logger *zap.Logger

	database database.Repository

	maxBulkAddBatch int
}

// NewFileSystemSQLIndexer creates a new FileSystemIndexer for the given paths.
func NewFileSystemSQLIndexer(logger *zap.Logger, dbRepository database.Repository, paths ...string) *FileSystemSQLIndexer {
	walkerFn := func(basePath, path string, info os.DirEntry) (bool, error) {
		relativePath, err := filepath.Rel(basePath, path)
		if err != nil {
			return false, err
		}

		dirs := strings.Split(relativePath, string(filepath.Separator))
		if len(dirs) < 2 {
			return false, nil // need to go to the package version level
		}

		if info.IsDir() {
			versionDir := dirs[1]
			_, err := semver.StrictNewVersion(versionDir)
			if err != nil {
				logger.Warn("ignoring unexpected directory",
					zap.String("file.path", path))
				return false, filepath.SkipDir
			}
			return true, nil
		}
		// Unexpected file, return nil in order to continue processing sibling directories
		// Fixes an annoying problem when the .DS_Store file is left behind and the package
		// is not loading without any error information
		logger.Warn("ignoring unexpected file", zap.String("file.path", path))
		return false, nil
	}
	return &FileSystemSQLIndexer{
		paths:           paths,
		label:           "FileSystemIndexer",
		walkerFn:        walkerFn,
		fsBuilder:       ExtractedFileSystemBuilder,
		logger:          logger,
		database:        dbRepository,
		maxBulkAddBatch: defaultMaxBulkAddBatch,
	}
}

var ExtractedFileSystemBuilder = func(p *packages.Package) (packages.PackageFileSystem, error) {
	return packages.NewExtractedPackageFileSystem(p)
}

// NewZipFileSystemSQLIndexer creates a new ZipFileSystemIndexer for the given paths.
func NewZipFileSystemSQLIndexer(logger *zap.Logger, dbRepository database.Repository, paths ...string) *FileSystemSQLIndexer {
	walkerFn := func(basePath, path string, info os.DirEntry) (bool, error) {
		if info.IsDir() {
			return false, nil
		}
		if !strings.HasSuffix(path, ".zip") {
			return false, nil
		}

		// Check if the file is actually a zip file.
		r, err := zip.OpenReader(path)
		if err != nil {
			logger.Warn("ignoring invalid zip file",
				zap.String("file.path", path), zap.Error(err))
			return false, nil
		}
		defer r.Close()

		return true, nil
	}

	return &FileSystemSQLIndexer{
		paths:           paths,
		label:           "ZipFileSystemIndexer",
		walkerFn:        walkerFn,
		fsBuilder:       packages.ZipFileSystemBuilder,
		logger:          logger,
		database:        dbRepository,
		maxBulkAddBatch: defaultMaxBulkAddBatch,
	}
}

// Init initializes the indexer.
func (i *FileSystemSQLIndexer) Init(ctx context.Context) (err error) {
	err = i.getPackagesFromFileSystem(ctx)
	if err != nil {
		return fmt.Errorf("reading packages from filesystem failed: %w", err)
	}
	return nil
}

// Get returns a slice with packages.
// Options can be used to filter the returned list of packages. When no options are passed
// or they don't contain any filter, no filtering is done.
// The list is stored in memory and on the second request directly served from memory.
// This assumes changes to packages only happen on restart (unless development mode is enabled).
// Caching the packages request many file reads every time this method is called.
func (i *FileSystemSQLIndexer) Get(ctx context.Context, opts *packages.GetOptions) (packages.Packages, error) {
	start := time.Now()
	defer func() {
		metrics.IndexerGetDurationSeconds.With(prometheus.Labels{"indexer": i.label}).Observe(time.Since(start).Seconds())
	}()

	span, ctx := apm.StartSpan(ctx, "GetFileSystemIndexer", "app")
	span.Context.SetLabel("indexer", i.label)
	defer span.End()

	options := &database.SQLOptions{}
	if opts != nil && opts.Filter != nil {
		options.Filter = &database.FilterOptions{
			Type:       opts.Filter.PackageType,
			Name:       opts.Filter.PackageName,
			Version:    opts.Filter.PackageVersion,
			Prerelease: opts.Filter.Prerelease,
			Category:   opts.Filter.Category,
		}
		if opts.Filter.Experimental {
			options.Filter.Prerelease = true
		}
	}
	// Packages are always read from file system, so we don't need full data.
	options.IncludeFullData = false

	var allPackages packages.Packages
	err := i.database.AllFunc(ctx, "packages", options, func(ctx context.Context, p *database.Package) error {
		var newPackage *packages.Package
		err := func() error {
			span, _ := apm.StartSpan(ctx, "Process new package", "app")
			span.Context.SetLabel("package.path", p.Path)
			defer span.End()

			var err error
			newPackage, err = packages.NewPackage(i.logger, p.Path, i.fsBuilder)
			if err != nil {
				return fmt.Errorf("failed to parse package %s-%s (path %q): %w", p.Name, p.Version, p.Path, err)
			}
			return nil
		}()
		if err != nil {
			i.logger.Error("failed to parse package", zap.String("package.name", p.Name), zap.String("package.version", p.Version), zap.String("package.path", p.Path), zap.Error(err))
			return nil
		}

		if opts != nil && opts.Filter != nil {
			pkgs, err := opts.Filter.Apply(ctx, packages.Packages{newPackage})
			if err != nil {
				return err
			}
			if len(pkgs) == 0 {
				return nil
			}
		}

		allPackages = append(allPackages, newPackage)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to obtain all packages: %w", err)
	}
	if opts == nil {
		return allPackages, nil
	}

	if len(allPackages) == 0 {
		return allPackages, nil
	}

	// Required to filter packages if condition `all=false`
	if opts.Filter != nil {
		return opts.Filter.Apply(ctx, allPackages)
	}

	return allPackages, nil
}

type packageKey struct {
	name    string
	version string
}

func (i *FileSystemSQLIndexer) getPackagesFromFileSystem(ctx context.Context) error {
	span, ctx := apm.StartSpan(ctx, "GetFromFileSystem", "app")
	span.Context.SetLabel("indexer", i.label)
	defer span.End()

	packagesFound := make(map[packageKey]struct{})

	for _, basePath := range i.paths {
		err := i.addPackagesToDatabase(ctx, basePath, &packagesFound)
		if err != nil {
			return fmt.Errorf("adding packages to database failed (path: %s): %w", basePath, err)
		}
	}

	return nil
}

func (i *FileSystemSQLIndexer) addPackagesToDatabase(ctx context.Context, basePath string, packagesFound *map[packageKey]struct{}) error {
	packagePaths, err := i.getPackagePaths(basePath)
	if err != nil {
		return err
	}

	totalProcessed := 0
	dbPackages := make([]*database.Package, 0, i.maxBulkAddBatch)

	i.logger.Info("Searching packages in "+basePath, zap.Int("pathsNum", len(packagePaths)), zap.String("indexer", i.label))
	for {
		read := 0
		// reuse slice to avoid allocations
		dbPackages = dbPackages[:0]
		endBatch := totalProcessed + i.maxBulkAddBatch

		for j := totalProcessed; j < endBatch && j < len(packagePaths); j++ {
			currentPackagePath := packagePaths[j]
			p, err := packages.NewPackage(i.logger, currentPackagePath, i.fsBuilder)
			if err != nil {
				return fmt.Errorf("loading package failed (path: %s): %w", currentPackagePath, err)
			}

			read++

			key := packageKey{name: p.Name, version: p.Version}
			if _, found := (*packagesFound)[key]; found {
				i.logger.Debug("duplicated package",
					zap.String("package.name", p.Name),
					zap.String("package.version", p.Version),
					zap.String("package.path", p.BasePath))
				continue
			}

			(*packagesFound)[key] = struct{}{}

			i.logger.Debug("found package",
				zap.String("package.name", p.Name),
				zap.String("package.version", p.Version),
				zap.String("package.path", p.BasePath))

			// database
			contents, err := json.Marshal(p)
			if err != nil {
				return fmt.Errorf("failed to marshal package (path: %s): %w", currentPackagePath, err)
			}

			// TODO: move to packages.NewPackage() ?
			pkgCategories := []string{}
			pkgCategories = append(pkgCategories, p.Categories...)
			for _, policyTemplate := range p.PolicyTemplates {
				if len(policyTemplate.Categories) == 0 {
					continue
				}
				pkgCategories = append(pkgCategories, policyTemplate.Categories...)
			}

			for _, category := range pkgCategories {
				if _, found := allCategories[category]; !found {
					continue
				}
				if allCategories[category].Parent == nil {
					continue
				}
				pkgCategories = append(pkgCategories, allCategories[category].Parent.Name)
			}

			dbPackage := database.Package{
				Name:       p.Name,
				Version:    p.Version,
				Type:       p.Type,
				Categories: strings.Join(pkgCategories, ","),
				Path:       currentPackagePath,
				Prerelease: p.IsPrerelease(),
				Release:    p.Release,
				Data:       string(contents),
				// TODO: set these fields properly as in SQLIndexer
				KibanaVersion:   "",
				Capabilities:    "",
				DiscoveryFields: "",
			}

			dbPackages = append(dbPackages, &dbPackage)
		}
		if len(dbPackages) > 0 {
			err = i.database.BulkAdd(ctx, "packages", dbPackages)
			if err != nil {
				return fmt.Errorf("failed to create all packages (bulk operation): %w", err)
			}
		}
		totalProcessed += read
		if totalProcessed >= len(packagePaths) {
			break
		}
	}

	return nil
}

// getPackagePaths returns list of available packages, one for each version.
func (i *FileSystemSQLIndexer) getPackagePaths(packagesPath string) ([]string, error) {
	var foundPaths []string
	err := filepath.WalkDir(packagesPath, func(path string, info os.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}

		shouldIndex, err := i.walkerFn(packagesPath, path, info)
		if err != nil {
			return err
		}
		if !shouldIndex {
			return nil
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
		return nil, fmt.Errorf("listing packages failed (path: %s): %w", packagesPath, err)
	}
	return foundPaths, nil
}

func (i *FileSystemSQLIndexer) Close(ctx context.Context) error {
	err := i.database.Close(ctx)
	if err != nil {
		return err
	}
	return nil
}
