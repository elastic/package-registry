// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"flag"
	"os"
	"strings"
)

func parseFlags() {
	parseFlagSetWithArgs(flag.CommandLine, os.Args)
}

func parseFlagSetWithArgs(flagSet *flag.FlagSet, args []string) {
	flagsFromEnv(flagSet)

	// Skip args[0] as flag.Parse() does.
	flagSet.Parse(args[1:])
}

func flagsFromEnv(flagSet *flag.FlagSet) {
	flagSet.VisitAll(func(f *flag.Flag) {
		envName := flagEnvName(f.Name)
		if value, found := os.LookupEnv(envName); found {
			f.Value.Set(value)
		}
	})
}

const flagEnvPrefix = "EPR_"

func flagEnvName(name string) string {
	name = strings.ToUpper(name)
	name = strings.ReplaceAll(name, "-", "_")
	return flagEnvPrefix + name
}
