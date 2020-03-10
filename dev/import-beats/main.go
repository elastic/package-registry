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
	// Kibana host and port
	var kibanaHostPort string

	flag.StringVar(&beatsDir, "beatsDir", "../beats", "Path to the beats repository")
	flag.StringVar(&outputDir, "outputDir", "dev/packages/beats", "Path to the output directory")
	flag.StringVar(&kibanaHostPort, "kibanaHostPort", "localhost:5601", "Kibana host and port")
	flag.Parse()

	if beatsDir == "" || outputDir == "" || kibanaHostPort == "" {
		log.Fatal("beatsDir, outputDir and kibanaHostPort must be set")
	}

	if err := build(beatsDir, outputDir, kibanaHostPort); err != nil {
		log.Fatal(err)
	}
}

// build method visits all beats in beatsDir to collect configuration data for modules.
// The package-registry groups integrations per target product not per module type. It's opposite to the beats project,
// where logs and metrics are distributed with different beats (oriented either on logs or metrics - metricbeat,
// filebeat, etc.).
func build(beatsDir, outputDir, kibanaHostPort string) error {
	kibanaMigrator := newKibanaMigrator(kibanaHostPort)
	repository := newPackageRepository(kibanaMigrator)

	for _, beatName := range logSources {
		err := repository.createPackagesFromSource(beatsDir, beatName, "logs")
		if err != nil {
			return errors.Wrap(err, "creating from logs source failed")
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
