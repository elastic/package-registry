// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/fsnotify/fsnotify"

	"github.com/prometheus/client_golang/prometheus"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/internal/workers"
	"github.com/elastic/package-registry/metrics"
)

const (
	zipFileSystemIndexerName = "ZipFileSystemIndexer"
	fileSystemIndexerName    = "FileSystemIndexer"
)

// ValidationDisabled is a flag which can disable package content validation (package, data streams, assets, etc.).
var ValidationDisabled bool

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
	// If p1 is empty, return p2 as it is.
	// This is a special case to avoid unnecessary checks.
	if len(p1) == 0 {
		return p2
	}

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

	FullData        bool
	SkipPackageData bool
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

	enablePathsWatcher bool

	m sync.RWMutex

	apmTracer *apm.Tracer
}

type FSIndexerOptions struct {
	Logger             *zap.Logger
	EnablePathsWatcher bool
	APMTracer          *apm.Tracer
}

// NewFileSystemIndexer creates a new FileSystemIndexer for the given paths.
func NewFileSystemIndexer(options FSIndexerOptions, paths ...string) *FileSystemIndexer {
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
				options.Logger.Warn("ignoring unexpected directory",
					zap.String("file.path", path))
				return false, filepath.SkipDir
			}
			return true, nil
		}
		// Unexpected file, return nil in order to continue processing sibling directories
		// Fixes an annoying problem when the .DS_Store file is left behind and the package
		// is not loading without any error information
		options.Logger.Warn("ignoring unexpected file", zap.String("file.path", path))
		return false, nil
	}
	return &FileSystemIndexer{
		paths:              paths,
		label:              fileSystemIndexerName,
		walkerFn:           walkerFn,
		fsBuilder:          ExtractedFileSystemBuilder,
		logger:             options.Logger,
		enablePathsWatcher: options.EnablePathsWatcher,
		apmTracer:          options.APMTracer,
	}
}

var ExtractedFileSystemBuilder = func(p *Package) (PackageFileSystem, error) {
	return NewExtractedPackageFileSystem(p)
}

// NewZipFileSystemIndexer creates a new ZipFileSystemIndexer for the given paths.
func NewZipFileSystemIndexer(options FSIndexerOptions, paths ...string) *FileSystemIndexer {
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
			options.Logger.Warn("ignoring invalid zip file",
				zap.String("file.path", path), zap.Error(err))
			return false, nil
		}
		defer r.Close()

		return true, nil
	}
	return &FileSystemIndexer{
		paths:              paths,
		label:              zipFileSystemIndexerName,
		walkerFn:           walkerFn,
		fsBuilder:          ZipFileSystemBuilder,
		logger:             options.Logger,
		enablePathsWatcher: options.EnablePathsWatcher,
		apmTracer:          options.APMTracer,
	}
}

var ZipFileSystemBuilder = func(p *Package) (PackageFileSystem, error) {
	return NewZipPackageFileSystem(p)
}

// Init initializes the indexer.
func (i *FileSystemIndexer) Init(ctx context.Context) (err error) {
	if err := i.updatePackageFileSystemIndex(ctx); err != nil {
		i.logger.Error("initializing package filesystem index failed",
			zap.Error(err),
			zap.String("indexer", i.label))
		return err
	}

	if i.enablePathsWatcher {
		// removing current transaction as we are starting a new one at watcher
		go i.watchPackageFileSystem(apm.ContextWithTransaction(ctx, nil))
	}
	return nil
}

func (i *FileSystemIndexer) watchPackageFileSystem(ctx context.Context) {
	// TODO: https://github.com/elastic/package-registry/issues/1488
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		i.logger.Error("failed to create fsnotify watcher", zap.String("indexer", i.label), zap.Error(err))
		return
	}
	defer watcher.Close()

	if i.apmTracer == nil {
		i.apmTracer = apm.DefaultTracer()
	}

	for _, path := range i.paths {
		if err := watcher.Add(path); err != nil {
			i.logger.Error("failed to watch path", zap.String("path", path), zap.String("indexer", i.label), zap.Error(err))
			return
		}
		i.logger.Debug("watching path for changes", zap.String("path", path), zap.String("indexer", i.label))
	}

	debouncer := time.NewTimer(0)
	debouncer.Stop()
	defer debouncer.Stop()

	for {
		select {
		case <-ctx.Done():
			i.logger.Info("stopping filesystem watcher", zap.String("indexer", i.label))
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Watching for create, write, rename and remove events
			// https://pkg.go.dev/github.com/fsnotify/fsnotify@v1.9.0#Watcher
			if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) &&
				!event.Has(fsnotify.Rename) && !event.Has(fsnotify.Remove) {
				continue
			}
			// skip events that are not relevant for this indexer
			if (i.label == zipFileSystemIndexerName && !strings.HasSuffix(event.Name, ".zip")) ||
				(i.label == fileSystemIndexerName && strings.HasSuffix(event.Name, ".zip")) {
				i.logger.Debug("skipping event at indexer", zap.String("indexer", i.label))
				continue
			}

			i.logger.Debug("filesystem change detected", zap.String("event", event.String()), zap.String("indexer", i.label))
			const debounceDelay = 1 * time.Second
			debouncer.Reset(debounceDelay)
		case <-debouncer.C:
			tx := i.apmTracer.StartTransaction("updateFSIndex", "backend.watcher")
			defer tx.End()

			ctx := apm.ContextWithTransaction(ctx, tx)
			// only when debouncer fires, we update the index
			// debouncer only fires when no new events arrive during the debounceDelay
			if err := i.updatePackageFileSystemIndex(ctx); err != nil {
				i.logger.Error("updating package filesystem index failed",
					zap.Error(err),
					zap.String("indexer", i.label))
			} else {
				i.logger.Info("package filesystem index updated",
					zap.String("indexer", i.label))
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			i.logger.Error("fsnotify watcher error", zap.Error(err), zap.String("indexer", i.label))
		}
	}
}

func (i *FileSystemIndexer) updatePackageFileSystemIndex(ctx context.Context) error {
	i.m.Lock()
	defer i.m.Unlock()

	newPackageList, err := i.getPackagesFromFileSystem(ctx)
	if err != nil {
		return err
	}
	i.packageList = newPackageList
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

	i.m.RLock()
	defer i.m.RUnlock()

	if opts == nil {
		return i.packageList, nil
	}

	if opts.Filter != nil {
		return opts.Filter.Apply(ctx, i.packageList)
	}

	return i.packageList, nil
}

func (i *FileSystemIndexer) Close(ctx context.Context) error {
	return nil
}

func (i *FileSystemIndexer) getPackagesFromFileSystem(ctx context.Context) (Packages, error) {
	span, _ := apm.StartSpan(ctx, "GetFromFileSystem", "app")
	span.Context.SetLabel("indexer", i.label)
	defer span.End()

	type packageKey struct {
		name    string
		version string
	}

	numWorkers := runtime.GOMAXPROCS(0)

	count := 0
	for _, basePath := range i.paths {
		packagePaths, err := i.getPackagePaths(basePath)
		if err != nil {
			return nil, err
		}
		count += len(packagePaths)
	}
	pList := make(Packages, count)

	taskPool := workers.NewTaskPool(numWorkers)

	i.logger.Info("Searching packages in filesystem", zap.String("indexer", i.label))
	count = 0
	for _, basePath := range i.paths {
		packagePaths, err := i.getPackagePaths(basePath)
		if err != nil {
			return nil, err
		}
		for _, p := range packagePaths {
			position := count
			path := p
			count++
			taskPool.Do(func() error {
				p, err := NewPackage(i.logger, path, i.fsBuilder)
				if err != nil {
					return fmt.Errorf("loading package failed (path: %s): %w", path, err)
				}

				func() {
					pList[position] = p

					i.logger.Debug("found package",
						zap.String("package.name", p.Name),
						zap.String("package.version", p.Version),
						zap.String("package.path", p.BasePath))

				}()
				return nil
			})
		}
	}

	if err := taskPool.Wait(); err != nil {
		return nil, err
	}

	// Remove duplicates while preserving filesystem discovery order.
	// Duplicate removal happens after initial loading to maintain the order packages
	// are discovered in the filesystem. This ensures that when the same package version
	// exists in multiple paths, we keep the version from the first path in the search order,
	// not necessarily the first one loaded by the concurrent workers.
	current := 0
	packagesFound := make(map[packageKey]struct{})
	for _, p := range pList {
		key := packageKey{name: p.Name, version: p.Version}
		if _, found := packagesFound[key]; found {
			i.logger.Debug("duplicated package",
				zap.String("package.name", p.Name),
				zap.String("package.version", p.Version),
				zap.String("package.path", p.BasePath))
			continue
		}
		packagesFound[key] = struct{}{}
		pList[current] = p
		current++
	}

	pList = pList[:current]
	i.logger.Info("Searching packages in filesystem done", zap.String("indexer", i.label), zap.Int("packages.size", len(pList)))

	return pList, nil
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
	Discovery      discoveryFilters
	AgentVersion   *semver.Version

	// Deprecated, release tags to be removed.
	Experimental bool
}

type discoveryFilters []*discoveryFilter

type discoveryFilter struct {
	Fields   discoveryFilterFields
	Datasets discoveryFilterDatasets
}

func NewDiscoveryFilter(filter string) (*discoveryFilter, error) {
	filterType, args, found := strings.Cut(filter, ":")
	if !found {
		return nil, fmt.Errorf("could not parse filter %q", filter)
	}

	var result discoveryFilter
	switch filterType {
	case "fields":
		for _, parameter := range strings.Split(args, ",") {
			result.Fields = append(result.Fields, newDiscoveryFilterField(parameter))
		}
	case "datasets":
		for _, parameter := range strings.Split(args, ",") {
			result.Datasets = append(result.Datasets, newDiscoveryFilterDataset(parameter))
		}
	default:
		return nil, fmt.Errorf("unknown discovery filter %q", filterType)
	}

	return &result, nil
}

func newDiscoveryFilterField(parameter string) DiscoveryField {
	return DiscoveryField{
		Name: parameter,
	}
}

func newDiscoveryFilterDataset(parameter string) DiscoveryDataset {
	return DiscoveryDataset{
		Name: parameter,
	}
}

func (f discoveryFilters) Matches(p *Package) bool {
	for _, filter := range f {
		if !filter.Matches(p) {
			return false
		}
	}
	return true
}

func (f *discoveryFilter) Matches(p *Package) bool {
	if f == nil {
		return true
	}
	if len(f.Fields) > 0 && !f.Fields.Matches(p) {
		return false
	}
	if len(f.Datasets) > 0 && !f.Datasets.Matches(p) {
		return false
	}
	return true
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

type discoveryFilterDatasets []DiscoveryDataset

// Matches implements matching for a collection of datasets used as discovery filter.
// It matches if at least one dataset in the package are included in the list of datasets in the query.
func (datasets discoveryFilterDatasets) Matches(p *Package) bool {
	// If the package doesn't define this filter, it doesn't match.
	if p.Discovery == nil || len(p.Discovery.Datasets) == 0 {
		return false
	}

	for _, packageDataset := range p.Discovery.Datasets {
		if slices.Contains([]DiscoveryDataset(datasets), packageDataset) {
			return true
		}
	}

	return false
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

		if f.AgentVersion != nil {
			if valid := p.HasAgentVersion(f.AgentVersion); !valid {
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
