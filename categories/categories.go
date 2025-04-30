// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package categories

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v2"
)

// Category is a common structure for all kinds of categories.
type Category struct {
	Name   string
	Title  string
	Parent *Category
}

// Categories is a list of categories.
type Categories map[string]Category

// ReadCategories reads the categories from a reader.
func ReadCategories(r io.Reader) (Categories, error) {
	var categoriesFile struct {
		Categories map[string]struct {
			Title         string `yaml:"title"`
			Subcategories map[string]struct {
				Title string `yaml:"title"`
			} `yaml:"subcategories"`
		} `yaml:"categories"`
	}
	dec := yaml.NewDecoder(r)
	err := dec.Decode(&categoriesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to decode categories: %w", err)
	}

	categories := make(Categories)
	addCategory := func(name, title string, parent *Category) error {
		if _, found := categories[name]; found {
			return fmt.Errorf("ambiguous definition for category %q", name)
		}
		categories[name] = Category{
			Name:   name,
			Title:  title,
			Parent: parent,
		}
		return nil
	}
	for name, category := range categoriesFile.Categories {
		err := addCategory(name, category.Title, nil)
		if err != nil {
			return nil, err
		}

		for subname, subcategory := range category.Subcategories {
			parent := categories[name]
			err := addCategory(subname, subcategory.Title, &parent)
			if err != nil {
				return nil, err
			}
		}
	}

	return categories, nil
}

// MustReadCategories reads the categories from a reader and panics if there is any error.
func MustReadCategories(r io.Reader) Categories {
	categories, err := ReadCategories(r)
	if err != nil {
		panic(err)
	}

	return categories
}
