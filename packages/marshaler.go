// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"encoding/json"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
)

type MarshallerOption func(packages *Packages) error

func MarshalJSON(pkgs *Packages) ([]byte, error) {
	return json.MarshalIndent(pkgs, " ", " ")
}

func UnmarshalJSON(content []byte, pkgs *Packages, options ...MarshallerOption) error {
	err := json.Unmarshal(content, pkgs)
	if err != nil {
		return err
	}

	for i := range *pkgs {
		err = (*pkgs)[i].setRuntimeFields()
		if err != nil {
			return err
		}
	}

	for _, opt := range options {
		err = opt(pkgs)
		if err != nil {
			return err
		}
	}
	return nil
}

func ResolveBasePaths(packagesPath ...string) MarshallerOption {
	return func(packages *Packages) error {
		for i := range *packages {
			var manifestFound bool
			for _, pp := range packagesPath {
				maybePath := filepath.Join(pp, (*packages)[i].Name, (*packages)[i].Version)
				maybeManifestPath := filepath.Join(maybePath, "manifest.yml")
				_, err := os.Stat(maybeManifestPath)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				if err == nil {
					(*packages)[i].BasePath = maybePath
					manifestFound = true
					break
				}
			}
			if !manifestFound {
				return errors.Errorf("manifest file is missing (package: %s, version: %s)", (*packages)[i].Name, (*packages)[i].Version)
			}
		}
		return nil
	}
}
