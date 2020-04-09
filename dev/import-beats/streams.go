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
	"sort"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/elastic/package-registry/util"
)

type manifestWithVars struct {
	Vars []util.Variable `yaml:"var"`
}

type varWithDefault struct {
	Default interface{} `yaml:"default"`
}

var ignoredConfigOptions = []string{
	"module",
	"metricsets",
	"enabled",
}

// createStreams method builds a set of stream inputs including configuration variables.
// Stream defintions depend on a beat type - log or metric.
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

	var mwv manifestWithVars
	err = yaml.Unmarshal(manifestFile, &mwv)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling manifest file failed (path: %s)", manifestPath)
	}

	return []util.Stream{
		{
			Input:       "logs",
			Title:       fmt.Sprintf("%s %s logs", moduleTitle, datasetName),
			Description: fmt.Sprintf("Collect %s %s logs", moduleTitle, datasetName),
			Vars:        adjustVariablesFormat(mwv).Vars,
		},
	}, nil
}

// adjustVariablesFormat method adjusts the format of variables defined in manifest:
// - ensure that all variable values are wrapped with a "default" field, even if they are defined for particular
//   operating systems (prefix: os.)
// - add field "multi: true" if value is an array
func adjustVariablesFormat(mwvs manifestWithVars) manifestWithVars {
	var withDefaults manifestWithVars
	for _, aVar := range mwvs.Vars {
		aVarWithDefaults := aVar
		aVarWithDefaults.Title = toVariableTitle(aVar.Name)
		aVarWithDefaults.Type = determineInputVariableType(aVar.Name, aVar.Default)
		aVarWithDefaults.Required = true
		aVarWithDefaults.ShowUser = true

		if aVarWithDefaults.OsDarwin != nil {
			aVarWithDefaults.OsDarwin = varWithDefault{
				Default: aVarWithDefaults.OsDarwin,
			}
		}

		if aVarWithDefaults.OsWindows != nil {
			aVarWithDefaults.OsWindows = varWithDefault{
				Default: aVarWithDefaults.OsWindows,
			}
		}
		withDefaults.Vars = append(withDefaults.Vars, aVarWithDefaults)
	}
	return withDefaults
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

	var configOptions []util.Variable

	if len(merged) > 0 {
		var moduleConfig []mapStr
		err = yaml.Unmarshal(merged, &moduleConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "unmarshalling module config failed (moduleName: %s, datasetName: %s)",
				moduleName, datasetName)
		}

		foundConfigEntries := map[string]bool{}

		for _, moduleConfigEntry := range moduleConfig {
			flatEntry := moduleConfigEntry.flatten()
			related, err := isConfigEntryRelatedToMetricset(flatEntry, moduleName, datasetName)
			if err != nil {
				return nil, errors.Wrapf(err, "checking if config entry is related failed (moduleName: %s, datasetName: %s)",
					moduleName, datasetName)
			}

			for name, value := range flatEntry {
				if shouldConfigOptionBeIgnored(name, value) {
					continue
				}

				if _, ok := foundConfigEntries[name]; ok {
					continue // already processed this config option
				}

				if related || strings.HasPrefix(name, fmt.Sprintf("%s.", datasetName)) {
					_, isArray := value.([]interface{})
					configOption := util.Variable{
						Name:     name,
						Type:     determineInputVariableType(name, value),
						Title:    toVariableTitle(name),
						Multi:    isArray,
						Required: true,
						ShowUser: true,
						Default:  value,
					}

					configOptions = append(configOptions, configOption)
					foundConfigEntries[name] = true
				}
			}
		}

		// sort variables to keep them in order while using version control.
		sort.Slice(configOptions, func(i, j int) bool {
			return sort.StringsAreSorted([]string{configOptions[i].Name, configOptions[j].Name})
		})
	}

	return []util.Stream{
		{
			Input:       moduleName + "/metrics",
			Title:       fmt.Sprintf("%s %s metrics", moduleTitle, datasetName),
			Description: fmt.Sprintf("Collect %s %s metrics", moduleTitle, datasetName),
			Vars:        configOptions,
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

// shouldConfigOptionBeIgnored method checks if the configuration option name should be skipped (not used, duplicate, etc.)
func shouldConfigOptionBeIgnored(optionName string, value interface{}) bool {
	if value == nil {
		return true
	}

	for _, ignored := range ignoredConfigOptions {
		if ignored == optionName {
			return true
		}
	}
	return false
}

// isConfigEntryRelatedToMetricset method checks if the configuration entry may affect the dataset settings,
// in other words, checks if the "metricsets" field is present and contains the given datasetName.
func isConfigEntryRelatedToMetricset(entry mapStr, moduleName, datasetName string) (bool, error) {
	var metricsetRelated bool
	if metricsets, ok := entry["metricsets"]; ok {
		metricsetsMapped, ok := metricsets.([]interface{})
		if !ok {
			return false, fmt.Errorf("mapping metricsets failed (moduleName: %s, datasetName: %s)",
				moduleName, datasetName)
		}
		if len(metricsetsMapped) == 0 {
			return false, fmt.Errorf("no metricsets defined (moduleName: %s, datasetName: %s)", moduleName,
				datasetName)
		}

		for _, metricset := range metricsetsMapped {
			if metricset.(string) == datasetName {
				metricsetRelated = true
				break
			}
		}
	}
	return metricsetRelated, nil
}

// determineInputVariableType method determines the most appropriate type of the value or the value in array.
// Support types: text, password, bool, integer
func determineInputVariableType(name, v interface{}) string {
	if arr, isArray := v.([]interface{}); isArray {
		if len(arr) == 0 {
			return "text" // array doesn't contain any items, assuming default type
		}
		return determineInputVariableType(name, arr[0])
	}

	if _, isBool := v.(bool); isBool {
		return "bool"
	} else if _, isInt := v.(int); isInt {
		return "integer"
	}

	if name == "password" {
		return "password"
	}
	return "text"
}

func toVariableTitle(name string) string {
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, ".", " ")
	return strings.Title(name)
}
