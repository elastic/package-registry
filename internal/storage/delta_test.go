// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"testing"

	"cloud.google.com/go/storage"
	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newDeltaServer(t *testing.T, deltaJSON string) (*fakestorage.Server, *storage.Client) {
	t.Helper()
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: FakePackageStorageBucketInternal,
				Name:       joinObjectPaths(v2MetadataStoragePath, "1", searchIndexDeltaFile),
				Md5Hash:    fakeObjectMD5Hash,
			},
			Content: []byte(deltaJSON),
		},
	})
	t.Cleanup(server.Stop)
	return server, ClientNoAuth(server)
}

func TestLoadSearchIndexDelta_ParsesAdded(t *testing.T) {
	server, client := newDeltaServer(t, `{"added":[{"package_manifest":{"name":"mypkg","version":"1.0.0","type":"integration"}}],"updated":[],"removed":[]}`)

	delta, err := loadSearchIndexDelta(t.Context(), zap.NewNop(), client, FakePackageStorageBucketInternal, "", cursor{Current: "1"})
	require.NoError(t, err)
	require.Len(t, delta.Added, 1)
	assert.Equal(t, "mypkg", delta.Added[0].PackageManifest.Name)
	assert.Equal(t, "1.0.0", delta.Added[0].PackageManifest.Version)
	assert.Empty(t, delta.Updated)
	assert.Empty(t, delta.Removed)
	_ = server
}

func TestLoadSearchIndexDelta_ParsesUpdated(t *testing.T) {
	server, client := newDeltaServer(t, `{"added":[],"updated":[{"package_manifest":{"name":"mypkg","version":"2.0.0","type":"integration"}}],"removed":[]}`)

	delta, err := loadSearchIndexDelta(t.Context(), zap.NewNop(), client, FakePackageStorageBucketInternal, "", cursor{Current: "1"})
	require.NoError(t, err)
	assert.Empty(t, delta.Added)
	require.Len(t, delta.Updated, 1)
	assert.Equal(t, "mypkg", delta.Updated[0].PackageManifest.Name)
	assert.Equal(t, "2.0.0", delta.Updated[0].PackageManifest.Version)
	assert.Empty(t, delta.Removed)
	_ = server
}

func TestLoadSearchIndexDelta_ParsesRemoved(t *testing.T) {
	server, client := newDeltaServer(t, `{"added":[],"updated":[],"removed":[{"name":"oldpkg","version":"1.0.0"}]}`)

	delta, err := loadSearchIndexDelta(t.Context(), zap.NewNop(), client, FakePackageStorageBucketInternal, "", cursor{Current: "1"})
	require.NoError(t, err)
	assert.Empty(t, delta.Added)
	assert.Empty(t, delta.Updated)
	require.Len(t, delta.Removed, 1)
	assert.Equal(t, "oldpkg", delta.Removed[0].Name)
	assert.Equal(t, "1.0.0", delta.Removed[0].Version)
	_ = server
}

func TestLoadSearchIndexDelta_AllFields(t *testing.T) {
	server, client := newDeltaServer(t, `{
		"added":[{"package_manifest":{"name":"newpkg","version":"1.0.0","type":"integration"}}],
		"updated":[{"package_manifest":{"name":"existpkg","version":"2.1.0","type":"integration"}}],
		"removed":[{"name":"oldpkg","version":"1.0.0"}]
	}`)

	delta, err := loadSearchIndexDelta(t.Context(), zap.NewNop(), client, FakePackageStorageBucketInternal, "", cursor{Current: "1"})
	require.NoError(t, err)
	require.Len(t, delta.Added, 1)
	require.Len(t, delta.Updated, 1)
	require.Len(t, delta.Removed, 1)
	assert.Equal(t, "newpkg", delta.Added[0].PackageManifest.Name)
	assert.Equal(t, "existpkg", delta.Updated[0].PackageManifest.Name)
	assert.Equal(t, "oldpkg", delta.Removed[0].Name)
	_ = server
}

func TestLoadSearchIndexDelta_MissingFile_ReturnsError(t *testing.T) {
	server := fakestorage.NewServer([]fakestorage.Object{})
	t.Cleanup(server.Stop)
	client := ClientNoAuth(server)

	_, err := loadSearchIndexDelta(t.Context(), zap.NewNop(), client, FakePackageStorageBucketInternal, "", cursor{Current: "1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, storage.ErrObjectNotExist)
}
