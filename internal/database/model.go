// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package database

type Package struct {
	Cursor                  string
	Name                    string
	Version                 string
	VersionMajor            int
	VersionMinor            int
	VersionPatch            int
	VersionPrerelease       string
	FormatVersion           string
	FormatVersionMajorMinor string
	Release                 string
	Prerelease              bool
	KibanaVersion           string
	DiscoveryFilterFields   string
	DiscoveryFilterDatasets string
	Type                    string
	Path                    string
	Data                    []byte
	BaseData                []byte
}
