// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

//go:build mage

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoVersionFilesInSync(t *testing.T) {
	tests := []struct {
		name               string
		goVersionContents  string
		dockerfileContents string
		expectedError      string
	}{
		{
			name:               "matching versions",
			goVersionContents:  "1.26.7\n",
			dockerfileContents: "ARG GO_VERSION=1.26.7\n",
		},
		{
			name:               "different versions",
			goVersionContents:  "1.26.7\n",
			dockerfileContents: "ARG GO_VERSION=1.25.0\n",
			expectedError:      "does not match",
		},
		{
			name:               "empty Go version",
			goVersionContents:  " \n",
			dockerfileContents: "ARG GO_VERSION=1.26.7\n",
			expectedError:      "empty go version",
		},
		{
			name:               "missing Dockerfile declaration",
			goVersionContents:  "1.26.7\n",
			dockerfileContents: "FROM golang:1.26.7\n",
			expectedError:      "found 0",
		},
		{
			name:               "multiple Dockerfile declarations",
			goVersionContents:  "1.26.7\n",
			dockerfileContents: "ARG GO_VERSION=1.26.7\nARG GO_VERSION=1.26.7\n",
			expectedError:      "found 2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			goVersionPath := filepath.Join(dir, ".go-version")
			dockerfilePath := filepath.Join(dir, "Dockerfile")
			require.NoError(t, os.WriteFile(goVersionPath, []byte(test.goVersionContents), 0o644))
			require.NoError(t, os.WriteFile(dockerfilePath, []byte(test.dockerfileContents), 0o644))

			err := checkGoVersionFilesInSync(goVersionPath, dockerfilePath)
			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}
