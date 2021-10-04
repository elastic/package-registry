// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"go.elastic.co/apm"

	"github.com/elastic/package-registry/util"
)

func searchHandler(packagesBasePaths []string, cacheTime time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := newSearchFilterFromParams(r)
		if err != nil {
			badRequest(w, err.Error())
			return
		}

		packages, err := util.GetPackages(r.Context(), packagesBasePaths)
		if err != nil {
			notFoundError(w, errors.Wrapf(err, "fetching package failed"))
			return
		}

		packages = filter.FilterPackages(r.Context(), packages)
		packagesList := filter.Filter(r.Context(), packages)

		data, err := getPackageOutput(r.Context(), packagesList)
		if err != nil {
			notFoundError(w, err)
			return
		}

		cacheHeaders(w, cacheTime)
		jsonHeader(w)
		w.Write(data)
	}
}

type searchFilter struct {
	Category      string
	Package       string
	KibanaVersion *semver.Version
	AllVersions   bool
	Internal      bool
	Experimental  bool
}

func newSearchFilterFromParams(r *http.Request) (searchFilter, error) {
	var filter searchFilter

	query := r.URL.Query()
	if len(query) == 0 {
		return filter, nil
	}

	var err error
	if v := query.Get("kibana.version"); v != "" {
		filter.KibanaVersion, err = semver.NewVersion(v)
		if err != nil {
			return filter, fmt.Errorf("invalid Kibana version '%s': %w", v, err)
		}
	}

	if v := query.Get("category"); v != "" {
		filter.Category = v
	}

	if v := query.Get("package"); v != "" {
		filter.Package = v
	}

	if v := query.Get("all"); v != "" {
		// Default is false, also on error
		filter.AllVersions, err = strconv.ParseBool(v)
		if err != nil {
			return filter, fmt.Errorf("invalid 'all' query param: '%s'", v)
		}
	}

	if v := query.Get("internal"); v != "" {
		// In case of error, keep it false
		filter.Internal, err = strconv.ParseBool(v)
		if err != nil {
			return filter, fmt.Errorf("invalid 'internal' query param: '%s'", v)
		}
	}

	if v := query.Get("experimental"); v != "" {
		// In case of error, keep it false
		filter.Experimental, err = strconv.ParseBool(v)
		if err != nil {
			return filter, fmt.Errorf("invalid 'experimental' query param: '%s'", v)
		}
	}

	return filter, nil
}

func (filter searchFilter) FilterPackages(ctx context.Context, packages util.Packages) util.Packages {
	span, ctx := apm.StartSpan(ctx, "FilterPackages", "app")
	defer span.End()

	if filter.AllVersions {
		return packages
	}

	packageList := map[string]util.Package{}

	// Get unique list of newest packages
	for _, p := range packages {
		if filter.KibanaVersion != nil {
			if valid := p.HasKibanaVersion(filter.KibanaVersion); !valid {
				continue
			}
		}

		// Check if the version exists and if it should be added or not.
		// If the package in the list is newer or equal, do nothing.
		if pp, ok := packageList[p.Name]; ok && pp.IsNewerOrEqual(p) {
			continue
		}

		// Otherwise delete and later add the new one.
		packageList[p.Name] = p
	}

	var filtered util.Packages
	for _, p := range packageList {
		filtered = append(filtered, p)
	}
	return filtered
}

func (filter searchFilter) Filter(ctx context.Context, packages util.Packages) map[string]map[string]util.Package {
	span, ctx := apm.StartSpan(ctx, "Filter", "app")
	defer span.End()

	packagesList := map[string]map[string]util.Package{}

	for _, p := range packages {
		// Filter by category first as this could heavily reduce the number of packages
		// It must happen before the version filtering as there only the newest version
		// is exposed and there could be an older package with more versions.
		if filter.Category != "" && !p.HasCategory(filter.Category) && !p.HasPolicyTemplateWithCategory(filter.Category) {
			continue
		}

		// Skip internal packages
		if p.Internal && !filter.Internal {
			continue
		}

		// Skip experimental packages if flag is not specified
		if p.Release == util.ReleaseExperimental && !filter.Experimental {
			continue
		}

		if filter.KibanaVersion != nil {
			if valid := p.HasKibanaVersion(filter.KibanaVersion); !valid {
				continue
			}
		}

		// If package Query is set, all versions of this package are returned
		if filter.Package != "" && filter.Package != p.Name {
			continue
		}

		addPackage := true
		if !filter.AllVersions {
			// Check if the version exists and if it should be added or not.
			for name, versions := range packagesList {
				if name != p.Name {
					continue
				}
				for _, pp := range versions {

					// If the package in the list is newer or equal, do nothing.
					if pp.IsNewerOrEqual(p) {
						addPackage = false
						continue
					}

					// Otherwise delete and later add the new one.
					delete(packagesList[pp.Name], pp.Version)
				}
			}
		}

		if addPackage {
			if _, ok := packagesList[p.Name]; !ok {
				packagesList[p.Name] = map[string]util.Package{}
			}

			if filter.Category != "" && !p.HasCategory(filter.Category) {
				// It means that package's policy template has the category
				p = filterPolicyTemplates(p, filter.Category)
			}

			if _, ok := packagesList[p.Name][p.Version]; !ok {
				packagesList[p.Name][p.Version] = p
			}
		}
	}
	return packagesList
}

func filterPolicyTemplates(p util.Package, category string) util.Package {
	var updatedPolicyTemplates []util.PolicyTemplate
	var updatedBasePolicyTemplates []util.BasePolicyTemplate
	for i, pt := range p.PolicyTemplates {
		if util.StringsContains(pt.Categories, category) {
			updatedPolicyTemplates = append(updatedPolicyTemplates, pt)
			updatedBasePolicyTemplates = append(updatedBasePolicyTemplates, p.BasePackage.BasePolicyTemplates[i])
		}
	}
	p.PolicyTemplates = updatedPolicyTemplates
	p.BasePackage.BasePolicyTemplates = updatedBasePolicyTemplates
	return p
}

type sortedPackage struct {
	name    string
	title   string
	version string
}

func newSortedPackage(aPackage util.Package) sortedPackage {
	title := aPackage.Name
	if aPackage.Title != nil {
		title = *aPackage.Title
	}
	return sortedPackage{
		name:    aPackage.Name,
		title:   title,
		version: aPackage.Version,
	}
}

type sortedPackages []sortedPackage

func (s sortedPackages) Len() int {
	return len(s)
}

func (s sortedPackages) Less(i, j int) bool {
	titlesSorted := sort.StringsAreSorted([]string{s[i].title, s[j].title})
	if s[i].title != s[j].title {
		return titlesSorted
	}
	return sort.StringsAreSorted([]string{s[i].version, s[j].version})
}

func (s sortedPackages) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

var _ sort.Interface = new(sortedPackages)

func getPackageOutput(ctx context.Context, packagesList map[string]map[string]util.Package) ([]byte, error) {
	span, ctx := apm.StartSpan(ctx, "GetPackageOutput", "app")
	defer span.End()

	// Packages need to be sorted to be always outputted in the same order
	var sorted sortedPackages
	for _, versionPackage := range packagesList {
		for _, aPackage := range versionPackage {
			sorted = append(sorted, newSortedPackage(aPackage))
		}
	}
	sort.Sort(sorted)

	var output []util.BasePackage
	for _, s := range sorted {
		m := packagesList[s.name][s.version]
		data := m.BasePackage
		output = append(output, data)
	}
	return util.MarshalJSONPretty(output)
}
