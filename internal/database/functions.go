// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package database

import (
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterScalarFunction("semver_compare_constraint", 2, semverCompareConstraint)
	sqlite.MustRegisterScalarFunction("semver_compare_ge", 2, semverCompareGreaterThanEqual)
	sqlite.MustRegisterScalarFunction("semver_compare_le", 2, semverCompareLessThanEqual)
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
