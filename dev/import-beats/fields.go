// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func loadDatasetFields(modulePath, datasetName string) ([]byte, error) {
	moduleFieldsPath := filepath.Join(modulePath, "_meta", "fields.yml")
	moduleFields, err := ioutil.ReadFile(moduleFieldsPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading module fields file failed (path: %s)", moduleFieldsPath)
	}

	var buffer bytes.Buffer
	buffer.Write(moduleFields)

	datasetFieldsPath := filepath.Join(modulePath, datasetName, "_meta", "fields.yml")
	datasetFieldsFile, err := os.Open(datasetFieldsPath)
	if os.IsNotExist(err) {
		log.Printf("Missing fields.yml file. Skipping. (path: %s)\n", datasetFieldsPath)
		return buffer.Bytes(), nil
	} else if err != nil {
		return nil, errors.Wrapf(err, "reading dataset fields file failed (path: %s)", moduleFieldsPath)
	}
	defer datasetFieldsFile.Close()

	scanner := bufio.NewScanner(datasetFieldsFile)
	for scanner.Scan() {
		line := scanner.Text()
		buffer.Write([]byte("        "))
		buffer.WriteString(line)
		buffer.WriteString("\n")
	}
	return buffer.Bytes(), nil
}
