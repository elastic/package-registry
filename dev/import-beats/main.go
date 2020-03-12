// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"flag"
	"log"

	"github.com/pkg/errors"
)

type importerOptions struct {
	// Beats repository directory
	beatsDir string

	// Kibana host and port
	kibanaHostPort string
	// Kibana repository directory
	kibanaDir string

	// Elastic UI Framework directory
	euiDir string

	// Target public directory where the generated packages should end up in
	outputDir string
}

func (io *importerOptions) validate() error {

}

func main() {
	var options importerOptions

	flag.StringVar(&options.beatsDir, "beatsDir", "../beats", "Path to the beats repository")
	flag.StringVar(&options.kibanaDir, "kibanaDir", "../kibana", "Path to the kibana repository")
	flag.StringVar(&options.kibanaHostPort, "kibanaHostPort", "localhost:5601", "Kibana host and port")
	flag.StringVar(&options.euiDir, "euiDir", "../eui", "Path to the Elastic UI framework repository")
	flag.StringVar(&options.outputDir, "outputDir", "dev/packages/beats", "Path to the output directory")
	flag.Parse()

	if beatsDir == "" || outputDir == "" || kibanaHostPort == "" {
		flag.Usage()
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
