// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package categories

import (
	_ "embed"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Category is a common structure for all kinds of categories.
type Category struct {
	Title string `yaml:"title"`
}

// MainCategory is a main category, that can have subcategories.
type MainCategory struct {
	Category `yaml:",inline"`

	SubCategories map[string]SubCategory `yaml:"subcategories"`
}

// SubCategory is a sub-category, should be contained in a Category.
type SubCategory struct {
	Category `yaml:",inline"`
}

// Categories is a list of categories.
type Categories map[string]MainCategory

func (categories Categories) TitlesMap() map[string]string {
	if len(categories) == 0 {
		return nil
	}
	titles := make(map[string]string)
	for name, category := range categories {
		titles[name] = category.Title

		for name, category := range category.SubCategories {
			titles[name] = category.Title
		}
	}
	return titles
}

// ReadCategories reads the categories from a reader.
func ReadCategories(r io.Reader) (Categories, error) {
	var categoriesFile struct {
		Categories Categories `yaml:"categories"`
	}
	dec := yaml.NewDecoder(r)
	err := dec.Decode(&categoriesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to decode categories: %w", err)
	}
	// TODO: Check for duplicated categories.
	return categoriesFile.Categories, nil
}

// MustReadCategories reads the categories from a reader and panics if there is any error.
func MustReadCategories(r io.Reader) Categories {
	categories, err := ReadCategories(r)
	if err != nil {
		panic(err)
	}

	return categories
}
