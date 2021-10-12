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
	flagsFromEnv()
	flag.Parse()
}

func flagsFromEnv() {
	flag.VisitAll(func(f *flag.Flag) {
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
