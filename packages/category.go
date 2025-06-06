// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

type Category struct {
	Id          string `yaml:"id" json:"id"`
	Title       string `yaml:"title" json:"title"`
	Count       int    `yaml:"count" json:"count"`
	ParentId    string `yaml:"parent_id,omitempty" json:"parent_id,omitempty"`
	ParentTitle string `yaml:"parent_title,omitempty" json:"parent_title,omitempty"`
}
