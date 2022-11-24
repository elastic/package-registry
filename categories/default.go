// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package categories

import (
	"bytes"
	_ "embed"
)

//go:embed categories.yml
var defaultCategoriesYml []byte

var defaultCategories Categories

var defaultCategoriesTitlesMap map[string]string

func init() {
	r := bytes.NewReader(defaultCategoriesYml)
	defaultCategories = MustReadCategories(r)
	defaultCategoriesTitlesMap = defaultCategories.TitlesMap()
}

// DefaultCategories returns the default categories.
func DefaultCategories() Categories {
	return defaultCategories
}

// TitlesMap is a convenience method that returns the titles of the default categories.
func TitlesMap() map[string]string {
	return defaultCategoriesTitlesMap
}
