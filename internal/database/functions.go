// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package database

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterScalarFunction("semver_compare_constraint", 2, semverCompareConstraint)
	sqlite.MustRegisterScalarFunction("semver_compare_ge", 2, semverCompareGreaterThanEqual)
	sqlite.MustRegisterScalarFunction("semver_compare_le", 2, semverCompareLessThanEqual)
	sqlite.MustRegisterScalarFunction("all_capabilities_are_supported", 2, allCapabilitiesAreSupported)
	sqlite.MustRegisterScalarFunction("all_discovery_filters_are_supported", 2, allDiscoveryFiltersAreSupported)
	sqlite.MustRegisterScalarFunction("any_discovery_filter_is_supported", 2, anyDiscoveryFilterIsSupported)
}

// semverCompare checks if a version satisfies a given semver constraint.
// It takes two string arguments: the version and the constraint.
// It returns a boolean indicating whether the version satisfies the constraint.
func semverCompareConstraint(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	version, err := parseArgAsSemver(args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse version: %w", err)
	}

	constraint, ok := args[1].(string)
	if !ok {
		return nil, errors.New("constraint argument must be a string")
	}

	// If there is no constraint provided, consider it a match
	if constraint == "" {
		return true, nil
	}

	constraintSemver, err := semver.NewConstraint(constraint)
	if err != nil {
		return nil, fmt.Errorf("invalid semver constraint: %w", err)
	}

	return constraintSemver.Check(version), nil
}

func parseArgAsSemver(arg driver.Value) (*semver.Version, error) {
	version, ok := arg.(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string")
	}

	semver, err := semver.NewVersion(version)
	if err != nil {
		return nil, err
	}

	return semver, nil
}

// semverCompareGreaterThanEqual checks if the first semantic version is greater than or equal to the second.
func semverCompareGreaterThanEqual(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	firstVersion, err := parseArgAsSemver(args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse firstVersion: %w", err)
	}
	secondVersion, err := parseArgAsSemver(args[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse firstVersion: %w", err)
	}

	return firstVersion.GreaterThanEqual(secondVersion), nil
}

// semverCompareLessThanEqual checks if the first semantic version is less than or equal to the second.
func semverCompareLessThanEqual(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	firstVersion, err := parseArgAsSemver(args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse firstVersion: %w", err)
	}
	secondVersion, err := parseArgAsSemver(args[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse firstVersion: %w", err)
	}

	return firstVersion.LessThanEqual(secondVersion), nil
}

// allCapabilitiesAreSupported checks if all the required capabilities (first array) are present in the second array (supported capabilities).
// Both arrays are represented as comma-separated strings.
// It returns true if all elements are present, false otherwise.
func allCapabilitiesAreSupported(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	requiredCaps, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("first argument must be a string")
	}
	supportedCaps, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("second argument must be a string")
	}

	if requiredCaps == "" {
		// No required capabilities, always satisfied.
		return true, nil
	}

	if supportedCaps == "" {
		// No supported capabilities used, always satisfied.
		// Based on (package).WorksWithCapabilities function logic.
		return true, nil
	}

	supportedCapsElements := strings.Split(supportedCaps, ",")

	for requiredCapability := range strings.SplitSeq(requiredCaps, ",") {
		if !slices.Contains(supportedCapsElements, requiredCapability) {
			return false, nil
		}
	}
	return true, nil
}

// allDiscoveryFiltersAreSupported checks if all required discovery filters in the package are present in the query filters.
// It takes two string arguments: the required filters and the query filters.
// It returns a boolean indicating whether all required filters are present in the query filters.
func allDiscoveryFiltersAreSupported(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	packageFilters, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("first argument must be a string")
	}
	queryFilters, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("second argument must be a string")
	}

	if packageFilters == "" {
		// If no package filters are specified, it is not a match.
		return false, nil
	}

	if queryFilters == "" {
		// No discovery filters used, it is not a match.
		// Based on (discoveryFilterFields).Matches(p *Package) function logic.
		return false, nil
	}

	queryFilterElements := strings.Split(queryFilters, ",")

	for requiredFilter := range strings.SplitSeq(packageFilters, ",") {
		if !slices.Contains(queryFilterElements, requiredFilter) {
			return false, nil
		}
	}
	return true, nil
}

// anyDiscoveryFilterIsSupported checks if any of the required discovery filters in the package are present in the query filters.
// It takes two string arguments: the required filters and the query filters.
// It returns a boolean indicating whether any required filter is present in the query filters.
func anyDiscoveryFilterIsSupported(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	packageFilters, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("first argument must be a string")
	}
	queryFilters, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("second argument must be a string")
	}

	if packageFilters == "" {
		// If no package filters are specified, it is not a match.
		return false, nil
	}

	if queryFilters == "" {
		// No discovery filters used, it is not a match.
		// Based on (discoveryFilterDatasets).Matches(p *Package) function logic.
		return false, nil
	}

	queryFilterElements := strings.Split(queryFilters, ",")

	for requiredFilter := range strings.SplitSeq(packageFilters, ",") {
		if slices.Contains(queryFilterElements, requiredFilter) {
			return true, nil
		}
	}
	return false, nil
}
