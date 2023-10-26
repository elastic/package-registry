// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

var supportedTLSVersions map[string]uint16 = map[string]uint16{
	"1.0": tls.VersionTLS10,
	"1.1": tls.VersionTLS11,
	"1.2": tls.VersionTLS12,
	"1.3": tls.VersionTLS13,
}

type tlsVersionValue struct {
	version uint16
}

func (t tlsVersionValue) String() string {
	switch t.version {
	case tls.VersionTLS10:
		return "1.0"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS13:
		return "1.3"
	default:
		return ""
	}
}

func (t tlsVersionValue) Value() uint16 {
	return t.version
}

func (t *tlsVersionValue) Set(s string) error {
	if _, ok := supportedTLSVersions[s]; !ok {
		return fmt.Errorf("unsupported TLS version: %s", s)
	}
	t.version = supportedTLSVersions[s]
	return nil
}

func parseFlags() error {
	return parseFlagSetWithArgs(flag.CommandLine, os.Args)
}

func parseFlagSetWithArgs(flagSet *flag.FlagSet, args []string) error {
	err := flagsFromEnv(flagSet)
	if err != nil {
		return err
	}

	// Skip args[0] as flag.Parse() does.
	flagSet.Parse(args[1:])
	return nil
}

func flagsFromEnv(flagSet *flag.FlagSet) error {
	var flagErrors error
	flagSet.VisitAll(func(f *flag.Flag) {
		envName := flagEnvName(f.Name)
		if value, found := os.LookupEnv(envName); found {
			if err := f.Value.Set(value); err != nil {
				flagErrors = errors.Join(flagErrors, fmt.Errorf("failed to set -%s: %v", f.Name, err))
			}
		}
	})
	return flagErrors
}

const flagEnvPrefix = "EPR_"

func flagEnvName(name string) string {
	name = strings.ToUpper(name)
	name = strings.ReplaceAll(name, "-", "_")
	return flagEnvPrefix + name
}
