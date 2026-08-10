// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func revisionObject(revision string) fakestorage.Object {
	return fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName: FakePackageStorageBucketInternal,
			Name:       joinObjectPaths(v2MetadataStoragePath, revision, searchIndexDeltaFile),
			Md5Hash:    fakeObjectMD5Hash,
		},
		Content: []byte(`{}`),
	}
}

func newCursorsServer(t *testing.T, revisions ...string) *fakestorage.Server {
	t.Helper()
	objects := make([]fakestorage.Object, 0, len(revisions))
	for _, r := range revisions {
		objects = append(objects, revisionObject(r))
	}
	server := fakestorage.NewServer(objects)
	t.Cleanup(server.Stop)
	return server
}

func TestListCursorsBetween_EmptyRange(t *testing.T) {
	server := newCursorsServer(t, "2", "3")
	client := ClientNoAuth(server)

	timestamps, err := listCursorsBetween(t.Context(), client, FakePackageStorageBucketInternal, "", "3", "3")
	require.NoError(t, err)
	assert.Empty(t, timestamps)
}

func TestListCursorsBetween_SingleIntermediate(t *testing.T) {
	server := newCursorsServer(t, "1", "2", "3")
	client := ClientNoAuth(server)

	timestamps, err := listCursorsBetween(t.Context(), client, FakePackageStorageBucketInternal, "", "1", "2")
	require.NoError(t, err)
	require.Len(t, timestamps, 1)
	assert.Equal(t, "2", timestamps[0])
}

func TestListCursorsBetween_MultipleIntermediates(t *testing.T) {
	server := newCursorsServer(t, "1", "2", "3", "4", "5")
	client := ClientNoAuth(server)

	timestamps, err := listCursorsBetween(t.Context(), client, FakePackageStorageBucketInternal, "", "1", "4")
	require.NoError(t, err)
	assert.Equal(t, []string{"2", "3", "4"}, timestamps)
}

func TestListCursorsBetween_ExcludesSince(t *testing.T) {
	server := newCursorsServer(t, "1", "2", "3")
	client := ClientNoAuth(server)

	timestamps, err := listCursorsBetween(t.Context(), client, FakePackageStorageBucketInternal, "", "1", "3")
	require.NoError(t, err)
	assert.NotContains(t, timestamps, "1")
	assert.Contains(t, timestamps, "2")
	assert.Contains(t, timestamps, "3")
}

func TestListCursorsBetween_IncludesUntil(t *testing.T) {
	server := newCursorsServer(t, "1", "2", "3")
	client := ClientNoAuth(server)

	timestamps, err := listCursorsBetween(t.Context(), client, FakePackageStorageBucketInternal, "", "1", "3")
	require.NoError(t, err)
	assert.Contains(t, timestamps, "3")
}
