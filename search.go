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

	"github.com/Masterminds/semver/v3"

	"go.elastic.co/apm/module/apmzap/v2"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/proxymode"
)

func searchHandler(logger *zap.Logger, options handlerOptions) (func(w http.ResponseWriter, r *http.Request), error) {
	options.proxyMode = proxymode.NoProxy(logger)
	return searchHandlerWithProxyMode(logger, options)
}

func searchHandlerWithProxyMode(logger *zap.Logger, options handlerOptions) (func(w http.ResponseWriter, r *http.Request), error) {
	if options.proxyMode == nil {
		logger.Warn("packageIndexHandlerWithProxyMode called without proxy mode, defaulting to no proxy")
		options.proxyMode = proxymode.NoProxy(logger)
	}
	if options.cacheTime < 0 {
		return nil, fmt.Errorf("cache time must be non-negative for search handler")
	}
	if options.indexer == nil {
		return nil, fmt.Errorf("indexer is required for search handler")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		logger := logger.With(apmzap.TraceContext(r.Context())...)

		if options.cache != nil {
			if response, ok := options.cache.Get(r.URL.String()); ok {
				logger.Debug("using as response cached search request", zap.String("cache.url", r.URL.String()), zap.Int("cache.size", options.cache.Len()))
				serveJSONResponse(r.Context(), w, options.cacheTime, response)
				return
			}
		}

		filter, err := newSearchFilterFromQuery(r.URL.Query(), options.allowUnknownQueryParameters)
		if err != nil {
			badRequest(w, err.Error())
			return
		}
		opts := packages.GetOptions{
			Filter: filter,
		}

		packages, err := options.indexer.Get(r.Context(), &opts)
		if err != nil {
			notFoundError(w, fmt.Errorf("fetching package failed: %w", err))
			return
		}

		if options.proxyMode.Enabled() {
			proxiedPackages, err := options.proxyMode.Search(r)
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

		serveJSONResponse(r.Context(), w, options.cacheTime, data)

		if options.cache != nil {
			val := options.cache.Add(r.URL.String(), data)
			logger.Debug("added to cache request", zap.String("cache.url", r.URL.String()), zap.Int("cache.size", options.cache.Len()), zap.Bool("cache.eviction", val))
		}
	}, nil
}

func newSearchFilterFromQuery(query url.Values, allowUnknownQueryParameters bool) (*packages.Filter, error) {
	var filter packages.Filter

	if len(query) == 0 {
		return &filter, nil
	}

	var err error
	for key, values := range query {
		if len(values) == 0 {
			continue // Skip empty values for backward compatibility without returning an error
		}
		v := values[0]
		switch key {
		case "kibana.version":
			if v != "" {
				filter.KibanaVersion, err = semver.NewVersion(v)
				if err != nil {
					return nil, fmt.Errorf("invalid Kibana version '%s': %w", v, err)
				}
			}
		case "category":
			if v != "" {
				filter.Category = v
			}
		case "package":
			if v != "" {
				filter.PackageName = v
			}
		case "type":
			if v != "" {
				filter.PackageType = v
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
		case "all":
			if v != "" {
				filter.AllVersions, err = strconv.ParseBool(v)
				if err != nil {
					return nil, fmt.Errorf("invalid 'all' query param: '%s'", v)
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
		case "discovery":
			for _, v := range values {
				discovery, err := packages.NewDiscoveryFilter(v)
				if err != nil {
					return nil, fmt.Errorf("invalid 'discovery' query param: '%s': %w", v, err)
				}
				filter.Discovery = append(filter.Discovery, discovery)
			}
		case "internal":
			// Parameter removed in https://github.com/elastic/package-registry/pull/765
			// Keep it here to avoid breaking existing clients.
		default:
			if !allowUnknownQueryParameters {
				return nil, fmt.Errorf("unknown query parameter: %q", key)
			}
		}
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
