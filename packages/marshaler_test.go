// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testFile = "./testdata/marshaler/packages.json"

var generateFlag = flag.Bool("generate", false, "Write golden files")

func TestMarshalJSON(t *testing.T) {
	// given
	packagesBasePaths := []string{"../testdata/second_package_path", "../testdata/package"}
	indexer := NewFileSystemIndexer(packagesBasePaths...)
	err := indexer.Init(context.Background())
	require.NoError(t, err, "can't initialize indexer")

	// when
	m, err := MarshalJSON(&indexer.packageList)
	require.NoError(t, err)

	// then
	assertExpectedContent(t, testFile, m)
}

func TestUnmarshalJSON(t *testing.T) {
	// given
	packagesBasePaths := []string{"../testdata/second_package_path", "../testdata/package"}
	indexer := NewFileSystemIndexer(packagesBasePaths...)
	err := indexer.Init(context.Background())

	expectedFile, err := os.ReadFile(testFile)
	require.NoError(t, err)

	var packages Packages

	// when
	err = UnmarshalJSON(expectedFile, &packages)

	// then
	require.NoError(t, err, "packages should be loaded")

	// TODO constraint
	// TODO semver
	// TODO base path
	// TODO fsBuilder
	require.Equal(t, packages[0], indexer.packageList[0])
}

func assertExpectedContent(t *testing.T, expectedPath string, actual []byte) {
	if *generateFlag {
		err := os.WriteFile(expectedPath, actual, 0644)
		require.NoError(t, err, "can't write the golden file")
	}

	expected, err := os.ReadFile(expectedPath)
	require.NoError(t, err, "can't read the golden file")

	assert.Equal(t, string(expected), string(actual))
}
