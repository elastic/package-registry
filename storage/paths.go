package storage

import "path/filepath"

const (
	// Internal bucket
	V2MetadataStoragePath = "v2/metadata"
	CursorStoragePath  = V2MetadataStoragePath + "/cursor.json"
	SearchIndexAllFile = "search-index-all.json"

	// Public bucket
	artifactsStoragePath         = "artifacts"
	artifactsPackagesStoragePath = artifactsStoragePath + "/packages"
	artifactsStaticStoragePath   = artifactsStoragePath + "/static"
)

func BuildSearchIndexAllStoragePath(cursorRevision, indexFile string) string {
	return JoinObjectPaths(cursorRevision, indexFile)
}

func JoinObjectPaths(paths ...string) string {
	p := filepath.Join(paths...)
	return normalizeObjectPath(p)
}

func normalizeObjectPath(path string) string {
	if path[0] == '/' {
		return path[1:]
	}
	return path
}

