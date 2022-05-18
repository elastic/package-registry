// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import "path/filepath"

const (
	// Internal bucket
	v2MetadataStoragePath = "v2/metadata"
	cursorStoragePath     = v2MetadataStoragePath + "/cursor.json"
	searchIndexAllFile    = "search-index-all.json"

	// Public bucket
	artifactsStoragePath         = "artifacts"
	artifactsPackagesStoragePath = artifactsStoragePath + "/packages"
	artifactsStaticStoragePath   = artifactsStoragePath + "/static"
)

func joinObjectPaths(paths ...string) string {
	p := filepath.Join(paths...)
	return normalizeObjectPath(p)
}

func normalizeObjectPath(path string) string {
	if path[0] == '/' {
		return path[1:]
	}
	return path
}
