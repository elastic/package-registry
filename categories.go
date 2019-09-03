package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/blang/semver"

	"github.com/elastic/integrations-registry/util"
)

var categoryTitles = map[string]string{
	"logs":    "Logs",
	"metrics": "Metrics",
}

// categoriesHandler is a dynamic handler as it will also allow filtering in the future.
func categoriesHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		integrations, err := getIntegrationPackages()
		if err != nil {
			notFound(w, err)
			return
		}

		packageList := map[string]*util.Manifest{}
		// Get unique list of newest packages
		for _, i := range integrations {
			m, err := util.ReadManifest(packagesPath, i)
			if err != nil {
				return
			}

			// Check if the version exists and if it should be added or not.
			if p, ok := packageList[m.Name]; ok {
				newVersion, _ := semver.Make(m.Version)
				oldVersion, _ := semver.Make(p.Version)

				// Skip addition of package if only lower or equal
				if newVersion.LTE(oldVersion) {
					continue
				}
			}
			packageList[m.Name] = m
		}

		categories := map[string]*Category{}

		for _, p := range packageList {
			for _, c := range p.Categories {
				if _, ok := categories[c]; !ok {
					categories[c] = &Category{
						Id:    c,
						Title: c,
						Count: 0,
					}
				}

				categories[c].Count = categories[c].Count + 1
			}
		}

		var keys []string
		for k := range categories {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var outputCategories []*Category
		for _, k := range keys {
			c := categories[k]
			if title, ok := categoryTitles[c.Title]; ok {
				c.Title = title
			}
			outputCategories = append(outputCategories, c)
		}

		j, err := json.MarshalIndent(outputCategories, "", "  ")
		if err != nil {
			notFound(w, err)
			return
		}
		jsonHeader(w)
		fmt.Fprint(w, string(j))
	}
}

type Category struct {
	Id    string `yaml:"id" json:"id"`
	Title string `yaml:"title" json:"title"`
	Count int    `yaml:"count" json:"count"`
}
