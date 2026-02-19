// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/package-registry/archiver"
	"github.com/elastic/package-registry/internal/util"
)

func BenchmarkInit(b *testing.B) {
	// given
	packagesBasePaths := []string{"../testdata/second_package_path", "../testdata/package"}

	testLogger := util.NewTestLoggerLevel(zapcore.FatalLevel)
	b.ResetTimer()
	fsOptions := FSIndexerOptions{
		Logger: testLogger,
	}
	for i := 0; i < b.N; i++ {
		zipIndexer := NewZipFileSystemIndexer(fsOptions, "../testdata/local-storage")
		dirIndexer := NewFileSystemIndexer(fsOptions, packagesBasePaths...)

		err := zipIndexer.Init(b.Context())
		require.NoError(b, err)

		err = dirIndexer.Init(b.Context())
		require.NoError(b, err)

		b.StopTimer()
		require.NoError(b, zipIndexer.Close(b.Context()))
		require.NoError(b, dirIndexer.Close(b.Context()))
		b.StartTimer()
	}
}

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
				filterTestPackage{Name: "obs_sec_package", Version: "2.0.0-rc1"},
				// Tech preview versions (major < 1) must be skipped if there are GA versions.
				filterTestPackage{Name: "redisenterprise", Version: "0.1.1"},
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
				// the GA one to avoid exposing prereleases and tech previews to legacy kibanas.
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
			result, err := c.Filter.Apply(t.Context(), packages)
			require.NoError(t, err)
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
			DiscoveryFields: []string{
				"host.ip",
				"nginx.stubstatus.hostname",
			},
		},
		{
			FormatVersion: "1.0.0",
			Name:          "mysql",
			Version:       "0.9.0",
			Release:       "experimental",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
			DiscoveryFields: []string{
				"server.fqdn",
			},
		},
		{
			FormatVersion: "3.0.0",
			Name:          "logstash",
			Version:       "1.1.0",
			Release:       "experimental",
			Type:          "integration",
			KibanaVersion: "^7.17.0 || ^8.0.0",
		},
		{
			FormatVersion: "3.1.0",
			Name:          "logstash",
			Version:       "2.0.0",
			Type:          "integration",
			KibanaVersion: "^8.4.0",
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
		{
			FormatVersion: "3.6.0",
			Name:          "redisenterprise",
			Version:       "1.1.0",
			Type:          "integration",
			KibanaVersion: "^8.5.0",
		},
		{
			FormatVersion: "3.6.1",
			Name:          "redisenterprise",
			Version:       "1.1.1",
			Type:          "integration",
			KibanaVersion: "^8.5.0",
			DiscoveryFields: []string{
				"redis.cluster.name",
			},
			DiscoveryDatasets: []string{
				"redisenterprise.cluster",
				"redisenterprise.database",
			},
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
				SpecMin:     semver.MustParse("0.0"),
				SpecMax:     semver.MustParse("5.0"),
			},
			Expected: filterTestPackages,
		},
		{
			Title: "no packages match spec",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				SpecMin:     semver.MustParse("5.0"),
				SpecMax:     semver.MustParse("6.0"),
			},
			Expected: []filterTestPackage{},
		},
		{
			Title: "use min and max spec to filter packages",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				SpecMin:     semver.MustParse("2.2"),
				SpecMax:     semver.MustParse("3.6"),
			},
			Expected: []filterTestPackage{
				{Name: "logstash", Version: "1.1.0"},
				{Name: "logstash", Version: "2.0.0"},
				{Name: "etcd", Version: "1.0.0-rc1"},
				{Name: "etcd", Version: "1.0.0-rc2"},
				{Name: "redisenterprise", Version: "0.1.1"},
				{Name: "redisenterprise", Version: "1.0.0"},
				{Name: "redisenterprise", Version: "1.1.0"},
				{Name: "redisenterprise", Version: "1.1.1"},
			},
		},
		{
			Title: "use spec and kibana.version to filter packages",
			Filter: Filter{
				AllVersions:   true,
				Prerelease:    true,
				KibanaVersion: semver.MustParse("8.1.0"),
				SpecMin:       semver.MustParse("2.2"),
				SpecMax:       semver.MustParse("3.6"),
			},
			Expected: []filterTestPackage{
				{Name: "logstash", Version: "1.1.0"},
				{Name: "etcd", Version: "1.0.0-rc1"},
				{Name: "etcd", Version: "1.0.0-rc2"},
				{Name: "redisenterprise", Version: "0.1.1"},
				{Name: "redisenterprise", Version: "1.0.0"},
			},
		},
		{
			Title: "use max spec to filter packages with no Kibana version and no min spec",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				SpecMax:     semver.MustParse("3.0"),
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "1.0.0"},
				{Name: "apache", Version: "2.0.0-rc2"},
				{Name: "nginx", Version: "2.0.0"},
				{Name: "mysql", Version: "0.9.0"},
				{Name: "logstash", Version: "1.1.0"},
				{Name: "etcd", Version: "1.0.0-rc1"},
				{Name: "etcd", Version: "1.0.0-rc2"},
				{Name: "redisenterprise", Version: "0.1.1"},
			},
		},
		{
			Title: "use just max spec to filter packages with Kibana version",
			Filter: Filter{
				AllVersions:   true,
				Prerelease:    true,
				KibanaVersion: semver.MustParse("7.17.0"),
				SpecMax:       semver.MustParse("3.0"),
			},
			Expected: []filterTestPackage{
				{Name: "apache", Version: "1.0.0"},
				{Name: "apache", Version: "2.0.0-rc2"},
				{Name: "nginx", Version: "2.0.0"},
				{Name: "mysql", Version: "0.9.0"},
				{Name: "logstash", Version: "1.1.0"},
				{Name: "redisenterprise", Version: "0.1.1"},
			},
		},
		{
			Title: "use just min spec to filter packages",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				SpecMin:     semver.MustParse("3.0"),
			},
			Expected: []filterTestPackage{
				{Name: "logstash", Version: "1.1.0"},
				{Name: "logstash", Version: "2.0.0"},
				{Name: "redisenterprise", Version: "1.0.0"},
				{Name: "redisenterprise", Version: "1.1.0"},
				{Name: "redisenterprise", Version: "1.1.1"},
			},
		},
		{
			Title: "use just min spec to filter packages with kibana version",
			Filter: Filter{
				AllVersions:   true,
				Prerelease:    true,
				KibanaVersion: semver.MustParse("8.1.0"),
				SpecMin:       semver.MustParse("3.0"),
			},
			Expected: []filterTestPackage{
				{Name: "logstash", Version: "1.1.0"},
				{Name: "redisenterprise", Version: "1.0.0"},
			},
		},
		{
			Title: "use fields discovery filter that no packages match",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				Discovery:   mustBuildDiscoveryFilter([]string{"fields:apache.status.total_bytes"}),
			},
			Expected: []filterTestPackage{},
		},
		{
			Title: "use fields discovery filter for the nginx package",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				Discovery:   mustBuildDiscoveryFilter([]string{"fields:host.ip,nginx.stubstatus.hostname"}),
			},
			Expected: []filterTestPackage{
				{Name: "nginx", Version: "2.0.0"},
			},
		},
		{
			Title: "use fields discovery filter for the nginx package with more query parameters",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				Discovery:   mustBuildDiscoveryFilter([]string{"fields:nginx.stubstatus.hostname,host.ip,other"}),
			},
			Expected: []filterTestPackage{
				{Name: "nginx", Version: "2.0.0"},
			},
		},
		{
			Title: "use fields discovery filter with no value and no matching any package",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				Discovery:   mustBuildDiscoveryFilter([]string{"fields:event.dataset"}),
			},
			Expected: []filterTestPackage{},
		},
		{
			Title: "use datasets discovery filter with all redisenterprise datasets",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				Discovery:   mustBuildDiscoveryFilter([]string{"datasets:redisenterprise.cluster,redisenterprise.database"}),
			},
			Expected: []filterTestPackage{
				{Name: "redisenterprise", Version: "1.1.1"},
			},
		},
		{
			Title: "use datasets discovery filter with just one redisenterprise dataset",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				Discovery:   mustBuildDiscoveryFilter([]string{"datasets:redisenterprise.cluster"}),
			},
			Expected: []filterTestPackage{
				{Name: "redisenterprise", Version: "1.1.1"},
			},
		},
		{
			Title: "use discovery filter with datasets and fields matching both",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				Discovery: mustBuildDiscoveryFilter([]string{
					"datasets:redisenterprise.cluster",
					"fields:redis.cluster.name",
				}),
			},
			Expected: []filterTestPackage{
				{Name: "redisenterprise", Version: "1.1.1"},
			},
		},
		{
			Title: "use discovery filter with datasets and fields but not matching both",
			Filter: Filter{
				AllVersions: true,
				Prerelease:  true,
				Discovery: mustBuildDiscoveryFilter([]string{
					"datasets:redisenterprise.cluster",
					"fields:redis.host.name",
				}),
			},
			Expected: []filterTestPackage{},
		},
	}

	for _, c := range cases {
		t.Run(c.Title, func(t *testing.T) {
			result, err := c.Filter.Apply(t.Context(), packages)
			require.NoError(t, err)
			assertFilterPackagesResult(t, c.Expected, result)
		})
	}
}

func TestFileSystemIndexer_watchPackageFileSystem(t *testing.T) {
	t.Run("context cancellation stops watcher", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ValidationDisabled = true

		tmpDir := t.TempDir()
		logger := zap.NewNop()

		indexer := NewFileSystemIndexer(FSIndexerOptions{
			Logger:             logger,
			EnablePathsWatcher: true,
		}, tmpDir)

		indexer.packageList = make(Packages, 0)

		done := make(chan struct{})
		go func() {
			indexer.watchPackageFileSystem(ctx)
			close(done)
		}()

		// Cancel context and verify watcher stops
		cancel()

		select {
		case <-done:
			// Success - watcher stopped
		case <-time.After(2 * time.Second):
			t.Fatal("watcher did not stop after context cancellation")
		}
	})

	t.Run("file indexer debouncing filesystem events", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ValidationDisabled = true
		tmpDir := t.TempDir()
		logger := zap.NewNop()

		indexer := NewFileSystemIndexer(FSIndexerOptions{
			Logger:             logger,
			EnablePathsWatcher: true,
		}, tmpDir)

		indexer.packageList = make(Packages, 0)

		done := make(chan struct{})
		go func() {
			indexer.watchPackageFileSystem(ctx)
			close(done)
		}()

		// Give watcher time to initialize
		time.Sleep(100 * time.Millisecond)

		for i := 0; i < 25; i++ {
			createMockPackage(t, tmpDir, fmt.Sprintf("mypackage%d", i))
		}

		// Wait for debounce period plus buffer
		time.Sleep(1500 * time.Millisecond)

		cancel()
		<-done

		assert.Len(t, indexer.packageList, 25)
	})

	t.Run("zip indexer debouncing filesystem events", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ValidationDisabled = true
		tmpDir := t.TempDir()
		logger := zap.NewNop()

		indexer := NewZipFileSystemIndexer(FSIndexerOptions{
			Logger:             logger,
			EnablePathsWatcher: true,
		}, tmpDir)

		indexer.packageList = make(Packages, 0)

		done := make(chan struct{})
		go func() {
			indexer.watchPackageFileSystem(ctx)
			close(done)
		}()

		// Give watcher time to initialize
		time.Sleep(100 * time.Millisecond)

		for i := 0; i < 25; i++ {
			path := createMockZipPackage(t, tmpDir, fmt.Sprintf("mypackage%d", i))
			defer func() {
				err := os.Remove(path)
				require.NoError(t, err)
			}()
		}

		// Wait for debounce period plus buffer
		time.Sleep(1500 * time.Millisecond)

		cancel()
		<-done

		assert.Len(t, indexer.packageList, 25)
	})

	t.Run("zip indexer ignores non-zip files", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ValidationDisabled = true

		tmpDir := t.TempDir()
		logger := zap.NewNop()

		indexer := NewZipFileSystemIndexer(FSIndexerOptions{
			Logger:             logger,
			EnablePathsWatcher: true,
		}, tmpDir)

		indexer.packageList = make(Packages, 0)

		done := make(chan struct{})
		go func() {
			indexer.watchPackageFileSystem(ctx)
			close(done)
		}()

		time.Sleep(100 * time.Millisecond)

		// Create non-zip file (should be ignored by zip indexer)
		createMockPackage(t, tmpDir, "mypackage")
		time.Sleep(1500 * time.Millisecond)
		assert.Empty(t, indexer.packageList)

		path := createMockZipPackage(t, tmpDir, "mypackage")
		defer func() {
			err := os.Remove(path)
			require.NoError(t, err)
		}()
		time.Sleep(1500 * time.Millisecond)

		cancel()
		<-done

		assert.Len(t, indexer.packageList, 1)
	})

	t.Run("file system indexer ignores zip files", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ValidationDisabled = true

		tmpDir := t.TempDir()
		logger := zap.NewNop()

		indexer := NewFileSystemIndexer(FSIndexerOptions{
			Logger:             logger,
			EnablePathsWatcher: true,
		}, tmpDir)

		indexer.packageList = make(Packages, 0)

		done := make(chan struct{})
		go func() {
			indexer.watchPackageFileSystem(ctx)
			close(done)
		}()

		time.Sleep(100 * time.Millisecond)

		// Create zip file (should be ignored by regular file system indexer)
		path := createMockZipPackage(t, tmpDir, "mypackage")
		defer func() {
			err := os.Remove(path)
			require.NoError(t, err)
		}()
		time.Sleep(1500 * time.Millisecond)
		assert.Empty(t, indexer.packageList)

		createMockPackage(t, tmpDir, "mypackage")
		time.Sleep(1500 * time.Millisecond)

		cancel()
		<-done

		assert.Len(t, indexer.packageList, 1)
	})
}

func createMockPackage(t *testing.T, dest, pkgName string) {
	t.Helper()

	pkgDir := filepath.Join(dest, pkgName, "1.0.0")
	err := os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)

	manifestContent := `name: ` + pkgName + `
version: 1.0.0
type: integration
format_version: 1.0.0
`
	err = os.WriteFile(filepath.Join(pkgDir, "manifest.yml"), []byte(manifestContent), 0644)
	require.NoError(t, err)

	docsDir := filepath.Join(pkgDir, "docs")
	err = os.MkdirAll(docsDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(docsDir, "README.md"), []byte("# My Package\nThis is a test package."), 0644)
	require.NoError(t, err)
}

func createMockZipPackage(t *testing.T, dest, pkgName string) string {
	t.Helper()

	tmpMockFSDir := t.TempDir()
	createMockPackage(t, tmpMockFSDir, pkgName)

	zipFilepath := filepath.Join(dest, pkgName+"-1.0.0.zip")
	file, err := os.Create(zipFilepath)
	require.NoError(t, err)

	err = archiver.ArchivePackage(file, archiver.PackageProperties{
		Name:    pkgName,
		Version: "1.0.0",
		Path:    filepath.Join(tmpMockFSDir, pkgName, "1.0.0"),
	})
	require.NoError(t, err)

	err = file.Close()
	require.NoError(t, err)

	return zipFilepath
}

func mustBuildDiscoveryFilter(filters []string) discoveryFilters {
	discoveryFilters := make([]*discoveryFilter, 0, len(filters))
	for _, filter := range filters {
		if filter == "" {
			panic("discovery filter cannot be empty")
		}
		f, err := NewDiscoveryFilter(filter)
		if err != nil {
			panic(err)
		}
		discoveryFilters = append(discoveryFilters, f)
	}
	return discoveryFilters
}

type filterTestPackage struct {
	FormatVersion     string
	Name              string
	Version           string
	Release           string
	Type              string
	KibanaVersion     string
	Capabilities      []string
	DiscoveryFields   []string
	DiscoveryDatasets []string
}

func (p filterTestPackage) Build() *Package {
	var build Package
	build.Name = p.Name
	build.Version = p.Version
	build.versionSemVer = semver.MustParse(p.Version)
	build.FormatVersion = p.FormatVersion
	if p.FormatVersion == "" {
		// set a default format_spec version for tests
		build.FormatVersion = "1.0.0"
	}

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

	for _, parameter := range p.DiscoveryFields {
		if build.Discovery == nil {
			build.Discovery = &Discovery{}
		}
		filterField := newDiscoveryFilterField(parameter)
		build.Discovery.Fields = append(build.Discovery.Fields, filterField)
	}

	for _, parameter := range p.DiscoveryDatasets {
		if build.Discovery == nil {
			build.Discovery = &Discovery{}
		}
		filterDataset := newDiscoveryFilterDataset(parameter)
		build.Discovery.Datasets = append(build.Discovery.Datasets, filterDataset)
	}

	// set spec semver.Version variables
	build.setRuntimeFields()
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

func strPtr(s string) *string { return &s }

func TestPackagesSort(t *testing.T) {
	t.Run("sorts by title alphabetically", func(t *testing.T) {
		pkgs := Packages{
			{BasePackage: BasePackage{Name: "c_pkg", Title: strPtr("Zebra Integration"), Version: "1.0.0"}},
			{BasePackage: BasePackage{Name: "a_pkg", Title: strPtr("Apple Integration"), Version: "1.0.0"}},
			{BasePackage: BasePackage{Name: "b_pkg", Title: strPtr("Mango Integration"), Version: "1.0.0"}},
		}
		sort.Sort(pkgs)
		assert.Equal(t, "Apple Integration", *pkgs[0].Title)
		assert.Equal(t, "Mango Integration", *pkgs[1].Title)
		assert.Equal(t, "Zebra Integration", *pkgs[2].Title)
	})

	t.Run("sorts by semver version when titles are equal", func(t *testing.T) {
		// Regression test: lexicographic ordering would sort 8.19.2 after 8.19.15
		// because "2" > "1", but semantic ordering puts 8.19.2 before 8.19.15.
		pkgs := Packages{
			{BasePackage: BasePackage{Name: "security_detection_engine", Title: strPtr("Prebuilt Security Detection Rules"), Version: "8.19.15"}},
			{BasePackage: BasePackage{Name: "security_detection_engine", Title: strPtr("Prebuilt Security Detection Rules"), Version: "8.19.2"}},
			{BasePackage: BasePackage{Name: "security_detection_engine", Title: strPtr("Prebuilt Security Detection Rules"), Version: "8.2.1"}},
			{BasePackage: BasePackage{Name: "security_detection_engine", Title: strPtr("Prebuilt Security Detection Rules"), Version: "1.0.1"}},
			{BasePackage: BasePackage{Name: "security_detection_engine", Title: strPtr("Prebuilt Security Detection Rules"), Version: "8.19.9"}},
		}
		sort.Sort(pkgs)
		versions := make([]string, len(pkgs))
		for i, p := range pkgs {
			versions[i] = p.Version
		}
		assert.Equal(t, []string{"1.0.1", "8.2.1", "8.19.2", "8.19.9", "8.19.15"}, versions)
	})

	t.Run("sorts by title first, then version within same title", func(t *testing.T) {
		pkgs := Packages{
			{BasePackage: BasePackage{Name: "b_pkg", Title: strPtr("Beta"), Version: "2.0.0"}},
			{BasePackage: BasePackage{Name: "a_pkg", Title: strPtr("Alpha"), Version: "1.0.0"}},
			{BasePackage: BasePackage{Name: "b_pkg", Title: strPtr("Beta"), Version: "10.0.0"}},
			{BasePackage: BasePackage{Name: "a_pkg", Title: strPtr("Alpha"), Version: "9.0.0"}},
		}
		sort.Sort(pkgs)
		assert.Equal(t, "Alpha", *pkgs[0].Title)
		assert.Equal(t, "1.0.0", pkgs[0].Version)
		assert.Equal(t, "Alpha", *pkgs[1].Title)
		assert.Equal(t, "9.0.0", pkgs[1].Version)
		assert.Equal(t, "Beta", *pkgs[2].Title)
		assert.Equal(t, "2.0.0", pkgs[2].Version)
		assert.Equal(t, "Beta", *pkgs[3].Title)
		assert.Equal(t, "10.0.0", pkgs[3].Version)
	})
}
