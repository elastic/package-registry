package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/mholt/archiver"
	"github.com/stretchr/testify/require"

	"github.com/elastic/package-registry/storage"
)

const (
	fakePackageStorageBucketInternal = "fake-package-storage-internal"
	fakePackageStorageBucketPublic   = "fake-package-storage-public"
)

func TestEndpoints_StorageIndexer(t *testing.T) {
	// Given
	// - index is already published to the internal bucket.
	// - there are sample packages loaded.
	const cursorRevision = "1"
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: fakePackageStorageBucketInternal, Name: storage.CursorStoragePath,
			},
			Content: []byte(`{"cursor":"` + cursorRevision + `"}`),
		}, {
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: fakePackageStorageBucketInternal, Name: storage.JoinObjectPaths(storage.V2MetadataStoragePath, cursorRevision, storage.SearchIndexAllFile),
			},
			Content: []byte(`{"cursor":"1"}`),
		}, {
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: fakePackageStorageBucketPublic, Name: "artifacts/packages/foobar-1.2.3.zip",
			},
			Content: createArchive(t, "testdata/packages/foobar-1.2.3"),
		},
	})
	defer server.Stop()
	client := server.Client()

}

func createArchive(t *testing.T, sourcePath string) []byte {
	destinationPath := filepath.Join(os.TempDir(), fmt.Sprintf("indexing-job-tests-%d.zip", time.Now().UnixNano()))
	defer os.RemoveAll(destinationPath)

	err := archiver.Archive([]string{sourcePath}, destinationPath)
	require.NoError(t, err)

	content, err := ioutil.ReadFile(destinationPath)
	require.NoError(t, err)
	return content
}
