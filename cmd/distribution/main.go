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
	var writeList, fromList listFlag
	flags.Var(&writeList, "write-list", "collect packages, write them as JSON to the list file and exit without performing actions; the file is the `list` of the configuration unless given as -write-list=file")
	flags.Var(&fromList, "from-list", "read packages from the JSON list file written by -write-list instead of collecting them; the file is the `list` of the configuration unless given as -from-list=file")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage:", os.Args[0], "[-write-list[=file] | -from-list[=file]] <config.yaml>")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return errors.New("expected exactly one configuration file")
	}
	if writeList.set && fromList.set {
		return errors.New("-write-list and -from-list cannot be used together")
	}

	configPath := flags.Arg(0)
	config, err := readConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read configuration from %s: %w", configPath, err)
	}

	// Resolve the paths before searching so a missing one fails fast.
	var fromPath, writePath string
	if fromList.set {
		fromPath, err = fromList.resolve("-from-list", config.List)
		if err != nil {
			return err
		}
	}
	if writeList.set {
		writePath, err = writeList.resolve("-write-list", config.List)
		if err != nil {
			return err
		}
	}

	var packages []packageInfo
	if fromList.set {
		packages, err = readPackageList(fromPath)
		if err != nil {
			return err
		}
	} else {
		packages, err = config.collect(httpClient)
		if err != nil {
			return fmt.Errorf("failed to collect packages: %w", err)
		}
		if writeList.set {
			if err := writePackageList(writePath, packages); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, len(packages), "packages written to", writePath)
			return nil
		}
	}

	for _, action := range config.Actions {
		if err := action.init(config); err != nil {
			return fmt.Errorf("failed to initialize actions: %w", err)
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

// listFlag is a flag with an optional value. Given bare, it only turns the
// mode on and the path comes from the configuration; given as -flag=file, the
// file overrides the configuration. Use ./true for a file literally named true.
type listFlag struct {
	set  bool
	path string
}

func (f *listFlag) String() string { return f.path }

func (f *listFlag) IsBoolFlag() bool { return true }

func (f *listFlag) Set(v string) error {
	f.set = true
	f.path = v
	if v == "true" {
		f.path = ""
	}
	return nil
}

// resolve returns the flag value, or fallback when the flag was given bare.
func (f *listFlag) resolve(name, fallback string) (string, error) {
	if f.path != "" {
		return f.path, nil
	}
	if fallback == "" {
		return "", fmt.Errorf("%s needs a file: set list in the configuration or use %s=<file>", name, name)
	}
	return fallback, nil
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
