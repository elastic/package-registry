// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package categories

import (
	"bytes"
	_ "embed"
)

//go:embed categories.yml
var defaultCategoriesYml []byte

var defaultCategories Categories

func init() {
	r := bytes.NewReader(defaultCategoriesYml)
	defaultCategories = MustReadCategories(r)
}

// DefaultCategories returns the default categories.
func DefaultCategories() Categories {
	return defaultCategories
}
