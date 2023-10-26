// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlagsFromEnv(t *testing.T) {
	expected := "my value"
	os.Setenv("EPR_TEST_DUMMY", expected)

	var dummyFlag string
	flagSet := flag.NewFlagSet("", flag.PanicOnError)
	flagSet.StringVar(&dummyFlag, "test-dummy", "default", "Dummy flag used for testing.")
	require.Equal(t, "default", dummyFlag)

	flagsFromEnv(flagSet)
	require.Equal(t, expected, dummyFlag)
}

func TestFlagsPrecedence(t *testing.T) {
	expected := "flag value"
	os.Setenv("EPR_TEST_PRECEDENCE_DUMMY", "other value")

	var dummyFlag string
	flagSet := flag.NewFlagSet("", flag.PanicOnError)
	flagSet.StringVar(&dummyFlag, "test-precedence-dummy", "default", "Dummy flag used for testing.")
	require.Equal(t, "default", dummyFlag)

	args := []string{"test", "-test-precedence-dummy=" + expected}
	err := parseFlagSetWithArgs(flagSet, args)
	require.NoError(t, err)
	require.Equal(t, expected, dummyFlag)
}

func TestFlagEnvName(t *testing.T) {
	cases := []struct {
		flagName string
		expected string
	}{
		{"dry-run", "EPR_DRY_RUN"},
		{"test-dummy", "EPR_TEST_DUMMY"},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, flagEnvName(c.flagName))
	}
}
