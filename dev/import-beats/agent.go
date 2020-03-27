// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/package-registry/util"
)

type agentContent struct {
	streams []streamContent
}

type streamContent struct {
	targetFileName string
	body           []byte
}

func createAgentContent(modulePath, moduleName, datasetName, beatType string, streams []util.Stream) (agentContent, error) {
	switch beatType {
	case "logs":
		return createAgentContentForLogs(modulePath, datasetName)
	case "metrics":
		return createAgentContentForMetrics(modulePath, moduleName, datasetName, streams)
	}
	return agentContent{}, fmt.Errorf("invalid beat type: %s", beatType)
}

func createAgentContentForLogs(modulePath, datasetName string) (agentContent, error) {
	configFilePaths, err := filepath.Glob(filepath.Join(modulePath, datasetName, "config", "*.yml"))
	if err != nil {
		return agentContent{}, errors.Wrapf(err, "location config files failed (modulePath: %s, datasetName: %s)", modulePath, datasetName)
	}

	if len(configFilePaths) == 0 {
		return agentContent{}, fmt.Errorf("expected at least one config file (modulePath: %s, datasetName: %s)", modulePath, datasetName)
	}

	var buffer bytes.Buffer

	for _, configFilePath := range configFilePaths {
		configFile, err := transformAgentConfigFile(configFilePath)
		if err != nil {
			return agentContent{}, errors.Wrapf(err, "loading config file failed (modulePath: %s, datasetName: %s)", modulePath, datasetName)
		}

		inputConfigName := extractInputConfigName(configFilePath)
		if len(configFilePaths) > 1 {
			buffer.WriteString(fmt.Sprintf("{{#if input == %s}}\n", inputConfigName))
		}
		buffer.Write(configFile)
		if len(configFilePaths) > 1 {
			buffer.WriteString("{{/if}}\n")
		}
	}
	return agentContent{
		streams: []streamContent{
			{
				targetFileName: "stream.yml",
				body:           buffer.Bytes(),
			},
		},
	}, nil
}

func extractInputConfigName(configFilePath string) string {
	i := strings.LastIndex(configFilePath, "/")
	inputConfigName := configFilePath[i+1:]
	j := strings.Index(inputConfigName, ".")
	return inputConfigName[:j]
}

func transformAgentConfigFile(configFilePath string) ([]byte, error) {
	var buffer bytes.Buffer

	configFile, err := os.Open(configFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "opening agent config file failed (path: %s)", configFilePath)
	}

	scanner := bufio.NewScanner(configFile)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "type: ") {
			line = strings.ReplaceAll(line, "type: ", "input: ")
		}

		// simple cases: if, range, -}}
		line = strings.ReplaceAll(line, "{{ ", "{{")
		line = strings.ReplaceAll(line, " }}", "}}")
		line = strings.ReplaceAll(line, "{{if .", "{{if this.")
		line = strings.ReplaceAll(line, "{{if", "{{#if")
		line = strings.ReplaceAll(line, "{{end}}", "{{/end}}")
		line = strings.ReplaceAll(line, "{{.", "{{this.")
		line = strings.ReplaceAll(line, "{{range .", "{{#each this.")
		line = strings.ReplaceAll(line, ".}}", "}}")
		line = strings.ReplaceAll(line, " -}}", "}}") // no support for cleaning trailing white characters?
		line = strings.ReplaceAll(line, "{{- ", "{{") // no support for cleaning trailing white characters?

		// if/else if eq
		if strings.Contains(line, " eq ") {
			line = strings.ReplaceAll(line, " eq .", " ")
			line = strings.ReplaceAll(line, " eq ", " ")

			skipSpaces := 1
			if strings.HasPrefix(line, "{{else") {
				skipSpaces = 2
			}

			splitCondition := strings.SplitN(line, " ", skipSpaces+2)
			line = strings.Join(splitCondition[:len(splitCondition)-1], " ") + " == " +
				splitCondition[len(splitCondition)-1]
		}

		if strings.Contains(line, "{{range ") || strings.Contains(line, " range ") {
			loopedVar, err := extractRangeVar(line)
			if err != nil {
				return nil, errors.Wrapf(err, "extracting range var failed")
			}

			line = fmt.Sprintf("{{#each %s}}\n", loopedVar)
			line += "  - {{this}}\n"
			line += "{{/each}}"

			for scanner.Scan() { // skip all lines inside range
				rangeLine := scanner.Text()
				if strings.Contains(rangeLine, "{{ end }}") {
					break
				}
			}
		}

		buffer.WriteString(line)
		buffer.WriteString("\n")
	}
	return buffer.Bytes(), nil
}

func extractRangeVar(line string) (string, error) {
	line = line[strings.Index(line, "range")+1:]
	line = strings.ReplaceAll(line, "}}", "")
	i := strings.Index(line, ":=")
	var sliced string
	if i >= 0 {
		line = strings.TrimSpace(line[i+3:])
		split := strings.Split(line, " ")
		sliced = split[0]
	} else {
		split := strings.Split(line, " ")
		sliced = split[1]
	}

	if strings.HasPrefix(sliced, ".") {
		sliced = sliced[1:]
	}
	return sliced, nil
}

func createAgentContentForMetrics(modulePath, moduleName, datasetName string, streams []util.Stream) (agentContent, error) {
	inputName := moduleName + "/metrics"
	vars := extractVarsFromStream(streams, inputName)

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("input: %s\n", inputName))
	buffer.WriteString(fmt.Sprintf("metricsets: [\"%s\"]\n", datasetName))

	for _, aVar := range vars {
		variableName := aVar["name"].(string)

		if !isAgentConfigOptionRequired(variableName) {
			buffer.WriteString(fmt.Sprintf("{{#if %s}}\n", variableName))
		}
		buffer.WriteString(fmt.Sprintf("%s: {{%s}}\n", variableName, variableName))
		if !isAgentConfigOptionRequired(variableName) {
			buffer.WriteString(fmt.Sprintf("{{#if %s}}\n", variableName))
		}
	}
	return agentContent{
		streams: []streamContent{
			{
				targetFileName: "stream.yml",
				body:           buffer.Bytes(),
			},
		},
	}, nil
}

func extractVarsFromStream(streams []util.Stream, inputName string) []map[string]interface{} {
	for _, stream := range streams {
		if stream.Input == inputName {
			return stream.Vars
		}
	}
	return []map[string]interface{}{}
}

func isAgentConfigOptionRequired(optionName string) bool {
	return optionName == "hosts" || optionName == "period"
}
