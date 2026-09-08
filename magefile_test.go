// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

//go:build mage

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateGoVersionFilesInSync(t *testing.T) {
	goVersionContents, err := os.ReadFile(goVersionFile)
	require.NoError(t, err)

	version := strings.TrimSpace(string(goVersionContents))
	assert.NotEmpty(t, version)

	dockerfileContents, err := os.ReadFile(dockerfile)
	require.NoError(t, err)

	matches := dockerfileGoVersionPattern.FindAll(dockerfileContents, -1)
	require.Len(t, matches, 1)
	assert.Equal(t, "ARG GO_VERSION="+version, string(matches[0]))
}

func TestUpdateGoVersion(t *testing.T) {
	dir := t.TempDir()
	goVersionPath := filepath.Join(dir, ".go-version")
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	dockerfileContents := "ARG BUILDER_IMAGE=golang\nARG GO_VERSION=1.25.0\nFROM ${BUILDER_IMAGE}:${GO_VERSION}\n"
	require.NoError(t, os.WriteFile(goVersionPath, []byte("1.25.0\n"), 0644))
	require.NoError(t, os.WriteFile(dockerfilePath, []byte(dockerfileContents), 0644))

	require.NoError(t, updateGoVersion(goVersionPath, dockerfilePath, " 1.26.7\n"))

	goVersion, err := os.ReadFile(goVersionPath)
	require.NoError(t, err)
	dockerfile, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	assert.Equal(t, "1.26.7\n", string(goVersion))
	assert.Equal(t, "ARG BUILDER_IMAGE=golang\nARG GO_VERSION=1.26.7\nFROM ${BUILDER_IMAGE}:${GO_VERSION}\n", string(dockerfile))

	// Repeating the update must leave both files unchanged.
	require.NoError(t, updateGoVersion(goVersionPath, dockerfilePath, "1.26.7"))
	goVersionAfterSecondUpdate, err := os.ReadFile(goVersionPath)
	require.NoError(t, err)
	dockerfileAfterSecondUpdate, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	assert.Equal(t, goVersion, goVersionAfterSecondUpdate)
	assert.Equal(t, dockerfile, dockerfileAfterSecondUpdate)
}

func TestUpdateGoVersionInvalidVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{name: "empty", version: " \n\t"},
		{name: "invalid", version: "1.26.7 unexpected"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := updateGoVersion("unused", "unused", test.version)
			assert.Error(t, err)
		})
	}
}

func TestUpdateGoVersionRequiresOneDockerfileDeclaration(t *testing.T) {
	tests := []struct {
		name       string
		dockerfile string
	}{
		{name: "missing", dockerfile: "FROM golang:1.25.0\n"},
		{name: "bare", dockerfile: "ARG GO_VERSION\nFROM golang:${GO_VERSION}\n"},
		{name: "multiple", dockerfile: "ARG GO_VERSION=1.25.0\nARG GO_VERSION=1.25.1\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			goVersionPath := filepath.Join(dir, ".go-version")
			dockerfilePath := filepath.Join(dir, "Dockerfile")
			require.NoError(t, os.WriteFile(goVersionPath, []byte("1.25.0\n"), 0644))
			require.NoError(t, os.WriteFile(dockerfilePath, []byte(test.dockerfile), 0644))

			err := updateGoVersion(goVersionPath, dockerfilePath, "1.26.7")
			assert.ErrorContains(t, err, "expected exactly one ARG GO_VERSION=<version> declaration")

			goVersion, readErr := os.ReadFile(goVersionPath)
			require.NoError(t, readErr)
			dockerfile, readErr := os.ReadFile(dockerfilePath)
			require.NoError(t, readErr)
			assert.Equal(t, "1.25.0\n", string(goVersion))
			assert.Equal(t, test.dockerfile, string(dockerfile))
		})
	}
}
