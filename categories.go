// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"go.elastic.co/apm/module/apmzap/v2"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/proxymode"
)

// categoriesHandler is a dynamic handler as it will also allow filtering in the future.
type categoriesHandler struct {
	logger    *zap.Logger
	indexer   Indexer
	cacheTime time.Duration

	cache     *expirable.LRU[string, []byte]
	proxyMode *proxymode.ProxyMode
}

type categoriesOption func(h *categoriesHandler)

func newCategoriesHandler(logger *zap.Logger, indexer Indexer, cacheTime time.Duration, opts ...categoriesOption) (*categoriesHandler, error) {
	if indexer == nil {
		return nil, errors.New("indexer is required for categories handler")
	}
	if cacheTime <= 0 {
		return nil, errors.New("cache time must be greater than 0s")
	}

	h := &categoriesHandler{
		logger:    logger,
		indexer:   indexer,
		cacheTime: cacheTime,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h, nil
}

func categoriesWithProxy(pm *proxymode.ProxyMode) categoriesOption {
	return func(h *categoriesHandler) {
		h.proxyMode = pm
	}
}

func categoriesWithCache(cache *expirable.LRU[string, []byte]) categoriesOption {
	return func(h *categoriesHandler) {
		h.cache = cache
	}
}

func (h *categoriesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(apmzap.TraceContext(r.Context())...)

	if h.cache != nil {
		if response, ok := h.cache.Get(r.URL.String()); ok {
			logger.Debug("using as response cached request", zap.String("cache.url", r.URL.String()), zap.Int("cache.size", h.cache.Len()))
			serveJSONResponse(r.Context(), w, h.cacheTime, response)
			return
		}
	}

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
	pkgs, err := h.indexer.Get(r.Context(), &opts)
	if err != nil {
		notFoundError(w, err)
		return
	}
	categories := getCategories(r.Context(), pkgs, includePolicyTemplates)

	if h.proxyMode.Enabled() {
		proxiedCategories, err := h.proxyMode.Categories(r)
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

	serveJSONResponse(r.Context(), w, h.cacheTime, data)

	if h.cache != nil {
		val := h.cache.Add(r.URL.String(), data)
		logger.Debug("added to cache request", zap.String("cache.url", r.URL.String()), zap.Int("cache.size", h.cache.Len()), zap.Bool("cache.eviction", val))
	}
}

func newCategoriesFilterFromQuery(query url.Values) (*packages.Filter, error) {
	var filter packages.Filter

	if len(query) == 0 {
		return &filter, nil
	}

	var err error
	for key, values := range query {
		// Same behavior as query.Get(key) to keep compatibility with previous versions
		v := ""
		if len(values) > 0 {
			v = values[0]
		}
		switch key {
		case "kibana.version":
			if v != "" {
				filter.KibanaVersion, err = semver.NewVersion(v)
				if err != nil {
					return nil, fmt.Errorf("invalid Kibana version '%s': %w", v, err)
				}
			}
		case "experimental":
			// Deprecated: release tags to be removed
			if v != "" {
				filter.Experimental, err = strconv.ParseBool(v)
				if err != nil {
					return nil, fmt.Errorf("invalid 'experimental' query param: '%s'", v)
				}
			}
		case "prerelease":
			if v != "" {
				filter.Prerelease, err = strconv.ParseBool(v)
				if err != nil {
					return nil, fmt.Errorf("invalid 'prerelease' query param: '%s'", v)
				}
			}
		case "capabilities":
			if v != "" {
				filter.Capabilities = strings.Split(v, ",")
			}
		case "spec.min":
			if v != "" {
				filter.SpecMin, err = getSpecVersion(v)
				if err != nil {
					return nil, fmt.Errorf("invalid 'spec.min' version: %w", err)
				}
			}
		case "spec.max":
			if v != "" {
				filter.SpecMax, err = getSpecVersion(v)
				if err != nil {
					return nil, fmt.Errorf("invalid 'spec.max' version: %w", err)
				}
			}
		case "discovery":
			for _, v := range values {
				discovery, err := packages.NewDiscoveryFilter(v)
				if err != nil {
					return nil, fmt.Errorf("invalid 'discovery' query param: '%s': %w", v, err)
				}
				filter.Discovery = append(filter.Discovery, discovery)
			}
		case "include_policy_templates":
			// This query parameter is allowed, but not used as a filter
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

					if !p.HasCategory(c) && !slices.Contains(extraPackageCategories, c) {
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

	outputCategories := []*packages.Category{}
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

func serveJSONResponse(ctx context.Context, w http.ResponseWriter, cacheTime time.Duration, data []byte) {
	span, _ := apm.StartSpan(ctx, "Serve JSON Response", "app")
	defer span.End()

	cacheHeaders(w, cacheTime)
	jsonHeader(w)
	w.Write(data)
}
