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

type Type struct {
	Name  string `yaml:"name" json:"name"`
	Title string `yaml:"title" json:"title"`
	Count int    `yaml:"count" json:"count"`
}

// typesHandler
func typesHandler(packagesBasePath string, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		packages, err := util.GetPackages(packagesBasePath)
		if err != nil {
			notFoundError(w, err)
			return
		}

		query := r.URL.Query()
		var experimental bool
		// Read query filter params which can affect the output
		if len(query) > 0 {
			if v := query.Get("experimental"); v != "" {
				if v != "" {
					experimental, err = strconv.ParseBool(v)
					if err != nil {
						badRequest(w, fmt.Sprintf("invalid 'experimental' query param: '%s'", v))
						return
					}

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
			if pp, ok := packageList[p.Name]; ok {
				// If the package in the list is newer, do nothing. Otherwise delete and later add the new one.
				if pp.IsNewer(p) {
					continue
				}
			}
			packageList[p.Name] = p
		}

		types := map[string]*Type{}

		for _, p := range packageList {
			for _, t := range p.DatasetTypes {
				if _, ok := types[t]; !ok {
					types[t] = &Type{
						Name:  t,
						Title: util.DatasetTypes[t],
						Count: 0,
					}
				}

				types[t].Count = types[t].Count + 1
			}
		}

		data, err := getTypesOutput(types)
		if err != nil {
			notFoundError(w, err)
			return
		}

		cacheHeaders(w, cacheTime)
		jsonHeader(w)
		fmt.Fprint(w, string(data))
	}
}

func getTypesOutput(types map[string]*Type) ([]byte, error) {

	var keys []string
	for k := range types {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var outputCategories []*Type
	for _, k := range keys {
		c := types[k]
		if title, ok := util.CategoryTitles[c.Title]; ok {
			c.Title = title
		}
		outputCategories = append(outputCategories, c)
	}

	return json.MarshalIndent(outputCategories, "", "  ")
}
