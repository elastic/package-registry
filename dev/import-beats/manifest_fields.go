// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

// determinePackageRelease function considers all release tags for modules and justifies a common release tag.
// If any of module has been released as "GA", then the package will be released as "GA",
// else if any of module has been release as "beta", then the package will be released as "beta",
// otherwise "experimental".
func determinePackageRelease(manifestRelease string, fields []fieldDefinition) (string, error) {
	if manifestRelease == "ga" { // manifestRelease
		return "ga", nil
	}

	moduleRelease := fields[0].Release
	if moduleRelease == "" || moduleRelease == "ga" {
		return "ga", nil // missing fields.release means "GA"
	}

	if moduleRelease == "beta" || manifestRelease == "beta" {
		return "beta", nil
	}
	return "experimental", nil
}

func determineDatasetRelease(moduleRelease string, fields []fieldDefinition) (string, error) {
	if len(fields) == 0 {
		return moduleRelease, nil
	}

	datasetRelease := fields[0].Release
	if datasetRelease != "" {
		return datasetRelease, nil
	}
	return moduleRelease, nil
}
