// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"

	"github.com/elastic/package-registry/internal/database"
)

const FakePackageStorageBucketInternal = "fake-package-storage-internal"

// fakeObjectMD5Hash is a placeholder MD5 digest (16 zero bytes, base64-encoded)
// used to pre-populate ObjectAttrs.Md5Hash on the fake objects we hand to
// fake-gcs-server.
//
// fake-gcs-server's backends only compute a real MD5 hash of the object
// content when Md5Hash is empty (see addObject in
// github.com/fsouza/fake-gcs-server/internal/backend), and that computation
// panics under strict FIPS 140-3 enforcement (GODEBUG=fips140=only) since MD5
// isn't an approved algorithm. The value itself is never inspected: the real
// cloud.google.com/go/storage client explicitly ignores the MD5 hash on
// reads, and package-registry never reads ObjectAttrs.Md5Hash/Etag.
var fakeObjectMD5Hash = base64.StdEncoding.EncodeToString(make([]byte, 16))

func RunFakeServerOnHostPort(indexPath, host string, port uint16) (*fakestorage.Server, error) {
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file %s: %w", indexPath, err)
	}

	const firstRevision = "1"
	serverObjects, _, err := PrepareServerObjects(firstRevision, indexContent)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare server objects: %w", err)
	}
	return fakestorage.NewServerWithOptions(fakestorage.Options{
		InitialObjects: serverObjects,
		Host:           host,
		Port:           port,
		Scheme:         "http",
	})
}

func CreateFakeIndexerOptions(db, swapDb database.Repository) (IndexerOptions, error) {
	fakeIndexerOptions := IndexerOptions{
		PackageStorageBucketInternal: "gs://" + FakePackageStorageBucketInternal,
		WatchInterval:                0,
		Database:                     db,
		SwapDatabase:                 swapDb,
	}
	return fakeIndexerOptions, nil
}

func PrepareFakeServer(tb testing.TB, indexPath string) *fakestorage.Server {
	indexContent, err := os.ReadFile(indexPath)
	require.NoError(tb, err, "index file must be populated")

	const firstRevision = "1"
	serverObjects, numPackages, err := PrepareServerObjects(firstRevision, indexContent)
	require.NoError(tb, err, "failed to prepare server objects")
	tb.Logf("Prepared %d packages with total %d server objects.", numPackages, len(serverObjects))
	return fakestorage.NewServer(serverObjects)
}

// ClientNoAuth returns a GCS client configured to talk to the server without any authentication.
// Base on https://github.com/fsouza/fake-gcs-server/blob/0c333c15145e533e5595bc79def33fbbb5792e8a/fakestorage/server.go#L502-L508
func ClientNoAuth(server *fakestorage.Server) *storage.Client {
	client, err := storage.NewClient(context.Background(),
		option.WithHTTPClient(server.HTTPClient()),
		option.WithoutAuthentication(),
	)
	if err != nil {
		panic(err)
	}
	return client
}

// UpdateFakeServer simulates an index update by stopping the given fake
// server and returning a new one that contains the previous server's objects
// plus the new revision's objects. Callers must point any client bound to
// the old server at the returned one.
//
// It doesn't add the new revision's objects to the running server via
// Server.CreateObject because fake-gcs-server's internal toBackendObjects
// converter (used on that path) strips Md5Hash before handing off to the
// backend, which then recomputes it from scratch — a path that panics under
// strict FIPS 140-3 enforcement (GODEBUG=fips140=only) since MD5 isn't an
// approved algorithm. InitialObjects bypass that recomputation (see
// PrepareFakeServer), so restarting the server keeps all object creation on
// that path instead.
//
// New revision objects are appended after the existing ones so they win any
// name conflicts (e.g. the cursor object is overwritten with the new revision).
func UpdateFakeServer(tb testing.TB, server *fakestorage.Server, revision, indexPath string) *fakestorage.Server {
	indexContent, err := os.ReadFile(indexPath)
	require.NoError(tb, err, "index file must be populated")

	newObjects, numPackages, err := PrepareServerObjects(revision, indexContent)
	require.NoError(tb, err, "failed to prepare server objects")
	tb.Logf("Prepared %d packages with total %d server objects.", numPackages, len(newObjects))

	// Collect all existing objects from the running server so the new server
	// is additive: previous revision files remain accessible alongside the
	// new ones. GetObject round-trips through fromBackendObjects which
	// preserves the stored Md5Hash, keeping subsequent InitialObjects
	// insertion FIPS-safe (non-empty Md5Hash skips recomputation).
	listResp, err := server.ListObjectsWithOptionsPaginated(FakePackageStorageBucketInternal, fakestorage.ListOptions{})
	require.NoError(tb, err, "failed to list existing server objects")
	existingAttrs := listResp.Objects
	allObjects := make([]fakestorage.Object, 0, len(existingAttrs)+len(newObjects))
	for _, attrs := range existingAttrs {
		obj, err := server.GetObject(FakePackageStorageBucketInternal, attrs.Name)
		require.NoError(tb, err, "failed to retrieve existing server object %q", attrs.Name)
		allObjects = append(allObjects, obj)
	}
	allObjects = append(allObjects, newObjects...)

	server.Stop()
	return fakestorage.NewServer(allObjects)
}

type searchIndexAll struct {
	Packages []packageIndex `json:"packages"`
}

func PrepareServerObjects(revision string, indexContent []byte) ([]fakestorage.Object, int, error) {
	var index searchIndexAll
	err := json.Unmarshal(indexContent, &index)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal index content: %w", err)
	}
	if len(index.Packages) == 0 {
		return nil, 0, fmt.Errorf("index file must contain some package entries")
	}

	var serverObjects []fakestorage.Object
	// Add cursor and index file
	serverObjects = append(serverObjects, fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName: FakePackageStorageBucketInternal, Name: cursorStoragePath, Md5Hash: fakeObjectMD5Hash,
		},
		Content: []byte(`{"current":"` + revision + `"}`),
	})
	serverObjects = append(serverObjects, fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName: FakePackageStorageBucketInternal, Name: joinObjectPaths(v2MetadataStoragePath, revision, searchIndexAllFile), Md5Hash: fakeObjectMD5Hash,
		},
		Content: indexContent,
	})
	return serverObjects, len(index.Packages), nil
}
