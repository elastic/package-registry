// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type fieldsData struct {
	Title   string `yaml:"title"`
	Release string `yaml:"release"`
}

// determinePackageRelease function considers all release tags for modules and justifies a common release tag.
// If any of module has been released as "GA", then the package will be released as "GA",
// else if any of module has been release as "beta", then the package will be released as "beta",
// otherwise "experimental".
func determinePackageRelease(manifestRelease string, moduleFields []byte) (string, error) {
	if manifestRelease == "ga" { // manifestRelease
		return "ga", nil
	}

	moduleRelease, err := loadReleaseFromFields(moduleFields)
	if err != nil {
		return "", errors.Wrapf(err, "loading module release failed")
	}

	if moduleRelease == "" || moduleRelease == "ga" {
		return "ga", nil // missing fields.release means "GA"
	}

	if moduleRelease == "beta" || manifestRelease == "beta" {
		return "beta", nil
	}
	return "experimental", nil
}

func determineDatasetRelease(moduleRelease string, datasetFields []byte) (string, error) {
	datasetRelease, err := loadReleaseFromFields(datasetFields)
	if err != nil {
		return "", errors.Wrapf(err, "loading dataset release failed")
	}

	if datasetRelease != "" {
		return datasetRelease, nil
	}
	return moduleRelease, nil
}

func loadReleaseFromFields(fields []byte) (string, error) {
	f, err := unmarshalFirstFieldsData(fields)
	if err != nil {
		return "", errors.Wrapf(err, "unmarshalling fields data failed")
	}
	return f.Release, nil
}

func loadTitleFromFields(fields []byte) (string, error) {
	f, err := unmarshalFirstFieldsData(fields)
	if err != nil {
		return "", errors.Wrapf(err, "unmarshalling fields data failed")
	}
	return f.Title, nil
}

func unmarshalFirstFieldsData(fields []byte) (fieldsData, error) {
	var fs []fieldsData
	err := yaml.Unmarshal(fields, &fs)
	if err != nil {
		return fieldsData{}, errors.Wrapf(err, "unmarshalling fields file failed")
	}
	if len(fs) > 0 {
		return fs[0], nil
	}
	return fieldsData{}, nil
}
