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

	"go.uber.org/zap"

	"github.com/Masterminds/semver/v3"
	"go.elastic.co/apm/module/apmzap/v2"
	"go.elastic.co/apm/v2"

	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/proxymode"
)

// categoriesHandler is a dynamic handler as it will also allow filtering in the future.
func categoriesHandler(logger *zap.Logger, indexer Indexer, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return categoriesHandlerWithProxyMode(logger, indexer, proxymode.NoProxy(logger), cacheTime)
}

// categoriesHandler is a dynamic handler as it will also allow filtering in the future.
func categoriesHandlerWithProxyMode(logger *zap.Logger, indexer Indexer, proxyMode *proxymode.ProxyMode, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := logger.With(apmzap.TraceContext(r.Context())...)

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
		pkgs, err := indexer.Get(r.Context(), &opts)
		if err != nil {
			notFoundError(w, err)
			return
		}
		categories := getCategories(r.Context(), pkgs, includePolicyTemplates)

		if proxyMode.Enabled() {
			proxiedCategories, err := proxyMode.Categories(r)
			if err != nil {
				logger.Error("proxy mode: categories failed", zap.Error(err))
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			for _, category := range proxiedCategories {
				if _, ok := categories[category.Id]; !ok {
					categories[category.Id] = &packages.Category{
						Id:          category.Id,
						Title:       category.Title,
						Count:       category.Count,
						ParentId:    category.ParentId,
						ParentTitle: category.ParentTitle,
					}
				} else {
					categories[category.Id].Count += category.Count
				}
			}
		}

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
	if v := query.Get("kibana.version"); v != "" {
		filter.KibanaVersion, err = semver.NewVersion(v)
		if err != nil {
			return nil, fmt.Errorf("invalid Kibana version '%s': %w", v, err)
		}
	}

	// Deprecated: release tags to be removed.
	if v := query.Get("experimental"); v != "" {
		filter.Experimental, err = strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid 'experimental' query param: '%s'", v)
		}
	}

	if v := query.Get("prerelease"); v != "" {
		// In case of error, keep it false
		filter.Prerelease, err = strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid 'prerelease' query param: '%s'", v)
		}
	}

	return &filter, nil
}

func getCategories(ctx context.Context, pkgs packages.Packages, includePolicyTemplates bool) map[string]*packages.Category {
	span, _ := apm.StartSpan(ctx, "FilterCategories", "app")
	defer span.End()

	categories := map[string]*packages.Category{}

	for _, p := range pkgs {
		for _, c := range p.Categories {
			if _, ok := categories[c]; !ok {
				categories[c] = &packages.Category{
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
						categories[c] = &packages.Category{
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

func getCategoriesOutput(ctx context.Context, categories map[string]*packages.Category) ([]byte, error) {
	span, _ := apm.StartSpan(ctx, "GetCategoriesOutput", "app")
	defer span.End()

	var keys []string
	for k := range categories {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var outputCategories []*packages.Category
	for _, k := range keys {
		c := categories[k]
		if category, ok := packages.Categories[c.Title]; ok {
			c.Title = category.Title
			if parent := category.Parent; parent != nil {
				c.ParentId = parent.Name
				c.ParentTitle = parent.Title
			}
		}
		outputCategories = append(outputCategories, c)
	}

	return util.MarshalJSONPretty(outputCategories)
}
