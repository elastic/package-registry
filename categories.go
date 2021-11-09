// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	"go.elastic.co/apm"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/util"
)

type Category struct {
	Id    string `yaml:"id" json:"id"`
	Title string `yaml:"title" json:"title"`
	Count int    `yaml:"count" json:"count"`
}

// categoriesHandler is a dynamic handler as it will also allow filtering in the future.
func categoriesHandler(indexer Indexer, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		filter, err := newCategoriesFilterFromQuery(query)
		if err != nil {
			badRequest(w, err.Error())
			return
		}

		includePolicyTemplates := false
		if v := query.Get("include_policy_templates"); v != "" {
			includePolicyTemplates, err = strconv.ParseBool(v)
			if err != nil {
				badRequest(w, fmt.Sprintf("invalid 'include_policy_templates' query param: '%s'", v))
				return
			}
		}

		opts := packages.GetOptions{
			Filter: filter,
		}
		packages, err := indexer.Get(r.Context(), &opts)
		if err != nil {
			notFoundError(w, err)
			return
		}

		categories := getCategories(r.Context(), packages, includePolicyTemplates)

		data, err := getCategoriesOutput(r.Context(), categories)
		if err != nil {
			notFoundError(w, err)
			return
		}

		cacheHeaders(w, cacheTime)
		jsonHeader(w)
		w.Write(data)
	}
}

func newCategoriesFilterFromQuery(query url.Values) (*packages.Filter, error) {
	var filter packages.Filter

	if len(query) == 0 {
		return &filter, nil
	}

	var err error
	for _, v := range query["kibana.version"] {
		parsed, err := semver.NewVersion(v)
		if err != nil {
			return nil, fmt.Errorf("invalid Kibana version '%s': %w", v, err)
		}
		filter.KibanaVersions = append(filter.KibanaVersions, parsed)
	}

	if v := query.Get("experimental"); v != "" {
		filter.Experimental, err = strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid 'experimental' query param: '%s'", v)
		}
	}

	return &filter, nil
}

func getCategories(ctx context.Context, packages packages.Packages, includePolicyTemplates bool) map[string]*Category {
	span, ctx := apm.StartSpan(ctx, "FilterCategories", "app")
	defer span.End()

	categories := map[string]*Category{}

	for _, p := range packages {
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
			// /categories counts policies and packages separately, but packages are counted too
			// if they don't match but any of their policies does (for the AWS case this would mean that
			// the count for "datastore" would be 3: the Package and the RDS and DynamoDB policies).
			var extraPackageCategories []string

			for _, t := range p.PolicyTemplates {
				// Skip when policy template level `categories` is empty and there is only one policy template
				if t.Categories == nil && len(p.PolicyTemplates) == 1 {
					break
				}

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

					if !p.HasCategory(c) && !util.StringsContains(extraPackageCategories, c) {
						extraPackageCategories = append(extraPackageCategories, c)
						categories[c].Count = categories[c].Count + 1
					}

					categories[c].Count = categories[c].Count + 1
				}
			}
		}
	}

	return categories
}

func getCategoriesOutput(ctx context.Context, categories map[string]*Category) ([]byte, error) {
	span, ctx := apm.StartSpan(ctx, "GetCategoriesOutput", "app")
	defer span.End()

	var keys []string
	for k := range categories {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var outputCategories []*Category
	for _, k := range keys {
		c := categories[k]
		if title, ok := packages.CategoryTitles[c.Title]; ok {
			c.Title = title
		}
		outputCategories = append(outputCategories, c)
	}

	return util.MarshalJSONPretty(outputCategories)
}
