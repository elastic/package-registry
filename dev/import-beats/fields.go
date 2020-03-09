// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func loadModuleFields(modulePath string) ([]byte, error) {
	moduleFieldsPath := filepath.Join(modulePath, "_meta", "fields.yml")
	moduleFieldsFile, err := os.Open(moduleFieldsPath)
	if err != nil {
		return nil, errors.Wrapf(err, "openning module fields file failed (path: %s)", moduleFieldsPath)
	}

	var buffer bytes.Buffer
	scanner := bufio.NewScanner(moduleFieldsFile)
	var fieldsKeyFound bool
	for scanner.Scan() {
		line := scanner.Text()
		if fieldsKeyFound {
			if len(line) > 4 { // move all fields two levels to the left
				buffer.WriteString(line[4:])
				buffer.WriteString("\n")
			}
		} else if strings.TrimLeft(line, " ") == "fields:" {
			fieldsKeyFound = true
		}
	}
	return buffer.Bytes(), nil
}

func loadDatasetFields(modulePath, moduleName, datasetName string) ([]byte, error) {
	var buffer bytes.Buffer

	datasetFieldsPath := filepath.Join(modulePath, datasetName, "_meta", "fields.yml")
	datasetFieldsFile, err := os.Open(datasetFieldsPath)
	if os.IsNotExist(err) {
		log.Printf("Missing fields.yml file. Skipping. (path: %s)\n", datasetFieldsPath)
		return buffer.Bytes(), nil
	} else if err != nil {
		return nil, errors.Wrapf(err, "reading dataset fields file failed (path: %s)", datasetFieldsPath)
	}
	defer datasetFieldsFile.Close()

	scanner := bufio.NewScanner(datasetFieldsFile)
	for scanner.Scan() {
		line := scanner.Text()
		if line == ("- name: " + datasetName) {
			line = fmt.Sprintf("- name: %s.%s", moduleName, datasetName)
		}
		buffer.WriteString(line)
		buffer.WriteString("\n")
	}
	return buffer.Bytes(), nil
}
