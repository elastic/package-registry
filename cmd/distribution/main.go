// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"fmt"
	"os"

	"github.com/elastic/package-registry/cmd/distribution/internal/workers"
)

const defaultAddress = "https://epr.elastic.co"

func main() {
	if len(os.Args) != 2 {
		usageAndExit(-1)
	}
	config, err := readConfig(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read configuration from %s: %s\n", os.Args[1], err)
		os.Exit(-1)
	}
	for _, action := range config.Actions {
		err := action.init(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to initialize actions: %s\n", err)
			os.Exit(-1)
		}
	}

	packages, err := config.collect(httpClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to collect packages: %s\n", err)
		os.Exit(-1)
	}

	taskpool := workers.NewTaskPool(downloadConcurrency)
	for _, info := range packages {
		taskpool.Do(func() error {
			for _, action := range config.Actions {
				err := action.perform(info)
				if err != nil {
					return fmt.Errorf("failed to perform action: %w", err)
				}
			}
			return nil
		})
	}
	if err := taskpool.Wait(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
	fmt.Fprintln(os.Stderr, len(packages), "packages total")
}

func usageAndExit(status int) {
	fmt.Fprintln(os.Stderr, os.Args[0], "[config.yaml]")
	os.Exit(status)
}

type printAction struct{}

func (a *printAction) init(c config) error {
	return nil
}

func (a *printAction) perform(i packageInfo) error {
	fmt.Println("- ", i.Name, i.Version)
	return nil
}
