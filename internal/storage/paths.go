// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"fmt"
	"net/url"
	"path"
)

const (
	// Internal bucket
	v2MetadataStoragePath = "v2/metadata"
	cursorStoragePath     = v2MetadataStoragePath + "/cursor.json"
	searchIndexAllFile    = "search-index-all.json"

	// Public bucket
	artifactsStoragePath         = "artifacts"
	ArtifactsPackagesStoragePath = artifactsStoragePath + "/packages"
	ArtifactsStaticStoragePath   = artifactsStoragePath + "/static"
)

func extractBucketNameFromURL(anURL string) (string, string, error) {
	u, err := url.Parse(anURL)
	if err != nil {
		return "", "", fmt.Errorf("can't parse object URL: %w", err)
	}

	uPath := u.Path
	if len(uPath) == 0 {
		return u.Host, "", nil
	}
	return u.Host, normalizeObjectPath(uPath), nil
}

func joinObjectPaths(paths ...string) string {
	p := path.Join(paths...)
	return normalizeObjectPath(p)
}

func normalizeObjectPath(path string) string {
	if path[0] == '/' {
		return path[1:]
	}
	return path
}
