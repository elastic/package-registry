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
	var publicDir string

	flag.StringVar(&beatsDir, "beatsDir", "", "Path to the beats repository")
	flag.StringVar(&publicDir, "publicDir", "", "Path to the public directory ")
	flag.Parse()

	if beatsDir == "" || publicDir == "" {
		log.Fatal("beatsDir and publicDir must be set")
	}

	if err := build(beatsDir, publicDir); err != nil {
		log.Fatal(err)
	}
}

func build(beatsDir, publicDir string) error {
	packages := packageMap{}

	for _, beatName := range logSources {
		err := packages.loadFromSource(beatsDir, beatName, "logs")
		if err != nil {
			return errors.Wrap(err, "loading logs source failed")
		}
	}

	for _, beatName := range metricSources {
		err := packages.loadFromSource(beatsDir, beatName, "metrics")
		if err != nil {
			return errors.Wrap(err, "loading metrics source failed")
		}
	}

	return packages.writePackages(publicDir)
}
