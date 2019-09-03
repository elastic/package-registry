package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/blang/semver"

	"github.com/elastic/integrations-registry/util"
)

func searchHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		query := r.URL.Query()

		var kibanaVersion *semver.Version
		var category string
		// Leaving out `a` here to not use a reserved name
		var pckage string
		var err error

		if len(query) > 0 {
			if v := query.Get("kibana"); v != "" {
				kibanaVersion, err = semver.New(v)
				if err != nil {
					notFound(w, fmt.Errorf("invalid Kibana version '%s': %s", v, err))
					return
				}
			}

			if v := query.Get("category"); v != "" {
				if v != "" {
					category = v
				}
			}

			if v := query.Get("package"); v != "" {
				if v != "" {
					pckage = v
				}
			}
		}

		packages, err := getPackages()
		if err != nil {
			notFound(w, err)
			return
		}

		packagesList := map[string]map[string]*util.Manifest{}

		// Checks that only the most recent version of an integration is added to the list
		for _, i := range packages {
			m, err := util.ReadManifest(packagesPath, i)
			if err != nil {
				notFound(w, err)
				return
			}

			// Filter by category first as this could heavily reduce the number of packages
			// It must happen before the version filtering as there only the newest version
			// is exposed and there could be an older package with more versions.
			if category != "" {
				hasCategory, err := hasCategory(category, m)
				if err != nil {
					notFound(w, err)
					return
				}
				if !hasCategory {
					continue
				}
			}

			valid, err := validKibanaVersion(kibanaVersion, m)
			if err != nil {
				notFound(w, err)
				return
			}

			if !valid {
				continue
			}

			if pckage == "" {
				// Check if the version exists and if it should be added or not.
				for _, versions := range packagesList {
					for _, p := range versions {
						if p.Name == m.Name {
							newVersion, _ := semver.Make(m.Version)
							oldVersion, _ := semver.Make(p.Version)

							if newVersion.LTE(oldVersion) {
								continue
							} else {
								delete(packagesList[p.Name], p.Version)
							}
						}
					}
				}
			} else {
				if pckage != m.Name {
					continue
				}
			}

			if _, ok := packagesList[m.Name]; !ok {
				packagesList[m.Name] = map[string]*util.Manifest{}
			}
			packagesList[m.Name][m.Version] = m
		}

		data, err := servePackages(packagesList, w)
		if err != nil {
			notFound(w, err)
			return
		}

		jsonHeader(w)
		fmt.Fprint(w, string(data))
	}
}

func hasCategory(category string, m *util.Manifest) (bool, error) {
	for _, c := range m.Categories {
		if c == category {
			return true, nil
		}
	}

	return false, nil
}

func validKibanaVersion(version *semver.Version, m *util.Manifest) (bool, error) {
	if version != nil {
		if m.Requirement.Kibana.Max != "" {
			maxKibana, err := semver.Parse(m.Requirement.Kibana.Max)
			if err != nil {
				return false, err
			}
			if version.GT(maxKibana) {
				return false, nil
			}
		}

		if m.Requirement.Kibana.Min != "" {
			minKibana, err := semver.Parse(m.Requirement.Kibana.Min)
			if err != nil {
				return false, nil
			}
			if version.LT(minKibana) {
				return false, err
			}
		}
	}
	return true, nil
}

func servePackages(packagesList map[string]map[string]*util.Manifest, w http.ResponseWriter) ([]byte, error) {

	separator := "@"
	// Packages need to be sorted to be always outputted in the same order
	var keys []string
	for key, k := range packagesList {
		for v := range k {
			keys = append(keys, key+separator+v)
		}
	}
	sort.Strings(keys)

	var output []map[string]interface{}

	for _, k := range keys {
		parts := strings.Split(k, separator)
		m := packagesList[parts[0]][parts[1]]
		data := map[string]interface{}{
			"name":        m.Name,
			"description": m.Description,
			"version":     m.Version,
			"download":    "/package/" + m.Name + "-" + m.Version + ".tar.gz",
		}
		if m.Title != nil {
			data["title"] = *m.Title
		}
		if m.Icons != nil {
			data["icons"] = m.Icons
		}
		output = append(output, data)
	}

	return json.MarshalIndent(output, "", "  ")
}
