// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/stretchr/testify/require"
)

const (
	fakePackageStorageBucketInternal = "fake-package-storage-internal"
	fakePackageStorageBucketPublic   = "fake-package-storage-public"
)

func prepareFakeServer(t *testing.T, indexPath string) *fakestorage.Server {
	indexContent, err := ioutil.ReadFile(indexPath)
	require.NoError(t, err, "index file must be populated")

	var index searchIndexAll
	err = json.Unmarshal(indexContent, &index)
	require.NoError(t, err, "index file must be valid")
	require.NotEmpty(t, index.Packages, "index file must contain some package entries")

	const firstRevision = "1"

	var serverObjects []fakestorage.Object
	// Add cursor and index file
	serverObjects = append(serverObjects, fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName: fakePackageStorageBucketInternal, Name: cursorStoragePath,
		},
		Content: []byte(`{"cursor":"` + firstRevision + `"}`),
	})
	serverObjects = append(serverObjects, fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName: fakePackageStorageBucketInternal, Name: joinObjectPaths(v2MetadataStoragePath, firstRevision, searchIndexAllFile),
		},
		Content: indexContent,
	})

	for _, aPackage := range index.Packages {
		nameVersion := fmt.Sprintf("%s-%s", aPackage.PackageManifest.Name, aPackage.PackageManifest.Version)

		// Add fake static resources: docs, img
		for _, asset := range aPackage.Assets {
			if !strings.HasPrefix(asset, "docs") &&
				!strings.HasPrefix(asset, "img") {
				continue
			}

			path := joinObjectPaths(artifactsStaticStoragePath, nameVersion, asset)
			serverObjects = append(serverObjects, fakestorage.Object{
				ObjectAttrs: fakestorage.ObjectAttrs{
					BucketName: fakePackageStorageBucketPublic, Name: path,
				},
				Content: []byte(filepath.Base(path)),
			})
		}

		// Add fake .zip package
		path := joinObjectPaths(artifactsPackagesStoragePath, nameVersion+".zip")
		serverObjects = append(serverObjects, fakestorage.Object{
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: fakePackageStorageBucketPublic, Name: path,
			},
			Content: []byte(filepath.Base(path)),
		})
	}

	t.Logf("Prepared %d packages with total %d server objects.", len(index.Packages), len(serverObjects))
	return fakestorage.NewServer(serverObjects)
}
