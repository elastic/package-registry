// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/prometheus/client_golang/prometheus"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/internal/database"
	"github.com/elastic/package-registry/metrics"
)

// ValidationDisabled is a flag which can disable package content validation (package, data streams, assets, etc.).
var ValidationDisabled bool

const defaultMaxBulkAddBatch = 2000

// Packages is a list of packages.
type Packages []*Package

func (p Packages) Len() int      { return len(p) }
func (p Packages) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p Packages) Less(i, j int) bool {
	if p[i].Title != nil && p[j].Title != nil && *p[i].Title != *p[j].Title {
		return *p[i].Title < *p[j].Title
	}
	return p[i].Version < p[j].Version
}

// Join returns a set of packages that combines both sets. If there is already
// a package in `p1` with the same name and version that a package in `p2`, the
// latter is not added.
func (p1 Packages) Join(p2 Packages) Packages {
	for _, p := range p2 {
		if p1.contains(p) {
			continue
		}
		p1 = append(p1, p)
	}
	return p1
}

// contains returns true if `ps` contains a package with the same name and version as `p`.
func (ps Packages) contains(p *Package) bool {
	return ps.index(p) >= 0
}

// index finds if `ps` contains a package with the same name and version as `p` and
// returns its index. If it is not found, it returns -1.
func (ps Packages) index(p *Package) int {
	for i, candidate := range ps {
		if candidate.Name != p.Name {
			continue
		}
		if cv, pv := candidate.versionSemVer, p.versionSemVer; cv != nil && pv != nil {
			if !cv.Equal(pv) {
				continue
			}
		}
		if candidate.Version != p.Version {
			continue
		}

		return i
	}
	return -1
}

// GetOptions can be used to pass options to Get.
type GetOptions struct {
	// Filter to apply when querying for packages. If the filter is nil,
	// all packages are returned. This is different to a zero-object filter,
	// where experimental packages are filtered by default.
	Filter *Filter

	FullData bool
}

// FileSystemIndexer indexes packages from the filesystem.
type FileSystemIndexer struct {
	paths       []string
	packageList Packages

	// Label used for APM instrumentation.
	label string

	// Walker function used to look for files, it returns true for paths that should be indexed.
	walkerFn func(basePath, path string, info os.DirEntry) (shouldIndex bool, err error)

	// Builder to access the files of a package in this indexer.
	fsBuilder FileSystemBuilder

	logger *zap.Logger

	database database.Repository

	maxBulkAddBatch int
}

// NewFileSystemIndexer creates a new FileSystemIndexer for the given paths.
func NewFileSystemIndexer(logger *zap.Logger, dbRepository database.Repository, paths ...string) *FileSystemIndexer {
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
	return &FileSystemIndexer{
		paths:           paths,
		label:           "FileSystemIndexer",
		walkerFn:        walkerFn,
		fsBuilder:       ExtractedFileSystemBuilder,
		logger:          logger,
		database:        dbRepository,
		maxBulkAddBatch: defaultMaxBulkAddBatch,
	}
}

var ExtractedFileSystemBuilder = func(p *Package) (PackageFileSystem, error) {
	return NewExtractedPackageFileSystem(p)
}

// NewZipFileSystemIndexer creates a new ZipFileSystemIndexer for the given paths.
func NewZipFileSystemIndexer(logger *zap.Logger, dbRepository database.Repository, paths ...string) *FileSystemIndexer {
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

	return &FileSystemIndexer{
		paths:           paths,
		label:           "ZipFileSystemIndexer",
		walkerFn:        walkerFn,
		fsBuilder:       ZipFileSystemBuilder,
		logger:          logger,
		database:        dbRepository,
		maxBulkAddBatch: defaultMaxBulkAddBatch,
	}
}

var ZipFileSystemBuilder = func(p *Package) (PackageFileSystem, error) {
	return NewZipPackageFileSystem(p)
}

// Init initializes the indexer.
func (i *FileSystemIndexer) Init(ctx context.Context) (err error) {
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
func (i *FileSystemIndexer) Get(ctx context.Context, opts *GetOptions) (Packages, error) {
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
		}
		if opts.Filter.Experimental {
			options.Filter.Prerelease = true
		}
	}

	var packages Packages
	err := i.database.AllFunc(ctx, "packages", options, func(ctx context.Context, p *database.Package) error {
		var newPackage *Package
		err := func() error {
			span, _ := apm.StartSpan(ctx, "Process new package", "app")
			span.Context.SetLabel("package.path", p.Path)
			defer span.End()

			var err error
			newPackage, err = NewPackage(i.logger, p.Path, i.fsBuilder)
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
			pkgs, err := opts.Filter.Apply(ctx, Packages{newPackage})
			if err != nil {
				return err
			}
			if len(pkgs) == 0 {
				return nil
			}
		}

		packages = append(packages, newPackage)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to obtain all packages: %w", err)
	}
	if opts == nil {
		return packages, nil
	}

	if len(packages) == 0 {
		return packages, nil
	}

	// Required to filter packages if condition `all=false`
	if opts.Filter != nil {
		return opts.Filter.Apply(ctx, packages)
	}

	return packages, nil
}

type packageKey struct {
	name    string
	version string
}

func (i *FileSystemIndexer) getPackagesFromFileSystem(ctx context.Context) error {
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

func (i *FileSystemIndexer) addPackagesToDatabase(ctx context.Context, basePath string, packagesFound *map[packageKey]struct{}) error {
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
			p, err := NewPackage(i.logger, currentPackagePath, i.fsBuilder)
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
			dbPackage := database.Package{
				Name:       p.Name,
				Version:    p.Version,
				Type:       p.Type,
				Path:       currentPackagePath,
				Prerelease: p.IsPrerelease(),
				Data:       string(contents),
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
func (i *FileSystemIndexer) getPackagePaths(packagesPath string) ([]string, error) {
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

func (i *FileSystemIndexer) Close(ctx context.Context) error {
	err := i.database.Close(ctx)
	if err != nil {
		return err
	}
	return nil
}

// Filter can be used to filter a list of packages.
type Filter struct {
	AllVersions    bool
	Category       string
	Prerelease     bool
	KibanaVersion  *semver.Version
	PackageName    string
	PackageVersion string
	PackageType    string
	Capabilities   []string
	SpecMin        *semver.Version
	SpecMax        *semver.Version
	Discovery      *discoveryFilter

	// Deprecated, release tags to be removed.
	Experimental bool
}

type discoveryFilter struct {
	Fields discoveryFilterFields
}

func NewDiscoveryFilter(filter string) (*discoveryFilter, error) {
	filterType, args, found := strings.Cut(filter, ":")
	if !found {
		return nil, fmt.Errorf("could not parse filter %q", filter)
	}

	var result discoveryFilter
	switch filterType {
	case "fields":
		for _, name := range strings.Split(args, ",") {
			result.Fields = append(result.Fields, DiscoveryField{
				Name: name,
			})
		}
	default:
		return nil, fmt.Errorf("unknown discovery filter %q", filterType)
	}

	return &result, nil
}

func (f *discoveryFilter) Matches(p *Package) bool {
	if f == nil {
		return true
	}
	return f.Fields.Matches(p)
}

type discoveryFilterFields []DiscoveryField

// Matches implements matching for a collection of fields used as discovery filter.
// It matches if all fields in the package are included in the list of fields in the query.
func (fields discoveryFilterFields) Matches(p *Package) bool {
	// If the package doesn't define this filter, it doesn't match.
	if p.Discovery == nil || len(p.Discovery.Fields) == 0 {
		return false
	}

	for _, packageField := range p.Discovery.Fields {
		if !slices.Contains([]DiscoveryField(fields), packageField) {
			return false
		}
	}

	return true
}

// Apply applies the filter to the list of packages, if the filter is nil, no filtering is done.
func (f *Filter) Apply(ctx context.Context, packages Packages) (Packages, error) {
	if f == nil {
		return packages, nil
	}

	span, ctx := apm.StartSpan(ctx, "FilterPackages", "app")
	defer span.End()

	if f.Experimental {
		return f.legacyApply(ctx, packages), nil
	}

	// Checks that only the most recent version of an integration is added to the list
	var packagesList Packages
	for _, p := range packages {
		// Skip experimental packages if flag is not specified.
		if p.Release == ReleaseExperimental && !f.Prerelease {
			continue
		}

		// Skip prerelease packages by default.
		if p.IsPrerelease() && !f.Prerelease {
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

		if f.PackageType != "" && f.PackageType != p.Type {
			continue
		}

		if f.Capabilities != nil {
			if valid := p.WorksWithCapabilities(f.Capabilities); !valid {
				continue
			}
		}

		if f.Discovery != nil && !f.Discovery.Matches(p) {
			continue
		}

		if f.SpecMin != nil || f.SpecMax != nil {
			valid, err := p.HasCompatibleSpec(f.SpecMin, f.SpecMax, f.KibanaVersion)
			if err != nil {
				return nil, fmt.Errorf("can't compare spec version for %s (%s-%s): %w", p.Name, f.SpecMin, f.SpecMax, err)
			}

			if !valid {
				continue
			}
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

	// Filter by category after selecting the newer packages.
	packagesList = filterCategories(packagesList, f.Category)

	return packagesList, nil
}

// legacyApply applies the filter to the list of packages for legacy clients using `experimental=true`.
func (f *Filter) legacyApply(ctx context.Context, packages Packages) Packages {
	if f == nil {
		return packages
	}

	// Checks that only the most recent version of an integration is added to the list
	var packagesList Packages
	for _, p := range packages {
		// Skip experimental packages if flag is not specified.
		if p.Release == ReleaseExperimental && !f.Experimental {
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

		if f.PackageType != "" && f.PackageType != p.Type {
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

				// If the package in the list is newer or equal, do nothing, unless it is a prerelease.
				if current.IsPrerelease() == p.IsPrerelease() && current.IsNewerOrEqual(p) {
					continue
				}

				// If the package in the list is not a prerelease, and current is, do nothing.
				if !current.IsPrerelease() && p.IsPrerelease() {
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

	if f.AllVersions {
		packageHasNonPrerelease := make(map[string]bool)
		for _, p := range packagesList {
			if !p.IsPrerelease() {
				packageHasNonPrerelease[p.Name] = true
			}
		}

		i := 0
		for _, p := range packagesList {
			if packageHasNonPrerelease[p.Name] && p.IsPrerelease() {
				continue
			}
			packagesList[i] = p
			i++
		}

		packagesList = packagesList[:i]
	}

	// Filter by category after selecting the newer packages.
	packagesList = filterCategories(packagesList, f.Category)

	return packagesList
}

func filterCategories(packages Packages, category string) Packages {
	if category == "" {
		return packages
	}
	var result Packages
	for _, p := range packages {
		if !p.HasCategory(category) && !p.HasPolicyTemplateWithCategory(category) {
			continue
		}
		if !p.HasCategory(category) {
			p = filterPolicyTemplates(*p, category)
		}

		result = append(result, p)
	}
	return result
}

func filterPolicyTemplates(p Package, category string) *Package {
	var updatedPolicyTemplates []PolicyTemplate
	var updatedBasePolicyTemplates []BasePolicyTemplate
	for i, pt := range p.PolicyTemplates {
		if slices.Contains(pt.Categories, category) {
			updatedPolicyTemplates = append(updatedPolicyTemplates, pt)
			updatedBasePolicyTemplates = append(updatedBasePolicyTemplates, p.BasePackage.BasePolicyTemplates[i])
		}
	}
	p.PolicyTemplates = updatedPolicyTemplates
	p.BasePackage.BasePolicyTemplates = updatedBasePolicyTemplates
	return &p
}

// NameVersionFilter is a helper to initialize a Filter with the usual
// options to look per name and version along all packages indexed.
func NameVersionFilter(name, version string) GetOptions {
	return GetOptions{
		Filter: &Filter{
			Experimental:   true,
			Prerelease:     true,
			PackageName:    name,
			PackageVersion: version,
		},
	}
}
