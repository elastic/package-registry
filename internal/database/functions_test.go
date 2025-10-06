// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package database

import (
	"database/sql/driver"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSemverCompareConstraint(t *testing.T) {
	tests := []struct {
		version    string
		constraint string
		expected   bool
	}{
		{"1.2.3", ">=1.0.0", true},
		{"1.2.3", "<1.0.0", false},
		{"1.2.3", "~1.2.0", true},
		{"1.2.3", "^1.2.0", true},
		{"1.2.3", ">=1.2.0, <2.0.0", true},
		{"1.2.3", ">=1.3.0", false},
		{"2.3.4", "^1.0.0 || ^2.0.0", true},
		{"2.3.4", "^1.0.0 || ~2.3.0", true},
		{"1.2.3-beta", ">=1.0.0", false},  // Pre-release versions are not included unless specified
		{"1.2.3-beta", ">=1.0.0-0", true}, // Pre-release versions are included with -0
		{"1.2.3", "", true},               // No constraint means always true
	}

	for _, tt := range tests {
		constraint := tt.constraint
		if constraint == "" {
			constraint = "<empty>"
		}
		t.Run(fmt.Sprintf("%s %s", tt.version, constraint), func(t *testing.T) {
			result, err := semverCompareConstraint(nil, []driver.Value{tt.version, tt.constraint})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSemverCompareGreaterThanEqual(t *testing.T) {
	tests := []struct {
		firstVersion  string
		secondVersion string
		expected      bool
	}{
		{"1.2.3", "1.0.0", true},
		{"1.0.0", "1.2.3", false},
		{"1.2.3-beta", "1.0.0", true}, // Pre-release versions are only "excluded" when matching against constrsaints, not in direct comparisons
		{"1.2.3-beta", "1.0.0-0", true},
		{"1.2.3-beta", "1.2.3", false},
		{"1.2.3", "1.2.3-beta", true},
		{"1.2.3-0", "1.2.3-beta", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.firstVersion, tt.secondVersion), func(t *testing.T) {
			result, err := semverCompareGreaterThanEqual(nil, []driver.Value{tt.firstVersion, tt.secondVersion})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSemverCompareLessThanEqual(t *testing.T) {
	tests := []struct {
		firstVersion  string
		secondVersion string
		expected      bool
	}{
		{"1.2.3", "1.0.0", false},
		{"1.0.0", "1.2.3", true},
		{"1.2.3-beta", "1.0.0", false}, // Pre-release versions are only "excluded" when matching against constrsaints, not in direct comparisons
		{"1.0.0", "1.2.3-beta", true},
		{"1.2.3", "1.2.3-beta", false},
		{"1.2.3-beta", "1.2.3", true},
		{"1.2.3-beta", "1.0.0-0", false},
		{"1.2.3-0", "1.2.3-beta", true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.firstVersion, tt.secondVersion), func(t *testing.T) {
			result, err := semverCompareLessThanEqual(nil, []driver.Value{tt.firstVersion, tt.secondVersion})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
