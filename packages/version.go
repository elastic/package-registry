// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

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
	SortByNameVersion(pkgsCopy)
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
