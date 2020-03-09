// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"flag"
	"log"

	"github.com/pkg/errors"
)

func main() {
	// Beats repository directory
	var beatsDir string
	// Target public directory where the generated packages should end up in
	var outputDir string

	flag.StringVar(&beatsDir, "beatsDir", "../beats", "Path to the beats repository")
	flag.StringVar(&outputDir, "outputDir", "dev/packages/beats", "Path to the output directory ")
	flag.Parse()

	if beatsDir == "" || outputDir == "" {
		log.Fatal("beatsDir and outputDir must be set")
	}

	if err := build(beatsDir, outputDir); err != nil {
		log.Fatal(err)
	}
}

// build method visits all beats in beatsDir to collect configuration data for modules.
// The package-registry groups integrations per target product not per module type. It's opposite to the beats project,
// where logs and metrics are distributed with different beats (oriented either on logs or metrics - metricbeat,
// filebeat, etc.).
func build(beatsDir, outputDir string) error {
	repository := newPackageRepository()

	for _, beatName := range logSources {
		err := repository.createPackagesFromSource(beatsDir, beatName, "logs")
		if err != nil {
			return errors.Wrap(err, "creating form logs source failed")
		}
	}

	for _, beatName := range metricSources {
		err := repository.createPackagesFromSource(beatsDir, beatName, "metrics")
		if err != nil {
			return errors.Wrap(err, "creating from metrics source failed")
		}
	}

	return repository.save(outputDir)
}
