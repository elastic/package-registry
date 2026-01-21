// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLatestDeprecatedNoticeFromPackages(t *testing.T) {
	newPackage := func(name string, version string, deprecated *Deprecated) *Package {
		p := new(Package)
		p.Name = name
		p.Version = version
		p.Deprecated = deprecated
		return p
	}

	deprecatedNotice := &Deprecated{
		Description: "This package is deprecated",
	}

	cases := []struct {
		title    string
		packages Packages
		expected DeprecatedPackages
	}{
		{
			title:    "empty package list",
			packages: Packages{},
			expected: DeprecatedPackages{},
		},
		{
			title: "single deprecated package",
			packages: Packages{
				newPackage("foo", "1.0.0", deprecatedNotice),
			},
			expected: DeprecatedPackages{
				"foo": *deprecatedNotice,
			},
		},
		{
			title: "single non-deprecated package",
			packages: Packages{
				newPackage("foo", "1.0.0", nil),
			},
			expected: DeprecatedPackages{},
		},
		{
			title: "multiple versions, latest is deprecated",
			packages: Packages{
				newPackage("foo", "1.0.0", nil),
				newPackage("foo", "2.0.0", deprecatedNotice),
			},
			expected: DeprecatedPackages{
				"foo": *deprecatedNotice,
			},
		},
		{
			// this case would be rare in practice, but we want to ensure the function behaves correctly
			// even if an older version is deprecated while a newer one is not
			title: "multiple versions, older version is deprecated",
			packages: Packages{
				newPackage("foo", "1.0.0", deprecatedNotice),
				newPackage("foo", "2.0.0", nil),
			},
			expected: DeprecatedPackages{
				"foo": *deprecatedNotice,
			},
		},
		{
			title: "multiple packages with different deprecation states",
			packages: Packages{
				newPackage("foo", "1.0.0", deprecatedNotice),
				newPackage("bar", "1.0.0", nil),
				newPackage("baz", "1.0.0", &Deprecated{Description: "Different notice"}),
			},
			expected: DeprecatedPackages{
				"foo": *deprecatedNotice,
				"baz": Deprecated{Description: "Different notice"},
			},
		},
		{
			// deprecation notice can be modified while the package is in maintenance mode
			title: "unsorted packages, latest deprecated version should be used",
			packages: Packages{
				newPackage("foo", "1.0.0", &Deprecated{Description: "Old notice"}),
				newPackage("foo", "3.0.0", nil),
				newPackage("foo", "2.0.0", &Deprecated{Description: "Latest deprecated notice"}),
			},
			expected: DeprecatedPackages{
				"foo": Deprecated{Description: "Latest deprecated notice"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			result := GetLatestDeprecatedNoticeFromPackages(c.packages)
			assert.Equal(t, c.expected, result)
		})
	}
}
