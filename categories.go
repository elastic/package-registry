// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/elastic/package-registry/util"
)

type Category struct {
	Id    string `yaml:"id" json:"id"`
	Title string `yaml:"title" json:"title"`
	Count int    `yaml:"count" json:"count"`
}

// categoriesHandler is a dynamic handler as it will also allow filtering in the future.
func categoriesHandler(packagesBasePaths []string, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		packages, err := util.GetPackages(packagesBasePaths)
		if err != nil {
			notFoundError(w, err)
			return
		}

		query := r.URL.Query()
		var experimental bool
		var includePolicyTemplates bool

		// Read query filter params which can affect the output
		if len(query) > 0 {
			if v := query.Get("experimental"); v != "" {
				experimental, err = strconv.ParseBool(v)
				if err != nil {
					badRequest(w, fmt.Sprintf("invalid 'experimental' query param: '%s'", v))
					return
				}
			}

			if v := query.Get("include_policy_templates"); v != "" {
				includePolicyTemplates, err = strconv.ParseBool(v)
				if err != nil {
					badRequest(w, fmt.Sprintf("invalid 'include_policy_templates' query param: '%s'", v))
					return
				}
			}
		}

		packageList := map[string]util.Package{}
		// Get unique list of newest packages
		for _, p := range packages {

			// Skip internal packages
			if p.Internal {
				continue
			}

			// Skip experimental packages if flag is not specified
			if p.Release == util.ReleaseExperimental && !experimental {
				continue
			}

			// Check if the version exists and if it should be added or not.
			// If the package in the list is newer or equal, do nothing.
			if pp, ok := packageList[p.Name]; ok && pp.IsNewerOrEqual(p) {
				continue
			}

			// Otherwise delete and later add the new one.
			packageList[p.Name] = p
		}

		categories := map[string]*Category{}

		for _, p := range packageList {
			for _, c := range p.Categories {
				if _, ok := categories[c]; !ok {
					categories[c] = &Category{
						Id:    c,
						Title: c,
						Count: 0,
					}
				}

				categories[c].Count = categories[c].Count + 1
			}

			if includePolicyTemplates {
				for _, t := range p.PolicyTemplates {
					for _, c := range p.Categories {
						categories[c].Count = categories[c].Count + 1
					}

					// Add policy template level categories.
					for _, c := range t.Categories {
						if _, ok := categories[c]; !ok {
							categories[c] = &Category{
								Id:    c,
								Title: c,
								Count: 0,
							}
						}

						categories[c].Count = categories[c].Count + 1
					}
				}
			}
		}

		data, err := getCategoriesOutput(categories)
		if err != nil {
			notFoundError(w, err)
			return
		}

		cacheHeaders(w, cacheTime)
		jsonHeader(w)
		fmt.Fprint(w, string(data))
	}
}

func getCategoriesOutput(categories map[string]*Category) ([]byte, error) {
	var keys []string
	for k := range categories {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var outputCategories []*Category
	for _, k := range keys {
		c := categories[k]
		if title, ok := util.CategoryTitles[c.Title]; ok {
			c.Title = title
		}
		outputCategories = append(outputCategories, c)
	}

	return json.MarshalIndent(outputCategories, "", "  ")
}
