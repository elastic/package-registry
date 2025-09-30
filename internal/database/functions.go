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
	sqlite.MustRegisterScalarFunction("semver_compare_op", 3, semverCompareOperation)
}

// semverCompare checks if a version satisfies a given semver constraint.
// It takes two string arguments: the version and the constraint.
// It returns a boolean indicating whether the version satisfies the constraint.
func semverCompareConstraint(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	version, ok := args[0].(string)
	if !ok {
		return nil, errors.New("version argument must be a string")
	}
	versionSemver, err := semver.NewVersion(version)
	if err != nil {
		return nil, err
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

	return constraintSemver.Check(versionSemver), nil
}

// semverCompareOperation compares two semantic versions based on a given operation.
// It takes three string arguments: the first version, the operation (e.g., '>=', '<=', etc.), and the second version.
// It returns a boolean indicating whether the comparison holds true.
func semverCompareOperation(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	firstVersion, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("firstVersion argument must be a string")
	}

	operation, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("operation argument must be a string")
	}

	secondVersion, ok := args[2].(string)
	if !ok {
		return nil, fmt.Errorf("secondVersion argument must be a string")
	}

	newArgs := []driver.Value{firstVersion, fmt.Sprintf("%s%s", operation, secondVersion)}

	return semverCompareConstraint(ctx, newArgs)
}
