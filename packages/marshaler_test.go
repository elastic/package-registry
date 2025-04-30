// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/package-registry/internal/database"
	"github.com/elastic/package-registry/internal/util"
)

const testFile = "./testdata/marshaler/packages.json"

var generateFlag = flag.Bool("generate", false, "Write golden files")

func TestMarshalJSON(t *testing.T) {
	// given
	db, err := database.NewMemorySQLDB()
	require.NoError(t, err)

	packagesBasePaths := []string{"../testdata/second_package_path", "../testdata/package"}
	indexer := NewFileSystemIndexer(util.NewTestLogger(), db, packagesBasePaths...)
	defer indexer.Close(context.Background())

	err = indexer.Init(context.Background())
	require.NoError(t, err, "can't initialize indexer")

	// when
	packageList, err := indexer.Get(context.Background(), nil)
	require.NoError(t, err)
	m, err := json.MarshalIndent(packageList, " ", " ")
	require.NoError(t, err)

	// then
	assertExpectedContent(t, testFile, m)
}

func TestUnmarshalJSON(t *testing.T) {
	// given
	db, err := database.NewMemorySQLDB()
	require.NoError(t, err)

	packagesBasePaths := []string{"../testdata/second_package_path", "../testdata/package"}
	indexer := NewFileSystemIndexer(util.NewTestLogger(), db, packagesBasePaths...)
	defer indexer.Close(context.Background())

	err = indexer.Init(context.Background())
	require.NoError(t, err)

	expectedFile, err := os.ReadFile(testFile)
	require.NoError(t, err)

	var packages Packages

	// when
	err = json.Unmarshal(expectedFile, &packages)

	// then
	require.NoError(t, err, "packages should be loaded")
	for i := range indexer.packageList {
		require.Equal(t, packages[i].Name, indexer.packageList[i].Name)
		assert.Equal(t, packages[i].Version, indexer.packageList[i].Version)
		assert.Equal(t, packages[i].Title, indexer.packageList[i].Title)
		assert.Equal(t, packages[i].versionSemVer, indexer.packageList[i].versionSemVer)
		assert.Len(t, packages[i].BasePolicyTemplates, len(packages[i].PolicyTemplates))
		if indexer.packageList[i].Conditions != nil && indexer.packageList[i].Conditions.Kibana != nil {
			assert.Equal(t, packages[i].Conditions.Kibana.constraint, indexer.packageList[i].Conditions.Kibana.constraint)
		}
		assert.Nil(t, packages[i].fsBuilder)
	}
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
