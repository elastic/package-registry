// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import "github.com/Masterminds/semver/v3"

type deprecatedMeta struct {
	deprecated *Deprecated
	version    *semver.Version
}

type DeprecatedPackages map[string]deprecatedMeta

func (d DeprecatedPackages) Deprecated(name string) (*Deprecated, bool) {
	meta, found := d[name]
	if !found {
		return nil, false
	}
	return meta.deprecated, true
}

// UpdateLatestDeprecatedPackagesMapByName updates a map of the latest deprecated packages by name.
// It ensures that for each package name, only the deprecation info of the latest version is stored.
func UpdateLatestDeprecatedPackagesMapByName(input Packages, deprecatedPackages *DeprecatedPackages) {
	if *deprecatedPackages == nil {
		*deprecatedPackages = make(DeprecatedPackages)
	}
	for _, pkg := range input {
		if pkg.BasePackage.Deprecated != nil {
			deprecated := pkg.BasePackage.Deprecated

			// if not existing or current version is greater than existing, update
			if existing, found := (*deprecatedPackages)[pkg.BasePackage.Name]; !found || pkg.versionSemVer.GreaterThan(existing.version) {
				(*deprecatedPackages)[pkg.BasePackage.Name] = deprecatedMeta{
					deprecated: deprecated,
					version:    pkg.versionSemVer,
				}
			}
		}
	}
}

// PropagateLatestDeprecatedInfoToPackageList adds deprecation information to all packages in the package list
// based on the latest deprecated info available in the deprecated packages map.
func PropagateLatestDeprecatedInfoToPackageList(packageList Packages, deprecatedPackages DeprecatedPackages) {
	for _, pkg := range packageList {
		if deprecatedInfo, found := deprecatedPackages.Deprecated(pkg.Name); found {
			pkg.Deprecated = deprecatedInfo
		}
	}
}
