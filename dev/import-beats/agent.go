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
)

type agentContent struct {
	streams []streamContent
}

type streamContent struct {
	targetFileName string
	body           []byte
}

func createAgentContent(modulePath, moduleName, datasetName, beatType string) (agentContent, error) {
	switch beatType {
	case "logs":
		return createAgentContentForLogs(modulePath, moduleName, datasetName)
	case "metrics":
		return createAgentContentForMetrics(modulePath, moduleName, datasetName)
	}
	return agentContent{}, fmt.Errorf("invalid beat type: %s", beatType)
}

func createAgentContentForLogs(modulePath, moduleName, datasetName string) (agentContent, error) {
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
		if strings.HasPrefix(line, "type: ") {
			line = strings.ReplaceAll(line, "type: ", "input: ")
			buffer.WriteString(line)
			buffer.WriteString("\n")
		} else if strings.Contains(line, "if eq .") {
			if strings.Contains(line, "{{ else") {
				buffer.WriteString("{{else if ")
			} else {
				buffer.WriteString("{{#if ")
			}
			i := strings.Index(line, ".")
			if i < 0 {
				return nil, fmt.Errorf("missing variable in condition, line: %s", line)
			}

			line = line[i+1:]
			var variableName, variableValue string
			_, err := fmt.Sscanf(line, `%s %s }}`, &variableName, &variableValue)
			if err != nil {
				return nil, errors.Wrapf(err, `scanning condition failed, line: '%s'`, line)
			}
			variableValue = strings.ReplaceAll(variableValue, `"`, ``)
			buffer.WriteString(fmt.Sprintf("%s == %s}}\n", variableName, variableValue))
		} else if strings.HasPrefix(line, "{{ if .") {
			line = strings.ReplaceAll(line, "{{ if .", "{{#if ")
			line = strings.ReplaceAll(line, " }}", "}}")
			buffer.WriteString(line)
			buffer.WriteString("\n")
		} else if strings.Contains(line, "{{range ") || strings.Contains(line, " range ") {
			loopedVar, err := extractRangeVar(line)
			if err != nil {
				return nil, errors.Wrapf(err, "extracting range var failed")
			}
			buffer.WriteString(fmt.Sprintf("{{#each %s}}\n", loopedVar))
			buffer.WriteString("  - {{this}}\n")
			buffer.WriteString("{{/each}}\n")

			for scanner.Scan() { // skip all lines inside range
				rangeLine := scanner.Text()
				if strings.Contains(rangeLine, "{{ end }}") {
					break
				}
			}
		} else if strings.HasPrefix(line, "{{ else }}") {
			buffer.WriteString("{{else}}\n")
		} else if strings.HasPrefix(line, "{{ end }}") {
			buffer.WriteString("{{/if}}\n")
		} else if strings.Contains(line, "{{.") || strings.Contains(line, "{{ .") {
			line = strings.ReplaceAll(line, "{{ .", "{{")
			line = strings.ReplaceAll(line, "{{.", "{{")
			line = strings.ReplaceAll(line, " }}", "}}")
			buffer.WriteString(line)
			buffer.WriteString("\n")
		} else if line != "" {
			buffer.WriteString(line)
			buffer.WriteString("\n")
		}
	}
	return buffer.Bytes(), nil
}

func extractRangeVar(line string) (string, error) {
	line = line[strings.Index(line, "range") + 1:]
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

func createAgentContentForMetrics(modulePath, moduleName, datasetName string) (agentContent, error) {
	return agentContent{}, nil // TODO
}
