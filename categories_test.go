// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/proxymode"
)

func TestCategoriesWithProxyMode(t *testing.T) {
	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `[
  {
    "id": "custom",
    "title": "Custom",
    "count": 10
  },
  {
    "id": "custom_logs",
    "title": "Custom Logs",
    "count": 3,
    "parent_id": "custom",
    "parent_title": "Custom"
  }
]`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, response)
	}))
	defer webServer.Close()

	indexerProxy := packages.NewFileSystemIndexer(testLogger, "./testdata/second_package_path")
	err := indexerProxy.Init(context.Background())
	require.NoError(t, err)

	proxyMode, err := proxymode.NewProxyMode(
		testLogger,
		proxymode.ProxyOptions{
			Enabled: true,
			ProxyTo: webServer.URL,
		},
	)
	require.NoError(t, err)

	categoriesWithProxyHandler := categoriesHandlerWithProxyMode(testLogger, indexerProxy, proxyMode, testCacheTime)

	tests := []struct {
		endpoint string
		path     string
		file     string
		handler  func(w http.ResponseWriter, r *http.Request)
	}{
		{"/categories", "/categories", "categories-proxy.json", categoriesWithProxyHandler},
		{"/categories?kibana.version=6.5.0", "/categories", "categories-proxy-kibana-filter.json", categoriesWithProxyHandler},
	}

	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			runEndpoint(t, test.endpoint, test.path, test.file, test.handler)
		})
	}
}

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
