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
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"go.elastic.co/apm/module/apmzap/v2"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/internal/util"
	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/proxymode"
)

func detectionRulesHandler(logger *zap.Logger, indexer Indexer, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return detectionRulesHandlerWithProxyMode(logger, indexer, proxymode.NoProxy(logger), cacheTime)
}

func detectionRulesHandlerWithProxyMode(logger *zap.Logger, indexer Indexer, proxyMode *proxymode.ProxyMode, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := logger.With(apmzap.TraceContext(r.Context())...)

		filter, err := newSearchFilterFromQuery(r.URL.Query())
		if err != nil {
			badRequest(w, err.Error())
			return
		}
		opts := packages.GetOptions{
			Filter: filter,
		}

		ps, err := indexer.Get(r.Context(), &opts)
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
			ps = ps.Join(proxiedPackages)
			if !opts.Filter.AllVersions {
				ps = latestPackagesVersion(ps)
			}
		}
		relevantPackages := packages.Packages{}
		for _, p := range ps {
			potential_file_paths := []string{}
			ds := p.DataStreams
			for _, s := range ds {
				streams := s.Streams
				for _, stream := range streams {
					vars := stream.Vars
					for _, v := range vars {
						// if the name of the variable contains path and the default value is a string, add the default value to the list
						if strings.Contains(v.Name, "path") {
							if defaultValue, ok := v.Default.(string); ok {
								potential_file_paths = append(potential_file_paths, defaultValue)
							}
							// if default value is an array, iterate over the array and add each value to the list
							if defaultValue, ok := v.Default.([]interface{}); ok {
								for _, value := range defaultValue {
									if value, ok := value.(string); ok {
										potential_file_paths = append(potential_file_paths, value)
									}
								}
							}
						}
					}
				}
			}
			// iterate over the potential file paths and add a detection rule for each
			for _, path := range potential_file_paths {
				// create the detection rule
				detection_rule := packages.DetectionRule{
					Query:    "file_handle",
					Contents: path,
				}
				p.DetectionRules = append(p.DetectionRules, detection_rule)
			}
			if len(p.DetectionRules) > 0 {
				relevantPackages = append(relevantPackages, p)
			}

		}

		// add hardcoded custom package to list
		data, err := getPackageDetectionOutput(r.Context(), relevantPackages)
		if err != nil {
			notFoundError(w, err)
			return
		}

		cacheHeaders(w, cacheTime)
		jsonHeader(w)
		fmt.Fprint(w, string(data))
	}
}

func _newSearchFilterFromQuery(query url.Values) (*packages.Filter, error) {
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

	return &filter, nil
}

func _getSpecVersion(version string) (*semver.Version, error) {
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

func getPackageDetectionOutput(ctx context.Context, packageList packages.Packages) ([]byte, error) {
	span, _ := apm.StartSpan(ctx, "GetPackageOutput", "app")
	defer span.End()

	// Packages need to be sorted to be always outputted in the same order
	sort.Sort(packageList)

	var output []packages.DetectionRulePackage
	for _, p := range packageList {
		output = append(output, packages.DetectionRulePackage{
			Name:           p.Name,
			DetectionRules: p.DetectionRules,
		})
	}

	output = append(output, packages.DetectionRulePackage{
		Name: "custom",
		DetectionRules: []packages.DetectionRule{
			{
				Query:    "file_handle",
				Contents: "*log",
			},
		},
	})

	// Instead of return `null` in case of an empty array, return []
	if len(output) == 0 {
		return []byte("[]"), nil
	}

	return util.MarshalJSONPretty(output)
}
