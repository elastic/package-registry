// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type fieldsRelease struct {
	Release string `yaml:"release"`
}

// determinePackageRelease function considers all release tags for modules and justifies a common release tag.
// If any of module has been released as "GA", then the package will be released as "GA",
// else if any of module has been release as "beta", then the package will be released as "beta",
// otherwise "experimental".
func determinePackageRelease(manifestRelease, modulePath string) (string, error) {
	if manifestRelease == "ga" { // manifestRelease
		return "ga", nil
	}

	moduleRelease, err := loadModuleRelease(modulePath)
	if err != nil {
		return "", errors.Wrapf(err, "loading module release failed (path: %s)", modulePath)
	}

	if moduleRelease == "" || moduleRelease == "ga" {
		return "ga", nil // missing fields.release means "GA"
	}

	if moduleRelease == "beta" || manifestRelease == "beta" {
		return "beta", nil
	}
	return "experimental", nil
}

func loadModuleRelease(modulePath string) (string, error) {
	moduleFieldsPath := filepath.Join(modulePath, "_meta", "fields.yml")
	moduleFieldsFile, err := os.Open(moduleFieldsPath)
	if err != nil {
		return "", errors.Wrapf(err, "opening module fields file failed (path: %s)", moduleFieldsPath)
	}

	moduleFields, err := ioutil.ReadAll(moduleFieldsFile)
	if err != nil {
		return "", errors.Wrapf(err, "reading module fields file failed (path: %s)", moduleFieldsPath)
	}

	var frs []fieldsRelease
	err = yaml.Unmarshal(moduleFields, &frs)
	if err != nil {
		return "", errors.Wrapf(err, "unmarshalling module fields file failed (path: %s)", moduleFieldsPath)
	}

	if len(frs) == 0 {
		return "", fmt.Errorf("module fields file is empty (path: %s)", moduleFieldsPath)
	}

	return frs[0].Release, nil
}
