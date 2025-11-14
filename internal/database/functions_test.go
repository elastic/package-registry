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

func TestAllCapabilitiesAreSupported(t *testing.T) {
	tests := []struct {
		requiredCaps  string
		supportedCaps string
		expected      bool
	}{
		{"a,b", "a,b,c", true},
		{"a,c", "a,b,c", true},
		{"a,d", "a,b,c", false},
		{"", "a,b,c", true},            // An empty required capabilities array means there are no requirements, so it's always satisfied.
		{"a,b", "", true},              // An empty supported capabilities array means there are no capabilities to check against, so it's always satisfied.
		{"", "", true},                 // Both arrays empty means no requirements and nothing to check against, so it's satisfied.
		{"a,b,a", "a,b,c", true},       // Duplicates in required capabilities array should not affect the outcome.
		{"a,b", "b,a,c", true},         // Order in supported capabilities array should not affect the outcome.
		{"a,b", "a,b", true},           // Exact match.
		{"a,b,c", "a,b", false},        // Required capabilities has more elements than supported capabilities.
		{"a,b,c", "a,b,c,d,e,f", true}, // Required capabilities is a subset of supported capabilities.
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("source: [%s] target: [%s]", tt.requiredCaps, tt.supportedCaps), func(t *testing.T) {
			result, err := allCapabilitiesAreSupported(nil, []driver.Value{tt.requiredCaps, tt.supportedCaps})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAllDiscoveryFiltersAreSupported(t *testing.T) {
	tests := []struct {
		packageFilters string
		queryFilters   string
		expected       bool
	}{
		{"filter1,filter2", "filter1,filter2,filter3", true},
		{"filter1,filter2", "filter1,filter3", false},
		{"filter1", "filter1", true},
		{"filter1", "filter2", false},
		{"", "filter1", false},                       // No required filters means no match
		{"filter1", "", false},                       // No query filters means no match if package requires filters
		{"", "", false},                              // No required filters means no match
		{"filter1,filter2", "filter2,filter1", true}, // Order doesn't matter
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("packageFilters: '%s', queryFilters: '%s'", tt.packageFilters, tt.queryFilters), func(t *testing.T) {
			result, err := allDiscoveryFiltersAreSupported(nil, []driver.Value{tt.packageFilters, tt.queryFilters})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnyDiscoveryFilterIsSupported(t *testing.T) {
	tests := []struct {
		packageFilters string
		queryFilters   string
		expected       bool
	}{
		{"filter1,filter2", "filter1,filter2,filter3", true},
		{"filter1,filter2", "filter1,filter3", true},
		{"filter1", "filter1", true},
		{"filter1", "filter2", false},
		{"", "filter1", false},                        // No required filters means no match
		{"filter1", "", false},                        // No query filters means no match if package requires filters
		{"", "", false},                               // No required filters means no match
		{"filter1,filter2", "filter2,filter1", true},  // Order doesn't matter
		{"filter1,filter2", "filter3,filter4", false}, // No matching filters
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("packageFilters: '%s', queryFilters: '%s'", tt.packageFilters, tt.queryFilters), func(t *testing.T) {
			result, err := anyDiscoveryFilterIsSupported(nil, []driver.Value{tt.packageFilters, tt.queryFilters})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
