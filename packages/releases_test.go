// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var releaseTests = []struct {
	release string
	exists  bool
}{
	{
		ReleaseBeta,
		true,
	},
	{
		"foo",
		false,
	},
	{
		ReleaseExperimental,
		true,
	},
	{
		ReleaseGa,
		true,
	},
}

func TestReleases(t *testing.T) {
	for _, tt := range releaseTests {
		t.Run(tt.release, func(t *testing.T) {
			exists := IsValidRelease(tt.release)
			assert.Equal(t, tt.exists, exists)
		})
	}
}
