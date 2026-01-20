// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"sort"

	"github.com/Masterminds/semver/v3"
)

// LatestPackagesVersion sorts the given package list and returns only the latest version of each package.
// The input package list is modified by the sorting process.
func LatestPackagesVersion(source Packages) (result Packages) {
	sort.Sort(byNameVersion(source))

	current := ""
	for _, p := range source {
		if p.Name == current {
			continue
		}

		current = p.Name
		result = append(result, p)
	}

	return result
}

type byNameVersion Packages

func (p byNameVersion) Len() int      { return len(p) }
func (p byNameVersion) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p byNameVersion) Less(i, j int) bool {
	if p[i].Name != p[j].Name {
		return p[i].Name < p[j].Name
	}

	// Newer versions first.
	iSemVer, _ := semver.NewVersion(p[i].Version)
	jSemVer, _ := semver.NewVersion(p[j].Version)
	if iSemVer != nil && jSemVer != nil {
		return jSemVer.LessThan(iSemVer)
	}
	return p[j].Version < p[i].Version
}

type DeprecatedPackages map[string]Deprecated

// IsDeprecated checks if the package has been deprecated and returns the deprecation notice information.
func (d DeprecatedPackages) IsDeprecated(name string) (Deprecated, bool) {
	deprecated, found := d[name]
	return deprecated, found
}

// GetLatestDeprecatedPackageVersion builds a map of deprecated notices from the latest version that has the notice.
// If a package is deprecated in version 1.2.0 with a notice, but 1.3.0 does not have the notice, the notice from 1.2.0
// will be used.
func GetLatestDeprecatedNoticeFromPackages(packages Packages) DeprecatedPackages {
	// copy packages so sorting does not affect the original slice
	var pkgsCopy = make(Packages, len(packages))
	if ok := copy(pkgsCopy, packages); ok != len(packages) {
		return nil
	}
	// sort all packages by name and version (newest first)
	sort.Sort(byNameVersion(pkgsCopy))
	// deprecated will hold the latest deprecated info per package
	deprecated := make(DeprecatedPackages, 0)

	for _, pkg := range pkgsCopy {
		// if we already have deprecated info for this package, skip
		if _, found := deprecated[pkg.Name]; found {
			continue
		}
		if pkg.IsDeprecated() {
			deprecated[pkg.Name] = *pkg.Deprecated
		}
	}
	return deprecated
}

// PropagateDeprecatedInfoToAllVersions adds deprecation information to all versions of deprecated packages.
func PropagateDeprecatedInfoToAllVersions(packageList Packages, deprecatedPackages DeprecatedPackages) {
	for _, pkg := range packageList {
		if deprecatedInfo, found := deprecatedPackages.IsDeprecated(pkg.Name); found {
			pkg.Deprecated = &deprecatedInfo
		}
	}
}
