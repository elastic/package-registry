// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var (
	title = "foo"
)

func TestValidate(t *testing.T) {
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
					Version:    "1.2.3",
				},
				FormatVersion: "1.0.0",
			},
			true,
			"unknown category",
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

	logger := zap.Must(zap.NewDevelopment())
	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			_, err := NewPackage(logger, c.path, fsBuilder)
			assert.NoError(t, err)
		})
	}
}

func TestMustParsePackageFromPath(t *testing.T) {
	cases := []struct {
		title         string
		path          string
		zip           bool
		expectedError bool
	}{
		{
			title:         "unknown category",
			path:          "../testdata/local-storage/nodirentries-1.0.0.zip",
			zip:           true,
			expectedError: true,
		},
		{
			title:         "valid package",
			path:          "../testdata/package/reference/1.0.0",
			zip:           false,
			expectedError: false,
		},
	}

	zipFsBuilder := func(p *Package) (PackageFileSystem, error) {
		return NewZipPackageFileSystem(p)
	}
	fsBuilder := func(p *Package) (PackageFileSystem, error) {
		return NewExtractedPackageFileSystem(p)
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			builder := fsBuilder
			if c.zip {
				builder = zipFsBuilder
			}
			_, err := MustParsePackage(c.path, builder)
			if c.expectedError {
				assert.Error(t, err)
				return
			}
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
	logger := zap.Must(zap.NewDevelopment())
	for b.Loop() {
		_, err := NewPackage(logger, "../testdata/package/reference/1.0.0", fsBuilder)
		assert.NoError(b, err)
	}
}

func BenchmarkNewZipPackage(b *testing.B) {
	fsBuilder := func(p *Package) (PackageFileSystem, error) {
		return NewZipPackageFileSystem(p)
	}
	logger := zap.Must(zap.NewDevelopment())
	for b.Loop() {
		_, err := NewPackage(logger, "../testdata/local-storage/example-1.0.1.zip", fsBuilder)
		assert.NoError(b, err)
	}
}

func TestPackageSetRuntimeFields(t *testing.T) {
	p := &Package{
		FormatVersion: "3.5.0",
		BasePackage: BasePackage{
			Version: "3.6.0",
			Conditions: &Conditions{
				Kibana: &KibanaConditions{
					Version: "^8.5.0",
				},
				Agent: &AgentConditions{
					Version: "^8.5.0",
				},
			},
		},
	}

	expectedVersion, err := semver.NewVersion("3.6.0")
	require.NoError(t, err)
	expectedKibanaConstraint, err := semver.NewConstraint("^8.5.0")
	require.NoError(t, err)
	expectedAgentConstraint, err := semver.NewConstraint("^8.5.0")
	require.NoError(t, err)

	err = p.setRuntimeFields()
	require.NoError(t, err)
	require.NotNil(t, p.versionSemVer)
	require.NotNil(t, p.Conditions.Kibana.constraint)
	require.NotNil(t, p.Conditions.Agent.constraint)
	require.NotNil(t, p.specMajorMinorSemVer)

	assert.Equal(t, expectedVersion, p.versionSemVer)
	assert.Equal(t, expectedKibanaConstraint, p.Conditions.Kibana.constraint)
	assert.Equal(t, expectedAgentConstraint, p.Conditions.Agent.constraint)
	assert.Equal(t, "3.5.0", p.specMajorMinorSemVer.String())
}
