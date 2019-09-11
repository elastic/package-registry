package util

import (
	"io/ioutil"
)

var packageList []Package

// GetPackagePaths returns list of available packages, one for each version.
func GetPackagePaths(packagesPath string) ([]string, error) {

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

func GetPackages(packagesBasePath string) ([]Package, error) {
	if packageList != nil {
		return packageList, nil
	}

	packagePaths, err := GetPackagePaths(packagesBasePath)
	if err != nil {
		return nil, err
	}

	// Get unique list of newest packages
	for _, i := range packagePaths {
		p, err := NewPackage(packagesBasePath, i)
		if err != nil {
			return nil, err
		}
		packageList = append(packageList, *p)
	}
	return packageList, nil
}
