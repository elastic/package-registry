// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type fields []fieldsEntry

type fieldsEntry struct {
	Key         string
	Title       *string
	Description string
}

func (f fields) getEntry(key string) (*fieldsEntry, error) {
	for _, entry := range f {
		if entry.Key == key {
			return &entry, nil
		}
	}
	return nil, fmt.Errorf("missing entry for key '%s'", key)
}

func loadFields(modulePath string) (fields, error) {
	fieldsFilePath := filepath.Join(modulePath, "fields.yml")

	data, err := ioutil.ReadFile(fieldsFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading file failed (path: %s)", fieldsFilePath)
	}

	var f fields
	err = yaml.Unmarshal(data, &f)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling fields file failed (path: %s)", fieldsFilePath)
	}
	return f, nil
}
