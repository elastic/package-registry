package storage

import "path/filepath"

const (
	v2MetadataStoragePath = "v2/metadata"

	artifactsStoragePath         = "artifacts"
	artifactsPackagesStoragePath = artifactsStoragePath + "/packages"
	artifactsStaticStoragePath   = artifactsStoragePath + "/static"

	cursorFile         = "cursor.json"
	cursorStoragePath  = v2MetadataStoragePath + "/" + cursorFile
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

