// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLatestPackagesVersion(t *testing.T) {
	newPackage := func(name string, version string) *Package {
		p := new(Package)
		p.Name = name
		p.Version = version
		return p
	}

	cases := []struct {
		title    string
		source   Packages
		expected Packages
	}{
		{
			title: "single package",
			source: Packages{
				newPackage("foo", "1.2.3"),
			},
			expected: Packages{
				newPackage("foo", "1.2.3"),
			},
		},
		{
			title: "single package sorted versions",
			source: Packages{
				newPackage("foo", "1.2.3"),
				newPackage("foo", "1.2.2"),
				newPackage("foo", "1.2.1"),
			},
			expected: Packages{
				newPackage("foo", "1.2.3"),
			},
		},
		{
			title: "single package unsorted versions",
			source: Packages{
				newPackage("foo", "1.2.2"),
				newPackage("foo", "1.2.1"),
				newPackage("foo", "1.2.3"),
			},
			expected: Packages{
				newPackage("foo", "1.2.3"),
			},
		},
		{
			title: "multiple packages with multiple versions",
			source: Packages{
				newPackage("foo", "1.2.2"),
				newPackage("bar", "1.2.1"),
				newPackage("bar", "1.2.2"),
				newPackage("foo", "1.2.1"),
				newPackage("bar", "1.2.3"),
				newPackage("foo", "1.2.3"),
			},
			expected: Packages{
				newPackage("bar", "1.2.3"),
				newPackage("foo", "1.2.3"),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			result := LatestPackagesVersion(c.source)
			assert.EqualValues(t, c.expected, result)
		})
	}

}

func TestPropagateDeprecatedInfoToAllVersions(t *testing.T) {
	t.Run("", func(t *testing.T) {
		
	})
}
