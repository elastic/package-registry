// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

func TestUpdateLatestDeprecatedPackagesMapByName(t *testing.T) {
	tests := []struct {
		name               string
		input              Packages
		deprecatedPackages DeprecatedPackages
		expected           DeprecatedPackages
	}{
		{
			name:               "nil deprecated packages map is initialized",
			input:              Packages{},
			deprecatedPackages: nil,
			expected:           DeprecatedPackages{},
		},
		{
			name: "inits map and adds new deprecated package",
			input: Packages{
				&Package{
					BasePackage: BasePackage{
						Name:       "test-package",
						Deprecated: &Deprecated{},
					},
					versionSemVer: semver.MustParse("1.0.0"),
				},
			},
			deprecatedPackages: nil,
			expected: DeprecatedPackages{
				"test-package": deprecatedMeta{
					deprecated: &Deprecated{},
					version:    semver.MustParse("1.0.0"),
				},
			},
		},
		{
			name: "updates with newer version",
			input: Packages{
				{
					BasePackage: BasePackage{
						Name:       "test-package",
						Deprecated: &Deprecated{},
					},
					versionSemVer: semver.MustParse("2.0.0"),
				},
			},
			deprecatedPackages: DeprecatedPackages{
				"test-package": deprecatedMeta{
					deprecated: &Deprecated{},
					version:    semver.MustParse("1.0.0"),
				},
			},
			expected: DeprecatedPackages{
				"test-package": deprecatedMeta{
					deprecated: &Deprecated{},
					version:    semver.MustParse("2.0.0"),
				},
			},
		},
		{
			name: "does not update with older version",
			input: Packages{
				&Package{
					BasePackage: BasePackage{
						Name:       "test-package",
						Deprecated: &Deprecated{},
					},
					versionSemVer: semver.MustParse("1.0.0"),
				},
			},
			deprecatedPackages: DeprecatedPackages{
				"test-package": deprecatedMeta{
					deprecated: &Deprecated{},
					version:    semver.MustParse("2.0.0"),
				},
			},
			expected: DeprecatedPackages{
				"test-package": deprecatedMeta{
					deprecated: &Deprecated{},
					version:    semver.MustParse("2.0.0"),
				},
			},
		},
		{
			name: "ignores non-deprecated packages",
			input: Packages{
				{
					BasePackage: BasePackage{
						Name:       "test-package",
						Deprecated: nil,
					},
					versionSemVer: semver.MustParse("1.0.0"),
				},
			},
			deprecatedPackages: DeprecatedPackages{},
			expected:           DeprecatedPackages{},
		},
		{
			name: "handles multiple packages",
			input: Packages{
				{
					BasePackage: BasePackage{
						Name:       "package-a",
						Deprecated: &Deprecated{},
					},
					versionSemVer: semver.MustParse("1.0.0"),
				},
				{
					BasePackage: BasePackage{
						Name:       "package-b",
						Deprecated: &Deprecated{},
					},
					versionSemVer: semver.MustParse("2.0.0"),
				},
			},
			deprecatedPackages: DeprecatedPackages{},
			expected: DeprecatedPackages{
				"package-a": deprecatedMeta{
					deprecated: &Deprecated{},
					version:    semver.MustParse("1.0.0"),
				},
				"package-b": deprecatedMeta{
					deprecated: &Deprecated{},
					version:    semver.MustParse("2.0.0"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateLatestDeprecatedPackagesMapByName(tt.input, &tt.deprecatedPackages)
			assert.Equal(t, tt.expected, tt.deprecatedPackages)
		})
	}
}
func TestPropagateLatestDeprecatedInfoToPackageList(t *testing.T) {
	tests := []struct {
		name               string
		packageList        Packages
		deprecatedPackages DeprecatedPackages
		expected           Packages
	}{
		{
			name:               "empty package list",
			packageList:        Packages{},
			deprecatedPackages: DeprecatedPackages{},
			expected:           Packages{},
		},
		{
			name: "propagates deprecation info to matching package",
			packageList: Packages{
				&Package{
					BasePackage: BasePackage{
						Name:       "test-package",
						Deprecated: nil,
					},
				},
			},
			deprecatedPackages: DeprecatedPackages{
				"test-package": deprecatedMeta{
					deprecated: &Deprecated{},
					version:    semver.MustParse("1.0.0"),
				},
			},
			expected: Packages{
				&Package{
					BasePackage: BasePackage{
						Name:       "test-package",
						Deprecated: &Deprecated{},
					},
				},
			},
		},
		{
			name: "does not modify package without deprecation info",
			packageList: Packages{
				&Package{
					BasePackage: BasePackage{
						Name:       "test-package",
						Deprecated: nil,
					},
				},
			},
			deprecatedPackages: DeprecatedPackages{},
			expected: Packages{
				&Package{
					BasePackage: BasePackage{
						Name:       "test-package",
						Deprecated: nil,
					},
				},
			},
		},
		{
			name: "handles multiple packages with mixed deprecation",
			packageList: Packages{
				&Package{
					BasePackage: BasePackage{
						Name:       "deprecated-package",
						Deprecated: nil,
					},
				},
				&Package{
					BasePackage: BasePackage{
						Name:       "active-package",
						Deprecated: nil,
					},
				},
			},
			deprecatedPackages: DeprecatedPackages{
				"deprecated-package": deprecatedMeta{
					deprecated: &Deprecated{
						Since: "2.0.0",
					},
					version: semver.MustParse("2.0.0"),
				},
			},
			expected: Packages{
				&Package{
					BasePackage: BasePackage{
						Name: "deprecated-package",
						Deprecated: &Deprecated{
							Since: "2.0.0",
						},
					},
				},
				&Package{
					BasePackage: BasePackage{
						Name:       "active-package",
						Deprecated: nil,
					},
				},
			},
		},
		{
			name: "overwrites existing deprecation info",
			packageList: Packages{
				&Package{
					BasePackage: BasePackage{
						Name: "test-package",
						Deprecated: &Deprecated{
							Since: "1.0.0",
						},
					},
				},
			},
			deprecatedPackages: DeprecatedPackages{
				"test-package": deprecatedMeta{
					deprecated: &Deprecated{
						Since: "3.0.0",
					},
					version: semver.MustParse("3.0.0"),
				},
			},
			expected: Packages{
				&Package{
					BasePackage: BasePackage{
						Name: "test-package",
						Deprecated: &Deprecated{
							Since: "3.0.0",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			PropagateLatestDeprecatedInfoToPackageList(tt.packageList, tt.deprecatedPackages)
			for i, pkg := range tt.packageList {
				assert.Equal(t, tt.expected[i].Deprecated, pkg.Deprecated)
			}
		})
	}
}
