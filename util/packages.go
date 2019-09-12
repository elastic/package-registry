// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"io/ioutil"
)

var packageList []Package

// GetPackages returns a slice with all existing packages.
// The list is stored in memory and on the second request directly
// served from memory. This assumes chnages to packages only happen on restart.
// Caching the packages request many file reads every time this method is called.
func GetPackages(packagesBasePath string) ([]Package, error) {
	if packageList != nil {
		return packageList, nil
	}

	packagePaths, err := getPackagePaths(packagesBasePath)
	if err != nil {
		return nil, err
	}

	for _, i := range packagePaths {
		p, err := NewPackage(packagesBasePath, i)
		if err != nil {
			return nil, err
		}
		packageList = append(packageList, *p)
	}
	return packageList, nil
}

// getPackagePaths returns list of available packages, one for each version.
func getPackagePaths(packagesPath string) ([]string, error) {

	files, err := ioutil.ReadDir(packagesPath)
	if err != nil {
		return nil, err
	}

	var packages []string
	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		packages = append(packages, f.Name())
	}

	return packages, nil
}
