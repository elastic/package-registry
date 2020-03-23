// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/elastic/package-registry/util"
)

type manifestWithVars struct {
	Vars []map[string]interface{} `yaml:"var"`
}

type varWithDefault struct {
	Default interface{} `yaml:"default"`
}

var ignoredConfigOptions = []string{
	"module",
	"metricsets",
	"enabled",
}

func createStreams(modulePath, moduleName, datasetName, beatType string) ([]util.Stream, error) {
	switch beatType {
	case "logs":
		return createLogStreams(modulePath, moduleName, datasetName)
	case "metrics":
		return createMetricStreams(modulePath, moduleName, datasetName)
	}
	return nil, fmt.Errorf("invalid beat type: %s", beatType)
}

func createLogStreams(modulePath, moduleName, datasetName string) ([]util.Stream, error) {
	manifestPath := path.Join(modulePath, datasetName, "manifest.yml")
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
			Title:       fmt.Sprintf("%s %s logs", strings.Title(moduleName), strings.Title(datasetName)),
			Description: fmt.Sprintf("Collect %s %s logs", strings.Title(moduleName), strings.Title(datasetName)),
			Vars:        wrapVariablesWithDefault(mwv).Vars,
		},
	}, nil
}

func wrapVariablesWithDefault(mwvs manifestWithVars) manifestWithVars {
	var withDefaults manifestWithVars
	for _, aVar := range mwvs.Vars {
		aVarWithDefaults := map[string]interface{}{}
		for k, v := range aVar {
			if strings.HasPrefix(k, "os.") {
				aVarWithDefaults[k] = varWithDefault{
					Default: v,
				}
			} else {
				aVarWithDefaults[k] = v
			}
		}
		withDefaults.Vars = append(withDefaults.Vars, aVarWithDefaults)
	}
	return withDefaults
}

func createMetricStreams(modulePath, moduleName, datasetName string) ([]util.Stream, error) {
	merged, err := mergeMetaConfigFiles(modulePath)
	if err != nil {
		return nil, errors.Wrapf(err, "merging config files failed")
	}

	var configOptions []map[string]interface{}

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
			if !related {
				continue
			}

			for name, value := range flatEntry {
				if shouldConfigOptionBeIgnored(name) {
					continue
				}

				if _, ok := foundConfigEntries[name]; ok {
					continue // already processed this config option
				}

				configOptions = append(configOptions, map[string]interface{}{
					"default": value,
					"name":    name,
				})
				foundConfigEntries[name] = true
			}
		}
	}

	sort.Slice(configOptions, func(i, j int) bool {
		return sort.StringsAreSorted([]string{configOptions[i]["name"].(string), configOptions[j]["name"].(string)})
	})

	return []util.Stream{
		{
			Input:       moduleName + "/metrics",
			Title:       fmt.Sprintf("%s %s logs", strings.Title(moduleName), strings.Title(datasetName)),
			Description: fmt.Sprintf("Collect %s %s metrics", strings.Title(moduleName), strings.Title(datasetName)),
			Vars:        configOptions,
		},
	}, nil
}

func mergeMetaConfigFiles(modulePath string) ([]byte, error) {
	configFilePaths, err := filepath.Glob(filepath.Join(modulePath, "_meta", "config*.yml"))
	if err != nil {
		return nil, errors.Wrapf(err, "location config files failed (modulePath: %s)", modulePath)
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

func shouldConfigOptionBeIgnored(optionName string) bool {
	for _, ignored := range ignoredConfigOptions {
		if ignored == optionName {
			return true
		}
	}
	return false
}

func isConfigEntryRelatedToMetricset(entry mapStr, moduleName, datasetName string) (bool, error) {
	var metricsetRelated bool
	if metricsets, ok := entry["metricsets"]; ok {
		metricsetsMapped, ok := metricsets.([]interface{})
		if !ok {
			return false, fmt.Errorf("mapping metricsets failed (moduleName: %s, datasetName: %s)",
				moduleName, datasetName)
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
