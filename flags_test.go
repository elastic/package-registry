// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"crypto/tls"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlagsFromEnv(t *testing.T) {
	expected := "my value"
	t.Setenv("EPR_TEST_DUMMY", expected)

	var dummyFlag string
	flagSet := flag.NewFlagSet("", flag.PanicOnError)
	flagSet.StringVar(&dummyFlag, "test-dummy", "default", "Dummy flag used for testing.")
	require.Equal(t, "default", dummyFlag)

	flagsFromEnv(flagSet)
	require.Equal(t, expected, dummyFlag)
}

func TestFlagsPrecedence(t *testing.T) {
	expected := "flag value"
	t.Setenv("EPR_TEST_PRECEDENCE_DUMMY", "other value")

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

func TestValidateTLSFlagsFIPSTLSMinVersion(t *testing.T) {
	tests := []struct {
		name       string
		fips       bool
		minVersion tlsVersionValue
		wantError  string
	}{
		{
			name:       "FIPS binary with TLS 1.1 is rejected",
			fips:       true,
			minVersion: tlsVersionValue(tls.VersionTLS11),
			wantError:  "FIPS 140-3 build: -tls-min-version 1.1 is not permitted; minimum allowed version is 1.2",
		},
		{
			name:       "FIPS binary with TLS 1.2 is allowed",
			fips:       true,
			minVersion: tlsVersionValue(tls.VersionTLS12),
		},
		{
			name:       "non-FIPS binary with TLS 1.1 is allowed",
			minVersion: tlsVersionValue(tls.VersionTLS11),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTLSFlags("cert.pem", "key.pem", tt.minVersion, tt.fips)
			if tt.wantError != "" {
				assert.EqualError(t, err, tt.wantError)
				return
			}
			assert.NoError(t, err)
		})
	}
}
