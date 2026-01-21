// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"sort"

	"github.com/Masterminds/semver/v3"
)

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

// SortByNameVersion sorts the packages by name and version (newest first).
// the original slice is modified.
func SortByNameVersion(packages Packages) {
	sort.Sort(byNameVersion(packages))
}
