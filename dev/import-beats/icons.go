// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"github.com/pkg/errors"
)

var errIconNotFound = errors.New("icon not found")

type iconRepository struct {
	icons map[string]string
}

func newIconRepository(euiDir, kibanaDir string) (*iconRepository, error) {
	icons, err := populateIconRepository(euiDir, kibanaDir)
	if err != nil {
		return nil, errors.Wrapf(err, "populating icon repository failed")
	}
	return &iconRepository{icons: icons}, nil
}

func populateIconRepository(euiDir, kibanaDir string) (map[string]string, error) {
	// TODO
	return nil, nil
}

func (ir *iconRepository) iconForModule(moduleName string) (imageContent, error) {
	source, ok := ir.icons[moduleName]
	if !ok {
		return imageContent{}, errIconNotFound
	}
	return imageContent{source: source}, nil
}

func createIcons(iconRepository *iconRepository, moduleName string) ([]imageContent, error) {
	anIcon, err := iconRepository.iconForModule(moduleName)
	if err == errIconNotFound {
		return []imageContent{}, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "fetching icon for module failed (moduleName: %s)", moduleName)
	}
	return []imageContent{anIcon}, nil
}
