// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/package-registry/packages"
)

func TestGetCategories(t *testing.T) {
	filterTestPackages := []filterCategoryTestPackage{
		{
			Name:    "mypackage",
			Version: "1.0.0",
			Categories: []string{
				"observability",
				"network",
			},
		},
		{
			Name:    "foo",
			Version: "1.1.0",
			Categories: []string{
				"observability",
				"security",
			},
			PolicyTemplateCategories: [][]string{
				[]string{
					"network",
				},
			},
		},
		{
			Name:    "web",
			Version: "2.0.0-rc2",
			Categories: []string{
				"web",
			},
			PolicyTemplateCategories: [][]string{
				[]string{
					"security",
				},
				[]string{
					"other",
				},
			},
		},
		{
			Name:    "redisenterprise",
			Version: "1.1.0",
		},
	}

	pkgs := buildFilterTestPackages(filterTestPackages)

	cases := []struct {
		Title                  string
		IncludePolicyTemplates bool
		Expected               map[string]*packages.Category
	}{
		{
			Title:                  "All categories without policy templates",
			IncludePolicyTemplates: false,
			Expected: map[string]*packages.Category{
				"network": &packages.Category{
					Id:    "network",
					Title: "network",
					Count: 1,
				},
				"observability": &packages.Category{
					Id:    "observability",
					Title: "observability",
					Count: 2,
				},
				"security": &packages.Category{
					Id:    "security",
					Title: "security",
					Count: 1,
				},
				"web": &packages.Category{
					Id:    "web",
					Title: "web",
					Count: 1,
				},
			},
		},
		{
			Title:                  "All categories including policies",
			IncludePolicyTemplates: true,
			Expected: map[string]*packages.Category{
				"network": &packages.Category{
					Id:    "network",
					Title: "network",
					Count: 3,
				},
				"observability": &packages.Category{
					Id:    "observability",
					Title: "observability",
					Count: 3,
				},
				"security": &packages.Category{
					Id:    "security",
					Title: "security",
					Count: 4,
				},
				"web": &packages.Category{
					Id:    "web",
					Title: "web",
					Count: 3,
				},
				"other": &packages.Category{
					Id:    "other",
					Title: "other",
					Count: 2,
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Title, func(t *testing.T) {
			result := getCategories(context.Background(), pkgs, c.IncludePolicyTemplates)
			assert.Equal(t, c.Expected, result)
		})
	}
}

type filterCategoryTestPackage struct {
	Name                     string
	Version                  string
	Categories               []string
	PolicyTemplateCategories [][]string
}

func (p filterCategoryTestPackage) Build() *packages.Package {
	var build packages.Package
	build.Name = p.Name
	build.Version = p.Version
	build.Categories = p.Categories

	if p.PolicyTemplateCategories != nil {
		for _, categories := range p.PolicyTemplateCategories {
			p := packages.PolicyTemplate{
				Categories: categories,
			}
			build.PolicyTemplates = append(build.PolicyTemplates, p)
		}
	}

	return &build
}

func buildFilterTestPackages(testPackages []filterCategoryTestPackage) packages.Packages {
	packages := make(packages.Packages, len(testPackages))
	for i, p := range testPackages {
		packages[i] = p.Build()
	}
	return packages
}
