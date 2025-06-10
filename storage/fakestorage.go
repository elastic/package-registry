// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package storage

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fsouza/fake-gcs-server/fakestorage"
)

const fakePackageStorageBucketInternal = "fake-package-storage-internal"

var FakeIndexerOptions = IndexerOptions{
	PackageStorageBucketInternal: "gs://" + fakePackageStorageBucketInternal,
	WatchInterval:                0,
}

func RunFakeServerOnHostPort(indexPath, host string, port uint16) (*fakestorage.Server, error) {
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file %s: %w", indexPath, err)
	}

	const firstRevision = "1"
	serverObjects, err := prepareServerObjects(firstRevision, indexContent)
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

func PrepareFakeServer(indexPath string) (*fakestorage.Server, error) {
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file %s: %w", indexPath, err)
	}

	const firstRevision = "1"
	serverObjects, err := prepareServerObjects(firstRevision, indexContent)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare server objects: %w", err)
	}
	return fakestorage.NewServer(serverObjects), nil
}

func updateFakeServer(server *fakestorage.Server, revision, indexPath string) error {
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("failed to read index file %s: %w", indexPath, err)
	}

	serverObjects, err := prepareServerObjects(revision, indexContent)
	if err != nil {
		return fmt.Errorf("failed to prepare server objects: %w", err)
	}

	for _, so := range serverObjects {
		server.CreateObject(so)
	}
	return nil
}

type searchIndexAll struct {
	Packages []packageIndex `json:"packages"`
}

func prepareServerObjects(revision string, indexContent []byte) ([]fakestorage.Object, error) {
	var index searchIndexAll
	err := json.Unmarshal(indexContent, &index)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal index content: %w", err)
	}
	if len(index.Packages) == 0 {
		return nil, fmt.Errorf("index file must contain some package entries")
	}

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
	return serverObjects, nil
}
