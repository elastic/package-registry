// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	title = "foo"
)
var packageTests = []struct {
	p           Package
	valid       bool
	description string
}{
	{
		Package{},
		false,
		"empty",
	},
	{
		Package{
			BasePackage: BasePackage{
				Title: &title,
			},
		},
		false,
		"missing description",
	},
	{
		Package{
			BasePackage: BasePackage{
				Title: &title,
				Conditions: &Conditions{
					Kibana: &KibanaConditions{Version: "bar"},
				},
			},
		},
		false,
		"invalid Kibana version",
	},
	{
		Package{
			BasePackage: BasePackage{
				Title:       &title,
				Description: "my description",
				Conditions: &Conditions{
					Kibana: &KibanaConditions{Version: ">=1.2.3 <=4.5.6"},
				},
				Categories: []string{"custom", "foo"},
			},
		},
		false,
		"invalid category ",
	},
	{
		Package{
			BasePackage: BasePackage{
				Title:       &title,
				Description: "my description",
				Categories:  []string{"custom", "web"},
			},
		},
		false,
		"missing format_version",
	},
	{
		Package{
			BasePackage: BasePackage{
				Title:       &title,
				Description: "my description",
				Categories:  []string{"custom", "web"},
			},
			FormatVersion: "1.0",
		},
		false,
		"invalid format_version",
	},
	{
		Package{
			BasePackage: BasePackage{
				Title:       &title,
				Description: "my description",
				Version:     "1.0",
				Categories:  []string{"custom", "web"},
			},
			FormatVersion: "1.0.0",
		},
		false,
		"invalid package version",
	},
	{
		Package{
			BasePackage: BasePackage{
				Title:       &title,
				Description: "my description",
				Version:     "1.2.3",
				Categories:  []string{"custom", "web"},
			},
			FormatVersion: "1.0.0",
		},
		true,
		"complete",
	},
}

func TestValidate(t *testing.T) {
	for _, tt := range packageTests {
		t.Run(tt.description, func(t *testing.T) {
			err := tt.p.Validate()

			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

var kibanaVersionPackageTests = []struct {
	description   string
	constraint    string
	kibanaVersion string
	check         bool
}{
	{
		"last major",
		">= 7.0.0",
		"6.7.0",
		false,
	},
	{
		"next minor",
		">= 7.0.0",
		"7.1.0",
		true,
	},
	{
		"next minor tilde",
		"~7",
		"7.1.0",
		true,
	},
	{
		"next minor tilde, x",
		"~7.x.x",
		"7.1.0",
		true,
	},
	{
		"next minor tilde, not matching",
		"~7.0.0",
		"7.1.0",
		false,
	},
	{
		"next minor tilde, matching",
		"~7.0.x",
		"7.0.2",
		true,
	},
	{
		"inside major, not",
		"^7.6.0",
		"7.0.2",
		false,
	},
	{
		"inside major",
		"^7.6.0",
		"7.12.2",
		true,
	},
}

func TestHasKibanaVersion(t *testing.T) {
	for _, tt := range kibanaVersionPackageTests {
		t.Run(tt.description, func(t *testing.T) {

			constraint, err := semver.NewConstraint(tt.constraint)
			assert.NoError(t, err)

			p := Package{
				BasePackage: BasePackage{
					Conditions: &Conditions{
						Kibana: &KibanaConditions{
							constraint: constraint,
						},
					},
				},
			}

			kibanaVersion, err := semver.NewVersion(tt.kibanaVersion)
			assert.NoError(t, err)

			check := p.HasKibanaVersion(kibanaVersion)
			assert.Equal(t, tt.check, check)

		})
	}
}

func TestNewPackageFromPath(t *testing.T) {
	packagePath := "../testdata/package/reference/1.0.0"
	absPath, err := filepath.Abs(packagePath)
	require.NoError(t, err)

	cases := []struct {
		title string
		path  string
	}{
		{
			title: "relative path",
			path:  packagePath,
		},
		{
			title: "relative path with slash",
			path:  packagePath + "/",
		},
		{
			title: "absolute path",
			path:  absPath,
		},
		{
			title: "absolute path with slash",
			path:  absPath + "/",
		},
	}

	fsBuilder := func(p *Package) (PackageFileSystem, error) {
		return NewExtractedPackageFileSystem(p)
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			_, err := NewPackage(c.path, fsBuilder)
			assert.NoError(t, err)
		})
	}
}

func TestIsPrerelease(t *testing.T) {
	cases := []struct {
		version    string
		prerelease bool
	}{
		{"0.1.0-rc1", true},
		{"0.1.0", true}, // Major version 0 shouldn't be considered stable.
		{"1.0.0-beta1", true},
		{"1.0.0-rc.1", true},
		{"1.0.0-SNAPSHOT", true},
		{"1.0.0", false},
	}

	for _, c := range cases {
		t.Run(c.version, func(t *testing.T) {
			semver := semver.MustParse(c.version)
			assert.Equal(t, c.prerelease, isPrerelease(semver))
		})
	}
}

func BenchmarkNewPackage(b *testing.B) {
	fsBuilder := func(p *Package) (PackageFileSystem, error) {
		return NewExtractedPackageFileSystem(p)
	}
	for i := 0; i < b.N; i++ {
		_, err := NewPackage("../testdata/package/reference/1.0.0", fsBuilder)
		assert.NoError(b, err)
	}
}

func BenchmarkNewZipPackage(b *testing.B) {
	fsBuilder := func(p *Package) (PackageFileSystem, error) {
		return NewZipPackageFileSystem(p)
	}
	for i := 0; i < b.N; i++ {
		_, err := NewPackage("../testdata/local-storage/example-1.0.1.zip", fsBuilder)
		assert.NoError(b, err)
	}
}
