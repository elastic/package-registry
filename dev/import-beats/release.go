// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"io/ioutil"
	"os"
	"path"

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

	moduleRelease, err := loadReleaseFromFields(path.Join(modulePath, "_meta"))
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

func determineDatasetRelease(moduleRelease, datasetPath string) (string, error) {
	datasetRelease, err := loadReleaseFromFields(path.Join(datasetPath, "_meta"))
	if err != nil {
		return "", errors.Wrapf(err, "loading dataset release failed (path: %s)", datasetPath)
	}

	if datasetRelease != "" {
		return datasetRelease, nil
	}
	return moduleRelease, nil
}

func loadReleaseFromFields(metaDir string) (string, error) {
	fieldsFilePath := path.Join(metaDir, "fields.yml")
	fieldsFile, err := os.Open(fieldsFilePath)
	if err != nil {
		return "", errors.Wrapf(err, "opening fields file failed (path: %s)", fieldsFilePath)
	}

	fields, err := ioutil.ReadAll(fieldsFile)
	if err != nil {
		return "", errors.Wrapf(err, "reading fields file failed (path: %s)", fieldsFilePath)
	}

	var frs []fieldsRelease
	err = yaml.Unmarshal(fields, &frs)
	if err != nil {
		return "", errors.Wrapf(err, "unmarshalling fields file failed (path: %s)", fieldsFilePath)
	}

	if len(frs) == 0 {
		return "", nil // reporting: release not set, but it's accepted
	}

	return frs[0].Release, nil
}
