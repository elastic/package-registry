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

func GetLatestPackageVersionDeprecated(packages Packages) DeprecatedPackages {
	// copy packages so LatestPackagesVersion doesn't modify the original slice
	var pkgsCopy = make(Packages, len(packages))
	if ok := copy(pkgsCopy, packages); ok != len(packages) {
		return nil
	}
	// Build the deprecated packages map from the latest package versions.
	deprecated := make(DeprecatedPackages)
	for _, pkg := range LatestPackagesVersion(pkgsCopy) {
		if pkg.IsDeprecated() {
			deprecated[pkg.Name] = *pkg.Deprecated
		}
	}
	return deprecated
}

// PropagateDeprecatedInfoToAllVersions adds deprecation information to all versions of deprecated packages.
func PropagateDeprecatedInfoToAllVersions(packageList Packages, deprecatedPackages DeprecatedPackages) Packages {
	packagesWithDeprecatedInfo := make(Packages, len(packageList))
	for idx, pkg := range packageList {
		// Create a copy of the package to avoid modifying the original one.
		pkgCopy := *pkg
		if deprecatedInfo, found := deprecatedPackages.IsDeprecated(pkg.Name); found {
			pkgCopy.Deprecated = &deprecatedInfo
		}
		packagesWithDeprecatedInfo[idx] = &pkgCopy
	}
	return packagesWithDeprecatedInfo
}
