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
	"github.com/pkg/errors"
	"go.elastic.co/apm"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/util"
)

func searchHandler(indexer Indexer, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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
			notFoundError(w, errors.Wrapf(err, "fetching package failed"))
			return
		}

		data, err := getPackageOutput(r.Context(), packages)
		if err != nil {
			notFoundError(w, err)
			return
		}

		cacheHeaders(w, cacheTime)
		jsonHeader(w)
		fmt.Fprint(w, string(data))
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

	if v := query.Get("elastic.subscription"); v != "" {
		filter.Subscription = v
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

		// For compatibility with older versions of Kibana.
		if filter.Experimental {
			filter.Prerelease = true
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

func getPackageOutput(ctx context.Context, packageList packages.Packages) ([]byte, error) {
	span, ctx := apm.StartSpan(ctx, "GetPackageOutput", "app")
	defer span.End()

	// Packages need to be sorted to be always outputted in the same order
	sort.Sort(packageList)

	var output []packages.BasePackage
	for _, p := range packageList {
		data := p.BasePackage
		output = append(output, data)
	}

	// Instead of return `null` in case of an empty array, return []
	if len(output) == 0 {
		return []byte("[]"), nil
	}

	return util.MarshalJSONPretty(output)
}
