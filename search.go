package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/blang/semver"

	"github.com/elastic/integrations-registry/p"
)

func searchHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		integrations, err := getIntegrationPackages()
		if err != nil {
			notFound(w, err)
			return
		}

		integrationsList := map[string]*p.Manifest{}

		query := r.URL.Query()

		var kibanaVersion *semver.Version

		if len(query) > 0 {
			if v := query.Get("kibana"); v != "" {
				kibanaVersion, err = semver.New(v)
				if err != nil {
					notFound(w, fmt.Errorf("invalid Kibana version '%s': %s", v, err))
					return
				}
			}
		}

		// Checks that only the most recent version of an integration is added to the list
		for _, i := range integrations {
			m, err := p.ReadManifest(packagesPath, i)
			if err != nil {
				notFound(w, err)
				return
			}

			valid, err := validKibanaVersion(kibanaVersion, m)
			if err != nil {
				notFound(w, err)
				return
			}

			if !valid {
				continue
			}

			// Check if the version exists and if it should be added or not.
			if i, ok := integrationsList[m.Name]; ok {
				newVersion, _ := semver.Make(m.Version)
				oldVersion, _ := semver.Make(i.Version)

				// Skip addition of integration if only lower or equal
				if newVersion.LTE(oldVersion) {
					continue
				}
			}
			integrationsList[m.Name] = m

		}

		data, err := servePackages(integrationsList, w)
		if err != nil {
			notFound(w, err)
			return
		}

		jsonHeader(w)
		fmt.Fprint(w, string(data))
	}
}

func validKibanaVersion(version *semver.Version, m *p.Manifest) (bool, error) {
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

func servePackages(packagesList map[string]*p.Manifest, w http.ResponseWriter) ([]byte, error) {
	var output []map[string]interface{}

	for _, m := range packagesList {
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
