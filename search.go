// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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

func searchHandler(logger *zap.Logger, indexer Indexer, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return searchHandlerWithProxyMode(logger, indexer, proxymode.NoProxy(logger), cacheTime, nil)
}

func searchHandlerWithProxyMode(logger *zap.Logger, indexer Indexer, proxyMode *proxymode.ProxyMode, cacheTime time.Duration, cache *expirable.LRU[string, []byte]) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := logger.With(apmzap.TraceContext(r.Context())...)

		if cache != nil {
			if response, ok := cache.Get(r.URL.String()); ok {
				logger.Debug("using as response cached search request", zap.String("cache.url", r.URL.String()), zap.Int("cache.size", cache.Len()))
				cacheHeaders(w, cacheTime)
				jsonHeader(w)
				w.Write(response)
				return
			}
		}

		filter, err := newSearchFilterFromQuery(r.URL.Query())
		if err != nil {
			badRequest(w, err.Error())
			return
		}
		opts := packages.GetOptions{
			Filter: filter,
		}

		packages, err := indexer.Get(r.Context(), &opts)
		if err != nil {
			notFoundError(w, fmt.Errorf("fetching package failed: %w", err))
			return
		}

		if proxyMode.Enabled() {
			proxiedPackages, err := proxyMode.Search(r)
			if err != nil {
				logger.Error("proxy mode: search failed", zap.Error(err))
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			packages = packages.Join(proxiedPackages)
			if !opts.Filter.AllVersions {
				packages = latestPackagesVersion(packages)
			}
		}

		data, err := getSearchOutput(r.Context(), packages)
		if err != nil {
			notFoundError(w, err)
			return
		}

		cacheHeaders(w, cacheTime)
		jsonHeader(w)
		w.Write(data)

		if cache != nil {
			val := cache.Add(r.URL.String(), data)
			logger.Debug("added to cache request", zap.String("cache.url", r.URL.String()), zap.Int("cache.size", cache.Len()), zap.Bool("cache.added", val))
		}
	}
}

func newSearchFilterFromQuery(query url.Values) (*packages.Filter, error) {
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

	if v := query.Get("category"); v != "" {
		filter.Category = v
	}

	if v := query.Get("package"); v != "" {
		filter.PackageName = v
	}

	if v := query.Get("type"); v != "" {
		filter.PackageType = v
	}

	if v := query.Get("capabilities"); v != "" {
		filter.Capabilities = strings.Split(v, ",")
	}

	if v := query.Get("spec.min"); v != "" {
		filter.SpecMin, err = getSpecVersion(v)
		if err != nil {
			return nil, fmt.Errorf("invalid 'spec.min' version: %w", err)
		}
	}

	if v := query.Get("spec.max"); v != "" {
		filter.SpecMax, err = getSpecVersion(v)
		if err != nil {
			return nil, fmt.Errorf("invalid 'spec.max' version: %w", err)
		}
	}

	if v := query.Get("all"); v != "" {
		// Default is false, also on error
		filter.AllVersions, err = strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid 'all' query param: '%s'", v)
		}
	}

	// Deprecated: release tags to be removed.
	if v := query.Get("experimental"); v != "" {
		// In case of error, keep it false
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

	for _, v := range query["discovery"] {
		discovery, err := packages.NewDiscoveryFilter(v)
		if err != nil {
			return nil, fmt.Errorf("invalid 'discovery' query param: '%s': %w", v, err)
		}
		filter.Discovery = append(filter.Discovery, discovery)
	}

	return &filter, nil
}

func getSpecVersion(version string) (*semver.Version, error) {
	// version must cointain just <major.minor>
	if len(strings.Split(version, ".")) != 2 {
		return nil, fmt.Errorf("invalid version '%s': it should be <major.version>", version)
	}
	specVersion, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("invalid spec version '%s': %w", version, err)
	}
	return specVersion, nil
}

func getSearchOutput(ctx context.Context, packageList packages.Packages) ([]byte, error) {
	span, _ := apm.StartSpan(ctx, "GetPackageOutput", "app")
	defer span.End()

	// Packages need to be sorted to be always outputted in the same order
	sort.Sort(packageList)

	var output []packages.BasePackage
	for _, p := range packageList {
		output = append(output, p.BasePackage)
	}

	// Instead of return `null` in case of an empty array, return []
	if len(output) == 0 {
		return []byte("[]"), nil
	}

	return util.MarshalJSONPretty(output)
}
