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

func TestValidateFlagsFIPSTLSMinVersion(t *testing.T) {
	// Save and restore globals modified by validateFlags.
	origIsFIPS := isFIPSBinary
	origTLSMin := tlsMinVersionValue
	origCert := tlsCertFile
	origKey := tlsKeyFile
	t.Cleanup(func() {
		isFIPSBinary = origIsFIPS
		tlsMinVersionValue = origTLSMin
		tlsCertFile = origCert
		tlsKeyFile = origKey
	})

	// Provide dummy cert/key paths so the existing cert-presence check passes.
	tlsCertFile = "cert.pem"
	tlsKeyFile = "key.pem"

	t.Run("FIPS binary with TLS 1.1 is rejected", func(t *testing.T) {
		isFIPSBinary = func() bool { return true }
		tlsMinVersionValue = tlsVersionValue(tls.VersionTLS11)
		err := validateFlags()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "FIPS 140-3")
		assert.Contains(t, err.Error(), "1.1")
	})

	t.Run("FIPS binary with TLS 1.2 is allowed", func(t *testing.T) {
		isFIPSBinary = func() bool { return true }
		tlsMinVersionValue = tlsVersionValue(tls.VersionTLS12)
		err := validateFlags()
		require.NoError(t, err)
	})

	t.Run("non-FIPS binary with TLS 1.1 is allowed", func(t *testing.T) {
		isFIPSBinary = func() bool { return false }
		tlsMinVersionValue = tlsVersionValue(tls.VersionTLS11)
		err := validateFlags()
		require.NoError(t, err)
	})
}
