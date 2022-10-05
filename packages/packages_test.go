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
			Version:       "1.0.0-rc1",
			Release:       "beta",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
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
		{
			Name:          "mysql",
			Version:       "0.9.0",
			Release:       "experimental",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
		{
			Name:          "logstash",
			Version:       "1.1.0",
			Release:       "experimental",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
		{
			Name:          "etcd",
			Version:       "1.0.0-rc1",
			Type:          "integration",
			KibanaVersion: "^8.0.0",
		},
		{
			Name:          "etcd",
			Version:       "1.0.0-rc2",
			Type:          "integration",
			KibanaVersion: "^8.0.0",
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
			Title: "prerelease package with experimental release flag default search",
			Filter: Filter{
				PackageName: "mysql",
			},
			Expected: []filterTestPackage{},
		},
		{
			Title: "prerelease package with experimental release flag prerelease search",
			Filter: Filter{
				PackageName: "mysql",
				Prerelease:  true,
			},
			Expected: []filterTestPackage{
				{Name: "mysql", Version: "0.9.0"},
			},
		},
		{
			Title: "non-prerelease package with experimental release flag default search",
			Filter: Filter{
				PackageName: "logstash",
			},
			Expected: []filterTestPackage{
				{Name: "logstash", Version: "1.1.0"},
			},
		},
		{
			Title: "non-prerelease package with experimental release flag prerelease search",
			Filter: Filter{
				PackageName: "logstash",
				Prerelease:  true,
			},
			Expected: []filterTestPackage{
				{Name: "logstash", Version: "1.1.0"},
			},
		},
		{
			Title: "not matching package version and all enabled",
			Filter: Filter{
				PackageName:    "apache",
				PackageVersion: "1.2.3",
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
				{Name: "logstash", Version: "1.1.0"},
			},
		},
		{
			Title: "all packages and all versions",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
			},
			Expected: filterTestPackages,
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

		// Legacy Kibana, experimental is always true.
		{
			Title: "all packages and versions - legacy kibana",
			Filter: Filter{
				AllVersions:  true,
				Experimental: true,
			},
			Expected: removeFilterTestPackages(filterTestPackages,
				filterTestPackage{Name: "apache", Version: "1.0.0-rc1"},
				filterTestPackage{Name: "apache", Version: "2.0.0-rc2"},
			),
		},
		{
			// Prerelease versions must be skipped if there are GA versions.
			// See: https://github.com/elastic/ingest-dev/issues/1285
			Title: "apache package experimental search - legacy kibana",
			Filter: Filter{
				PackageName:  "apache",
				Experimental: true,
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "1.0.0"},
			},
		},
		{
			// Prerelease versions must be skipped if there are GA versions.
			// See: https://github.com/elastic/ingest-dev/issues/1285
			Title: "apache package experimental search all versions - legacy kibana",
			Filter: Filter{
				PackageName:  "apache",
				Experimental: true,
				AllVersions:  true,
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "1.0.0"},
			},
		},
		{
			Title: "nginx package experimental search - legacy kibana",
			Filter: Filter{
				PackageName:  "nginx",
				Experimental: true,
			},
			Expected: []filterTestPackage{
				{Name: "nginx", Version: "2.0.0"},
			},
		},
		{
			Title: "nginx package experimental search all versions - legacy kibana",
			Filter: Filter{
				PackageName:  "nginx",
				Experimental: true,
				AllVersions:  true,
			},
			Expected: []filterTestPackage{
				{Name: "nginx", Version: "1.0.0"},
				{Name: "nginx", Version: "2.0.0"},
			},
		},
		{
			Title: "logstash package experimental search - legacy kibana",
			Filter: Filter{
				PackageName:  "logstash",
				Experimental: true,
			},
			Expected: []filterTestPackage{
				{Name: "logstash", Version: "1.1.0"},
			},
		},
		{
			Title: "mysql package experimental search - legacy kibana",
			Filter: Filter{
				PackageName:  "mysql",
				Experimental: true,
			},
			Expected: []filterTestPackage{
				{Name: "mysql", Version: "0.9.0"},
			},
		},
		{
			Title: "etcd package experimental search - legacy kibana",
			Filter: Filter{
				PackageName:  "etcd",
				Experimental: true,
			},
			Expected: []filterTestPackage{
				{Name: "etcd", Version: "1.0.0-rc2"},
			},
		},
		{
			Title: "etcd package experimental search all versions - legacy kibana",
			Filter: Filter{
				PackageName:  "etcd",
				Experimental: true,
				AllVersions:  true,
			},
			Expected: []filterTestPackage{
				{Name: "etcd", Version: "1.0.0-rc1"},
				{Name: "etcd", Version: "1.0.0-rc2"},
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

func removeFilterTestPackages(testPackages []filterTestPackage, remove ...filterTestPackage) []filterTestPackage {
	var filtered []filterTestPackage
	for _, tp := range testPackages {
		found := false
		for _, rp := range remove {
			if rp.Name == tp.Name && rp.Version == tp.Version {
				found = true
				break
			}
		}
		if !found {
			filtered = append(filtered, tp)
		}
	}
	return filtered
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
