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
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"

	"github.com/elastic/package-registry/util"
)

func searchHandler(packagesBasePaths []string, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		var kibanaVersion *semver.Version
		var category string
		// Leaving out `a` here to not use a reserved name
		var packageQuery string
		var all bool
		var internal bool
		var experimental bool
		var err error

		// Read query filter params which can affect the output
		if len(query) > 0 {
			if v := query.Get("kibana.version"); v != "" {
				kibanaVersion, err = semver.NewVersion(v)
				if err != nil {
					badRequest(w, fmt.Sprintf("invalid Kibana version '%s': %s", v, err))
					return
				}
			}

			if v := query.Get("category"); v != "" {
				category = v
			}

			if v := query.Get("package"); v != "" {
				packageQuery = v
			}

			if v := query.Get("all"); v != "" {
				// Default is false, also on error
				all, err = strconv.ParseBool(v)
				if err != nil {
					badRequest(w, fmt.Sprintf("invalid 'all' query param: '%s'", v))
					return
				}
			}

			if v := query.Get("internal"); v != "" {
				// In case of error, keep it false
				internal, err = strconv.ParseBool(v)
				if err != nil {
					badRequest(w, fmt.Sprintf("invalid 'internal' query param: '%s'", v))
					return
				}
			}

			if v := query.Get("experimental"); v != "" {
				// In case of error, keep it false
				experimental, err = strconv.ParseBool(v)
				if err != nil {
					badRequest(w, fmt.Sprintf("invalid 'experimental' query param: '%s'", v))
					return
				}
			}
		}

		packages, err := util.GetPackages(packagesBasePaths)
		if err != nil {
			notFoundError(w, errors.Wrapf(err, "fetching package failed"))
			return
		}
		packagesList := map[string]map[string]util.Package{}

		// Checks that only the most recent version of an integration is added to the list
		for _, p := range packages {

			// Skip internal packages by default
			if p.Internal && !internal {
				continue
			}

			// Skip experimental packages if flag is not specified
			if p.Release == util.ReleaseExperimental && !experimental {
				continue
			}

			// Filter by category first as this could heavily reduce the number of packages
			// It must happen before the version filtering as there only the newest version
			// is exposed and there could be an older package with more versions.
			if category != "" && !p.HasCategory(category) {
				continue
			}

			if kibanaVersion != nil {
				if valid := p.HasKibanaVersion(kibanaVersion); !valid {
					continue
				}
			}

			// If package Query is set, all versions of this package are returned
			if packageQuery != "" && packageQuery != p.Name {
				continue
			}

			if !all {
				// Check if the version exists and if it should be added or not.
				for _, versions := range packagesList {
					for _, pp := range versions {
						if pp.Name == p.Name {

							// If the package in the list is newer or equal, do nothing.
							if pp.IsNewerOrEqual(p) {
								continue
							}

							// Otherwise delete and later add the new one.
							delete(packagesList[pp.Name], pp.Version)
						}
					}
				}
			}

			if _, ok := packagesList[p.Name]; !ok {
				packagesList[p.Name] = map[string]util.Package{}
			}
			packagesList[p.Name][p.Version] = p
		}

		data, err := getPackageOutput(packagesList)
		if err != nil {
			notFoundError(w, err)
			return
		}

		cacheHeaders(w, cacheTime)
		jsonHeader(w)
		fmt.Fprint(w, string(data))
	}
}

func getPackageOutput(packagesList map[string]map[string]util.Package) ([]byte, error) {

	separator := "@"
	// Packages need to be sorted to be always outputted in the same order
	var keys []string
	for key, k := range packagesList {
		for v := range k {
			keys = append(keys, key+separator+v)
		}
	}
	sort.Strings(keys)

	var output []util.BasePackage

	for _, k := range keys {
		parts := strings.Split(k, separator)
		m := packagesList[parts[0]][parts[1]]
		data := m.BasePackage
		output = append(output, data)
	}

	// Instead of return `null` in case of an empty array, return []
	if len(output) == 0 {
		return []byte("[]"), nil
	}

	return json.MarshalIndent(output, "", "  ")
}
