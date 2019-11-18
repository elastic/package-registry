// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	title = "foo"
)
var packageTests = []struct {
	p           Package
	valid       bool
	description string
}{
	{
		Package{},
		false,
		"empty",
	},
	{
		Package{
			Title: &title,
		},
		false,
		"missing description",
	},
	{
		Package{
			Title: &title,
			Requirement: Requirement{
				Kibana{
					Versions: "bar",
				},
			},
		},
		false,
		"invalid Kibana version",
	},
	{
		Package{
			Title:       &title,
			Description: "my description",
			Requirement: Requirement{
				Kibana{
					Versions: ">=1.2.3 <=4.5.6",
				},
			},
			Categories: []string{"metrics", "logs", "foo"},
		},
		false,
		"invalid category ",
	},
	{
		Package{
			Title:       &title,
			Description: "my description",
			Categories:  []string{"metrics", "logs"},
		},
		true,
		"complete",
	},
}

func TestValidate(t *testing.T) {
	for _, tt := range packageTests {
		t.Run(tt.description, func(t *testing.T) {
			err := tt.p.Validate()

			if err != nil {
				assert.False(t, tt.valid)
			} else {
				assert.True(t, tt.valid)
			}
		})
	}
}
