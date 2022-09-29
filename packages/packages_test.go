// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestPackagesFilter(t *testing.T) {
	filterTestPackages := []filterTestPackage{
		{
			Name:          "apache",
			Version:       "1.0.0",
			Release:       "ga",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
		{
			Name:          "apache",
			Version:       "2.0.0-rc2",
			Release:       "beta",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
		{
			Name:          "nginx",
			Version:       "1.0.0",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
		{
			Name:          "nginx",
			Version:       "2.0.0",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
	}
	packages := buildFilterTestPackages(filterTestPackages)

	cases := []struct {
		Title    string
		Filter   Filter
		Expected []filterTestPackage
	}{
		{
			Title: "not matching package name",
			Filter: Filter{
				PackageName: "unknown",
			},
			Expected: []filterTestPackage{},
		},
		{
			Title: "not matching package version",
			Filter: Filter{
				PackageName:    "apache",
				PackageVersion: "1.2.3",
			},
			Expected: []filterTestPackage{},
		},
		{
			Title: "not matching package version and all enabled",
			Filter: Filter{
				PackageName:    "apache",
				PackageVersion: "1.2.3",
				Experimental:   true,
				Prerelease:     true,
				AllVersions:    true,
			},
			Expected: []filterTestPackage{},
		},
		{
			Title:  "all packages",
			Filter: Filter{},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "1.0.0"},
				{Name: "nginx", Version: "2.0.0"},
			},
		},
		{
			Title: "apache package default search",
			Filter: Filter{
				PackageName: "apache",
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "1.0.0"},
			},
		},
		{
			Title: "apache package prerelease search",
			Filter: Filter{
				PackageName: "apache",
				Prerelease:  true,
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "2.0.0-rc2"},
			},
		},
		{
			Title: "apache package experimental search",
			Filter: Filter{
				PackageName:  "apache",
				Experimental: true,
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "2.0.0-rc2"},
			},
		},
		{
			Title: "nginx package experimental search",
			Filter: Filter{
				PackageName:  "nginx",
				Experimental: true,
			},
			Expected: []filterTestPackage{
				{Name: "nginx", Version: "2.0.0"},
			},
		},

		// Legacy Kibana, experimental is always true, prerelease set to true for compatibility.
		{
			Title: "all packages and versions - legacy kibana",
			Filter: Filter{
				AllVersions:  true,
				Experimental: true,
				Prerelease:   ExperimentalPrereleaseCompatibility,
			},
			Expected: filterTestPackages,
		},
		{
			Title: "apache package experimental search - legacy kibana",
			Filter: Filter{
				PackageName:  "apache",
				Experimental: true,
				Prerelease:   ExperimentalPrereleaseCompatibility,
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "2.0.0-rc2"},
			},
		},
		{
			Title: "nginx package experimental search - legacy kibana",
			Filter: Filter{
				PackageName:  "nginx",
				Experimental: true,
				Prerelease:   ExperimentalPrereleaseCompatibility,
			},
			Expected: []filterTestPackage{
				{Name: "nginx", Version: "2.0.0"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Title, func(t *testing.T) {
			result := c.Filter.Apply(context.Background(), packages)
			assertFilterPackagesResult(t, c.Expected, result)
		})
	}
}

type filterTestPackage struct {
	Name          string
	Version       string
	Release       string
	Type          string
	KibanaVersion string
}

func (p filterTestPackage) Build() *Package {
	var build Package
	build.Name = p.Name
	build.Version = p.Version
	build.versionSemVer = semver.MustParse(p.Version)

	build.Release = p.Release
	build.Type = p.Type

	constraints, err := semver.NewConstraint(p.KibanaVersion)
	if err != nil {
		panic(err)
	}
	build.Conditions = &Conditions{
		Kibana: &KibanaConditions{
			Version:    p.KibanaVersion,
			constraint: constraints,
		},
	}
	return &build
}

func (p filterTestPackage) Instances(i *Package) bool {
	if p.Name != i.Name {
		return false
	}
	if p.Version != i.Version {
		return false
	}
	return true
}

func (p filterTestPackage) String() string {
	return p.Name + "-" + p.Version
}

func buildFilterTestPackages(testPackages []filterTestPackage) Packages {
	packages := make(Packages, len(testPackages))
	for i, p := range testPackages {
		packages[i] = p.Build()
	}
	return packages
}

func assertFilterPackagesResult(t *testing.T, expected []filterTestPackage, found Packages) {
	t.Helper()

	if len(expected) != len(found) {
		t.Errorf("expected %d packages, found %d", len(expected), len(found))
	}
	for _, e := range expected {
		ok := false
		for _, f := range found {
			if e.Instances(f) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("expected package %s not found", e)
		}
	}

	if t.Failed() {
		t.Log("Packages found:")
		for _, p := range found {
			t.Logf("- %s-%s", p.Name, p.Version)
		}
	}
}
