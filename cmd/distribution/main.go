// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/elastic/package-registry/cmd/distribution/internal/workers"
)

const defaultAddress = "https://epr.elastic.co"

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "update-matrix":
			if len(os.Args) < 3 {
				fmt.Fprintln(os.Stderr, "usage:", os.Args[0], "update-matrix <config.yaml>...")
				os.Exit(-1)
			}
			if err := runUpdateMatrix(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
			return
		}
	}

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	var searchOnly, downloadOnly bool
	var listPath string
	flags.BoolVar(&searchOnly, "search-only", false, "collect packages, write them as JSON to the list file and exit without performing actions; needs `list` in the configuration or -list")
	flags.BoolVar(&downloadOnly, "download-only", false, "read packages from the JSON list file written by -search-only instead of collecting them, and perform the actions; needs `list` in the configuration or -list")
	flags.StringVar(&listPath, "list", "", "`file` for the package list, overriding `list` in the configuration; only used with -search-only or -download-only")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage:", os.Args[0], "[-search-only | -download-only] [-list file] <config.yaml>")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return errors.New("expected exactly one configuration file")
	}
	if searchOnly && downloadOnly {
		return errors.New("-search-only and -download-only cannot be used together")
	}
	if listPath != "" && !searchOnly && !downloadOnly {
		return errors.New("-list needs -search-only or -download-only")
	}

	configPath := flags.Arg(0)
	config, err := readConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read configuration from %s: %w", configPath, err)
	}

	// Resolve the list path before searching so a missing one fails fast.
	if searchOnly || downloadOnly {
		if listPath == "" {
			listPath = config.List
		}
		if listPath == "" {
			flagName := "-search-only"
			if downloadOnly {
				flagName = "-download-only"
			}
			return fmt.Errorf("%s needs a list file: set list in the configuration or use -list <file>", flagName)
		}
	}

	// Initialize the actions before searching so a bad configuration fails
	// fast. -search-only never runs them, so it must not create their outputs.
	if !searchOnly {
		for _, action := range config.Actions {
			if err := action.init(config); err != nil {
				return fmt.Errorf("failed to initialize actions: %w", err)
			}
		}
	}

	var packages []packageInfo
	if downloadOnly {
		if len(config.Queries) > 0 || len(config.Matrix) > 0 || config.VersionLimit != 0 {
			fmt.Fprintf(os.Stderr, "warning: queries, matrix and version.limit are ignored with -download-only; only the packages in %s are processed\n", listPath)
		}
		packages, err = readPackageList(listPath)
		if err != nil {
			return err
		}
	} else {
		packages, err = config.collect(httpClient)
		if err != nil {
			return fmt.Errorf("failed to collect packages: %w", err)
		}
		if searchOnly {
			if err := writePackageList(listPath, packages); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, len(packages), "packages written to", listPath, "(actions skipped)")
			return nil
		}
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
		return err
	}
	fmt.Fprintln(os.Stderr, len(packages), "packages total")
	return nil
}

// writePackageList writes packages as JSON to path.
func writePackageList(path string, packages []packageInfo) error {
	d, err := json.MarshalIndent(packages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode package list: %w", err)
	}
	if err := os.WriteFile(path, append(d, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write package list to %s: %w", path, err)
	}
	return nil
}

// readPackageList reads a list written by writePackageList and checks that
// every entry can be downloaded.
func readPackageList(path string) ([]packageInfo, error) {
	d, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read package list: %w", err)
	}
	var packages []packageInfo
	if err := json.Unmarshal(d, &packages); err != nil {
		return nil, fmt.Errorf("failed to parse package list %s: %w", path, err)
	}
	for i, p := range packages {
		if p.Download == "" || p.SignaturePath == "" {
			return nil, fmt.Errorf("invalid package list %s: entry %d (%s-%s) needs download and signature_path", path, i, p.Name, p.Version)
		}
	}
	return packages, nil
}

type printAction struct{}

func (a *printAction) init(c config) error {
	return nil
}

func (a *printAction) perform(i packageInfo) error {
	fmt.Println("- ", i.Name, i.Version)
	return nil
}
