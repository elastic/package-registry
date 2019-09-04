package util

import "io/ioutil"

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
