// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/package-registry/util"
)

// createStreams method builds a set of stream inputs including configuration variables.
// Stream definitions depend on a beat type - log or metric.
// At the moment, the array returns only one stream.
func createStreams(modulePath, moduleName, moduleTitle, datasetName, beatType string) ([]util.Stream, error) {
	switch beatType {
	case "logs":
		return createLogStreams(modulePath, moduleTitle, datasetName)
	case "metrics":
		return createMetricStreams(modulePath, moduleName, moduleTitle, datasetName)
	}
	return nil, fmt.Errorf("invalid beat type: %s", beatType)
}

// createLogStreams method builds a set of stream inputs for logs oriented dataset.
// The method unmarshals "manifest.yml" file and picks all configuration variables.
func createLogStreams(modulePath, moduleTitle, datasetName string) ([]util.Stream, error) {
	manifestPath := filepath.Join(modulePath, datasetName, "manifest.yml")
	manifestFile, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading manifest file failed (path: %s)", manifestPath)
	}

	vars, err := createLogStreamVariables(manifestFile)
	if err != nil {
		return nil, errors.Wrapf(err, "creating log stream variables failed (path: %s)", manifestPath)
	}
	return []util.Stream{
		{
			Input:       "logs",
			Title:       fmt.Sprintf("%s %s logs", moduleTitle, datasetName),
			Description: fmt.Sprintf("Collect %s %s logs", moduleTitle, datasetName),
			Vars:        vars,
		},
	}, nil
}

// wrapVariablesWithDefault method builds a set of stream inputs for metrics oriented dataset.
// The method combines all config files in module's _meta directory, unmarshals all configuration entries and selects
// ones related to the particular metricset (first seen, first occurrence, next occurrences skipped).
//
// The method skips commented variables, but keeps arrays of structures (even if it's not possible to render them using
// UI).
func createMetricStreams(modulePath, moduleName, moduleTitle, datasetName string) ([]util.Stream, error) {
	merged, err := mergeMetaConfigFiles(modulePath)
	if err != nil {
		return nil, errors.Wrapf(err, "merging config files failed")
	}

	vars, err := createMetricStreamVariables(merged, moduleName, datasetName)
	if err != nil {
		return nil, errors.Wrapf(err, "creating metric stream variables failed (modulePath: %s)", modulePath)
	}
	return []util.Stream{
		{
			Input:       moduleName + "/metrics",
			Title:       fmt.Sprintf("%s %s metrics", moduleTitle, datasetName),
			Description: fmt.Sprintf("Collect %s %s metrics", moduleTitle, datasetName),
			Vars:        vars,
		},
	}, nil
}

// mergeMetaConfigFiles method visits all configuration YAML files and combines them into single document.
func mergeMetaConfigFiles(modulePath string) ([]byte, error) {
	configFilePaths, err := filepath.Glob(filepath.Join(modulePath, "_meta", "config*.yml"))
	if err != nil {
		return nil, errors.Wrapf(err, "locating config files failed (modulePath: %s)", modulePath)
	}

	var mergedConfig bytes.Buffer
	for _, configFilePath := range configFilePaths {
		configFile, err := ioutil.ReadFile(configFilePath)
		if err != nil && !os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "reading config file failed (path: %s)", configFilePath)
		}
		mergedConfig.Write(configFile)
		mergedConfig.WriteString("\n")
	}
	return mergedConfig.Bytes(), nil
}
