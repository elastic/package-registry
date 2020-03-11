// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"github.com/pkg/errors"
)

type iconRepository struct{}

func (ir *iconRepository) iconForModule(moduleName string) (imageContent, error) {
	return imageContent{}, nil // TODO
}

func createIcons(iconRepository *iconRepository, moduleName string) ([]imageContent, error) {
	anIcon, err := iconRepository.iconForModule(moduleName)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching icon for module failed (moduleName: %s)", moduleName)
	}
	return []imageContent{anIcon}, nil
}
