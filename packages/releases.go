// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"github.com/Masterminds/semver/v3"
)

const (
	ReleaseExperimental = "experimental"
	ReleaseBeta         = "beta"
	ReleaseGa           = "ga"

	// Default release if no release is configured
	DefaultRelease            = ReleaseGa
	DefaultPrerelease         = ReleaseBeta
	DefaultReleaseTechPreview = ReleaseBeta
	DefaultLicense            = "basic"
)

var ReleaseTypes = map[string]interface{}{
	ReleaseExperimental: nil,
	ReleaseBeta:         nil,
	ReleaseGa:           nil,
}

// agentlessReleaseTypes contains valid release values for deployment_modes.agentless.release.
// Unlike the package-level release, "experimental" is not valid here.
var agentlessReleaseTypes = map[string]interface{}{
	ReleaseBeta: nil,
	ReleaseGa:   nil,
}

func IsValidRelease(release string) bool {
	_, exists := ReleaseTypes[release]
	return exists
}

func isValidAgentlessRelease(release string) bool {
	_, exists := agentlessReleaseTypes[release]
	return exists
}

// releaseForSemVerCompat is a compatibility function that returns a release
// for a given version.
func releaseForSemVerCompat(version *semver.Version) string {
	if isPrerelease(version) {
		return DefaultPrerelease
	}
	if isTechPreview(version) {
		return DefaultReleaseTechPreview
	}

	return DefaultRelease
}
