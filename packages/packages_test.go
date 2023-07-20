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
		{
			Name:          "redisenterprise",
			Version:       "0.1.1",
			Release:       "beta",
			Type:          "integration",
			KibanaVersion: "^7.14.0 || ^8.0.0",
		},
		{
			Name:          "redisenterprise",
			Version:       "1.0.0",
			Type:          "integration",
			KibanaVersion: "^8.0.0",
		},
		{
			Name:         "obs_package",
			Version:      "1.1.0",
			Type:         "integration",
			Capabilities: []string{"observability"},
		},
		{
			Name:         "obs_sec_package",
			Version:      "1.0.0",
			Type:         "integration",
			Capabilities: []string{"observability", "security"},
		},
		{
			Name:         "obs_sec_package",
			Version:      "2.0.0-rc1",
			Type:         "integration",
			Capabilities: []string{"observability", "security"},
		},
		{
			Name:         "obs_sec_package",
			Version:      "2.0.0",
			Type:         "integration",
			Capabilities: []string{"observability", "security"},
		},
		{
			Name:         "obs_sec_uptime_package",
			Version:      "2.0.0",
			Type:         "integration",
			Capabilities: []string{"observability", "security", "uptime"},
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
				// It is ok to don't return the following package, these cases
				// should be released without experimental flag as they have
				// GA versions. It would be returned in any case if
				// `prerelease=true` is used, as in the following test.
				// {Name: "logstash", Version: "1.1.0"}
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
				{Name: "redisenterprise", Version: "1.0.0"},
				{Name: "obs_package", Version: "1.1.0"},
				{Name: "obs_sec_package", Version: "2.0.0"},
				{Name: "obs_sec_uptime_package", Version: "2.0.0"},
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
		{
			Title: "redisenterprise experimental search - future kibana",
			Filter: Filter{
				PackageName:   "redisenterprise",
				Prerelease:    true,
				KibanaVersion: semver.MustParse("8.7.0"),
			},
			Expected: []filterTestPackage{
				{Name: "redisenterprise", Version: "1.0.0"},
			},
		},
		{
			Title: "redisenterprise experimental search all versions - future kibana",
			Filter: Filter{
				PackageName:   "redisenterprise",
				Prerelease:    true,
				KibanaVersion: semver.MustParse("8.7.0"),
				AllVersions:   true,
			},
			Expected: []filterTestPackage{
				{Name: "redisenterprise", Version: "0.1.1"},
				{Name: "redisenterprise", Version: "1.0.0"},
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
				// Prerelease versions must be skipped if there are GA versions.
				// See: https://github.com/elastic/package-registry/pull/893
				filterTestPackage{Name: "apache", Version: "1.0.0-rc1"},
				filterTestPackage{Name: "apache", Version: "2.0.0-rc2"},
				filterTestPackage{Name: "redisenterprise", Version: "0.1.1"},
				filterTestPackage{Name: "obs_sec_package", Version: "2.0.0-rc1"},
			),
		},
		{
			// Prerelease versions must be skipped if there are GA versions.
			// See: https://github.com/elastic/package-registry/pull/893
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
			// See: https://github.com/elastic/package-registry/pull/893
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
			Title: "redisenterprise experimental search all versions - legacy kibana 7.14.0",
			Filter: Filter{
				PackageName:   "redisenterprise",
				Experimental:  true,
				KibanaVersion: semver.MustParse("7.14.0"),
				AllVersions:   true,
			},
			Expected: []filterTestPackage{
				// Only version available for 7.14 is 0.1.1, that is a prerelease.
				{Name: "redisenterprise", Version: "0.1.1"},
			},
		},
		{
			Title: "redisenterprise experimental search all versions - legacy kibana 8.5.0",
			Filter: Filter{
				PackageName:   "redisenterprise",
				Experimental:  true,
				KibanaVersion: semver.MustParse("8.5.0"),
				AllVersions:   true,
			},
			Expected: []filterTestPackage{
				// There are two versions available for 8.5, but we return only
				// the GA one to avoid exposing prereleases to legacy kibanas.
				// See: https://github.com/elastic/package-registry/pull/893
				{Name: "redisenterprise", Version: "1.0.0"},
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
		{
			Title: "non existing capabilities search",
			Filter: Filter{
				Capabilities: []string{"no_match"},
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "1.0.0"},
				{Name: "nginx", Version: "2.0.0"},
				{Name: "redisenterprise", Version: "1.0.0"},
			},
		},
		{
			Title: "observability capabilities search",
			Filter: Filter{
				Capabilities: []string{"observability"},
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "1.0.0"},
				{Name: "nginx", Version: "2.0.0"},
				{Name: "redisenterprise", Version: "1.0.0"},
				{Name: "obs_package", Version: "1.1.0"},
			},
		},
		{
			Title: "observability and security capabilities search",
			Filter: Filter{
				Capabilities: []string{"observability", "security"},
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "1.0.0"},
				{Name: "nginx", Version: "2.0.0"},
				{Name: "redisenterprise", Version: "1.0.0"},
				{Name: "obs_package", Version: "1.1.0"},
				{Name: "obs_sec_package", Version: "2.0.0"},
			},
		},
		{
			Title: "observability, security and uptime capabilities search - legacy kibana",
			Filter: Filter{
				Experimental: true,
				Capabilities: []string{"observability", "security", "uptime"},
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "1.0.0"},
				{Name: "nginx", Version: "2.0.0"},
				{Name: "mysql", Version: "0.9.0"},
				{Name: "logstash", Version: "1.1.0"},
				{Name: "etcd", Version: "1.0.0-rc2"},
				{Name: "redisenterprise", Version: "1.0.0"},
				{Name: "obs_package", Version: "1.1.0"},
				{Name: "obs_sec_package", Version: "2.0.0"},
				{Name: "obs_sec_uptime_package", Version: "2.0.0"},
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

func TestPackagesSpecMinMaxFilter(t *testing.T) {
	filterTestPackages := []filterTestPackage{
		{
			FormatVersion: "2.0.0",
			Name:          "apache",
			Version:       "1.0.0",
			Release:       "ga",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
		{
			FormatVersion: "2.0.0",
			Name:          "apache",
			Version:       "2.0.0-rc2",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
		{
			FormatVersion: "2.1.0",
			Name:          "nginx",
			Version:       "2.0.0",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
		{
			FormatVersion: "1.0.0",
			Name:          "mysql",
			Version:       "0.9.0",
			Release:       "experimental",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
		{
			FormatVersion: "3.0.0",
			Name:          "logstash",
			Version:       "1.1.0",
			Release:       "experimental",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.2.0",
		},
		{
			FormatVersion: "2.9.0",
			Name:          "etcd",
			Version:       "1.0.0-rc1",
			Type:          "integration",
			KibanaVersion: "^8.0.0",
		},
		{
			FormatVersion: "2.9.0",
			Name:          "etcd",
			Version:       "1.0.0-rc2",
			Type:          "integration",
			KibanaVersion: "^8.0.0",
		},
		{
			FormatVersion: "2.9.0",
			Name:          "redisenterprise",
			Version:       "0.1.1",
			Release:       "beta",
			Type:          "integration",
			KibanaVersion: "^7.14.0 || ^8.0.0",
		},
		{
			FormatVersion: "3.5.0",
			Name:          "redisenterprise",
			Version:       "1.0.0",
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
			Title: "all packages",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				SpecMin:     semver.MustParse("0.0.0"),
				SpecMax:     semver.MustParse("5.0.0"),
			},
			Expected: filterTestPackages,
		},
		{
			Title: "no packages match spec",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				SpecMin:     semver.MustParse("5.0.0"),
				SpecMax:     semver.MustParse("6.0.0"),
			},
			Expected: []filterTestPackage{},
		},
		{
			Title: "use min and max spec to filter packages",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				SpecMin:     semver.MustParse("2.1.0"),
				SpecMax:     semver.MustParse("3.0.0"),
			},
			Expected: []filterTestPackage{
				{Name: "nginx", Version: "2.0.0"},
				{Name: "logstash", Version: "1.1.0"},
				{Name: "etcd", Version: "1.0.0-rc1"},
				{Name: "etcd", Version: "1.0.0-rc2"},
				{Name: "redisenterprise", Version: "0.1.1"},
			},
		},
		{
			Title: "use just max spec to filter packages without kibana Version",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				SpecMax:     semver.MustParse("3.4.0"),
			},
			Expected: []filterTestPackage{
				{Name: "logstash", Version: "1.1.0"},
			},
		},
		{
			Title: "use just max spec to filter packages with kibana Version",
			Filter: Filter{
				AllVersions:   true,
				Prerelease:    true,
				KibanaVersion: semver.MustParse("8.1.0"),
				SpecMax:       semver.MustParse("3.0.0"),
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "1.0.0"},
				{Name: "apache", Version: "2.0.0-rc2"},
				{Name: "nginx", Version: "2.0.0"},
				{Name: "mysql", Version: "0.9.0"},
				{Name: "etcd", Version: "1.0.0-rc1"},
				{Name: "etcd", Version: "1.0.0-rc2"},
				{Name: "redisenterprise", Version: "0.1.1"},
			},
		},
		{
			Title: "use just min spec to filter packages",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				SpecMin:     semver.MustParse("3.0.0"),
			},
			Expected: []filterTestPackage{
				{Name: "logstash", Version: "1.1.0"},
				{Name: "redisenterprise", Version: "1.0.0"},
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
	FormatVersion string
	Name          string
	Version       string
	Release       string
	Type          string
	KibanaVersion string
	Capabilities  []string
}

func (p filterTestPackage) Build() *Package {
	var build Package
	build.Name = p.Name
	build.Version = p.Version
	build.versionSemVer = semver.MustParse(p.Version)
	build.FormatVersion = p.FormatVersion

	build.Release = p.Release
	build.Type = p.Type

	if p.KibanaVersion != "" {
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
	}
	if p.Capabilities != nil {
		elasticConditions := ElasticConditions{
			Capabilities: p.Capabilities,
		}
		if build.Conditions != nil {
			build.Conditions.Elastic = &elasticConditions
		} else {
			build.Conditions = &Conditions{
				Elastic: &elasticConditions,
			}
		}
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
