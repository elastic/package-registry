// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"github.com/Masterminds/semver/v3"
)

const (
	ReleaseExperimental = "experimental"
	ReleaseBeta         = "beta"
	ReleaseGa           = "ga"

	// Default release if no release is configured
	DefaultRelease    = ReleaseGa
	DefaultPrerelease = ReleaseBeta
	DefaultLicense    = "basic"
)

var ReleaseTypes = map[string]interface{}{
	ReleaseExperimental: nil,
	ReleaseBeta:         nil,
	ReleaseGa:           nil,
}

func IsValidRelease(release string) bool {
	_, exists := ReleaseTypes[release]
	return exists
}

// releaseForSemVerCompat is a compatibility function that returns a release
// for a given version.
func releaseForSemVerCompat(version *semver.Version) string {
	if isPrerelease(version) {
		return DefaultPrerelease
	}
	return DefaultRelease
}
