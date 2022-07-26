// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/package-registry/packages"
)

func TestLatestPackagesVersion(t *testing.T) {
	newPackage := func(name string, version string) *packages.Package {
		p := new(packages.Package)
		p.Name = name
		p.Version = version
		return p
	}

	cases := []struct {
		title    string
		source   packages.Packages
		expected packages.Packages
	}{
		{
			title: "single package",
			source: packages.Packages{
				newPackage("foo", "1.2.3"),
			},
			expected: packages.Packages{
				newPackage("foo", "1.2.3"),
			},
		},
		{
			title: "single package sorted versions",
			source: packages.Packages{
				newPackage("foo", "1.2.3"),
				newPackage("foo", "1.2.2"),
				newPackage("foo", "1.2.1"),
			},
			expected: packages.Packages{
				newPackage("foo", "1.2.3"),
			},
		},
		{
			title: "single package unsorted versions",
			source: packages.Packages{
				newPackage("foo", "1.2.2"),
				newPackage("foo", "1.2.1"),
				newPackage("foo", "1.2.3"),
			},
			expected: packages.Packages{
				newPackage("foo", "1.2.3"),
			},
		},
		{
			title: "multiple packages with multiple versions",
			source: packages.Packages{
				newPackage("foo", "1.2.2"),
				newPackage("bar", "1.2.1"),
				newPackage("bar", "1.2.2"),
				newPackage("foo", "1.2.1"),
				newPackage("bar", "1.2.3"),
				newPackage("foo", "1.2.3"),
			},
			expected: packages.Packages{
				newPackage("bar", "1.2.3"),
				newPackage("foo", "1.2.3"),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			result := latestPackagesVersion(c.source)
			assert.EqualValues(t, c.expected, result)
		})
	}

}
