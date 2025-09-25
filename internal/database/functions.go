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
	sqlite.MustRegisterScalarFunction("semver_compare", 2, semverCompare)
}

// semverCompare checks if a version satisfies a given semver constraint.
// It takes two string arguments: the version and the constraint.
// It returns a boolean indicating whether the version satisfies the constraint.
func semverCompare(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	kibanaVersion, ok := args[0].(string)
	if !ok {
		return nil, errors.New("first argument must be a string")
	}
	semverKibanaVersion, err := semver.NewVersion(kibanaVersion)
	if err != nil {
		return nil, err
	}

	kibanaConstraint, ok := args[1].(string)
	if !ok {
		return nil, errors.New("second argument must be a string")
	}
	// There could be packages wihtout kibana constraint defined
	// In that case, any version is acceptable.
	if kibanaConstraint == "" {
		return true, nil
	}

	constraint, err := semver.NewConstraint(kibanaConstraint)
	if err != nil {
		return nil, fmt.Errorf("invalid semver constraint: %w", err)
	}
	result := constraint.Check(semverKibanaVersion)
	return result, nil
}
