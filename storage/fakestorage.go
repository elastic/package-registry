// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fakePackageStorageBucketInternal = "fake-package-storage-internal"
	fakePackageStorageBucketPublic   = "fake-package-storage-public"
)

var FakeIndexerOptions = IndexerOptions{
	PackageStorageBucketInternal: "gs://" + fakePackageStorageBucketInternal,
	PackageStorageBucketPublic:   "gs://" + fakePackageStorageBucketPublic,
	WatchInterval:                0,
}

func PrepareFakeServer(t *testing.T, indexPath string) *fakestorage.Server {
	indexContent, err := ioutil.ReadFile(indexPath)
	require.NoError(t, err, "index file must be populated")

	const firstRevision = "1"
	serverObjects := prepareServerObjects(t, firstRevision, indexContent)
	return fakestorage.NewServer(serverObjects)
}

func updateFakeServer(t *testing.T, server *fakestorage.Server, revision, indexPath string) {
	indexContent, err := ioutil.ReadFile(indexPath)
	require.NoError(t, err, "index file must be populated")

	serverObjects := prepareServerObjects(t, revision, indexContent)

	for _, so := range serverObjects {
		server.CreateObject(so)
	}
}

func prepareServerObjects(t *testing.T, revision string, indexContent []byte) []fakestorage.Object {
	var index searchIndexAll
	err := json.Unmarshal(indexContent, &index)
	require.NoError(t, err, "index file must be valid")
	require.NotEmpty(t, index.Packages, "index file must contain some package entries")

	var serverObjects []fakestorage.Object
	// Add cursor and index file
	serverObjects = append(serverObjects, fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName: fakePackageStorageBucketInternal, Name: cursorStoragePath,
		},
		Content: []byte(`{"current":"` + revision + `"}`),
	})
	serverObjects = append(serverObjects, fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName: fakePackageStorageBucketInternal, Name: joinObjectPaths(v2MetadataStoragePath, revision, searchIndexAllFile),
		},
		Content: indexContent,
	})

	for _, aPackage := range index.Packages {
		nameVersion := fmt.Sprintf("%s-%s", aPackage.PackageManifest.Name, aPackage.PackageManifest.Version)

		// Add fake static resources: docs, img
		for _, asset := range aPackage.PackageManifest.Assets {
			assetPath, err := filepath.Rel(filepath.Join("/package", aPackage.PackageManifest.Name, aPackage.PackageManifest.Version), asset)
			require.NoError(t, err, "relative path expected")
			if !strings.HasPrefix(assetPath, "docs") &&
				!strings.HasPrefix(assetPath, "img") {
				continue
			}

			path := joinObjectPaths(artifactsStaticStoragePath, nameVersion, assetPath)
			serverObjects = append(serverObjects, fakestorage.Object{
				ObjectAttrs: fakestorage.ObjectAttrs{
					BucketName: fakePackageStorageBucketPublic, Name: path,
				},
				Content: []byte(filepath.Base(path)),
			})
		}

		// Add fake .zip.sig
		path := joinObjectPaths(artifactsPackagesStoragePath, nameVersion+".zip.sig")
		serverObjects = append(serverObjects, fakestorage.Object{
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: fakePackageStorageBucketPublic, Name: path,
			},
			Content: []byte(filepath.Base(path)),
		})

		// Add fake .zip package
		path = joinObjectPaths(artifactsPackagesStoragePath, nameVersion+".zip")
		serverObjects = append(serverObjects, fakestorage.Object{
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: fakePackageStorageBucketPublic, Name: path,
			},
			Content: []byte(filepath.Base(path)),
		})
	}
	t.Logf("Prepared %d packages with total %d server objects.", len(index.Packages), len(serverObjects))
	return serverObjects
}

func TestPrepareFakeServer(t *testing.T) {
	// given
	indexFile := "testdata/search-index-all-full.json"
	testIndexFile, err := os.ReadFile(indexFile)
	require.NoErrorf(t, err, "index file should be present in testdata")

	// when
	fs := PrepareFakeServer(t, indexFile)
	defer fs.Stop()

	// then
	client := fs.Client()
	require.NotNil(t, client, "client should be initialized")

	aCursor := readObject(t, client.Bucket(fakePackageStorageBucketInternal).Object(cursorStoragePath))
	assert.Equal(t, []byte(`{"current":"1"}`), aCursor)
	anIndex := readObject(t, client.Bucket(fakePackageStorageBucketInternal).Object(joinObjectPaths(v2MetadataStoragePath, "1", searchIndexAllFile)))
	assert.Equal(t, testIndexFile, anIndex)
	packageZip := readObject(t, client.Bucket(fakePackageStorageBucketPublic).Object(joinObjectPaths(artifactsPackagesStoragePath, "1password-1.1.1.zip")))
	assert.NotZero(t, len(packageZip), ".zip package must have fake content")
	packageSig := readObject(t, client.Bucket(fakePackageStorageBucketPublic).Object(joinObjectPaths(artifactsPackagesStoragePath, "1password-1.1.1.zip.sig")))
	assert.NotZero(t, len(packageSig), ".zip.sig must have fake content")

	// check few static files
	readme := readObject(t, client.Bucket(fakePackageStorageBucketPublic).Object(joinObjectPaths(artifactsStaticStoragePath, "1password-1.1.1", "docs/README.md")))
	assert.Equal(t, []byte("README.md"), readme)
	screenshot := readObject(t, client.Bucket(fakePackageStorageBucketPublic).Object(joinObjectPaths(artifactsStaticStoragePath, "1password-1.1.1", "img/1password-signinattempts-screenshot.png")))
	assert.Equal(t, []byte("1password-signinattempts-screenshot.png"), screenshot)
}

func readObject(t *testing.T, handle *storage.ObjectHandle) []byte {
	reader, err := handle.NewReader(context.Background())
	require.NoErrorf(t, err, "can't initialize reader for object %s", handle.ObjectName())
	content, err := io.ReadAll(reader)
	require.NoErrorf(t, err, "io.ReadAll failed for object %s", handle.ObjectName())
	return content
}
