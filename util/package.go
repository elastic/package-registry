// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/blang/semver"

	ucfg "github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
)

const defaultType = "integration"

var CategoryTitles = map[string]string{
	"logs":    "Logs",
	"metrics": "Metrics",
}

type Package struct {
	Name          string  `config:"name" json:"name"`
	Title         *string `config:"title,omitempty" json:"title,omitempty"`
	Version       string  `config:"version" json:"version"`
	Readme        *string `config:"readme,omitempty" json:"readme,omitempty"`
	versionSemVer semver.Version
	Description   string      `config:"description" json:"description"`
	Type          string      `config:"type" json:"type"`
	Categories    []string    `config:"categories" json:"categories"`
	Requirement   Requirement `config:"requirement" json:"requirement"`
	Screenshots   []Image     `config:"screenshots,omitempty" json:"screenshots,omitempty"`
	Icons         []Image     `config:"icons,omitempty" json:"icons,omitempty"`
	Assets        []string    `config:"assets,omitempty" json:"assets,omitempty"`
	Internal      bool        `config:"internal,omitempty" json:"internal,omitempty"`
	FormatVersion string      `config:"format_version" json:"format_version"`
	DataSets      []*DataSet  `config:"datasets,omitempty" json:"datasets,omitempty"`
	Download      string      `json:"download"`
	Path          string      `json:"path"`
}

type Requirement struct {
	Kibana Kibana `config:"kibana" json:"kibana"`
}

type Kibana struct {
	Versions    string `config:"versions,omitempty" json:"versions,omitempty"`
	semVerRange semver.Range
}

type Version struct {
	Min string `config:"min,omitempty" json:"min,omitempty"`
	Max string `config:"max,omitempty" json:"max,omitempty"`
}

type Image struct {
	Src   string `config:"src" json:"src,omitempty"`
	Title string `config:"title" json:"title,omitempty"`
	Size  string `config:"size" json:"size,omitempty"`
	Type  string `config:"type" json:"type,omitempty"`
}

func (i Image) getPath(p *Package) string {
	return "/package/" + p.Name + "-" + p.Version + i.Src
}

// NewPackage creates a new package instances based on the given base path + package name.
// The package name passed contains the version of the package.
func NewPackage(basePath, packageName string) (*Package, error) {

	manifest, err := yaml.NewConfigWithFile(basePath+"/"+packageName+"/manifest.yml", ucfg.PathSep("."))
	if err != nil {
		return nil, err
	}

	var p = &Package{}
	err = manifest.Unpack(p)
	if err != nil {
		return nil, err
	}

	if p.Type == "" {
		p.Type = defaultType
	}

	if p.Icons != nil {
		for k, i := range p.Icons {
			p.Icons[k].Src = i.getPath(p)
		}
	}

	if p.Screenshots != nil {
		for k, s := range p.Screenshots {
			p.Screenshots[k].Src = s.getPath(p)
		}
	}

	if p.Requirement.Kibana.Versions != "" {
		p.Requirement.Kibana.semVerRange, err = semver.ParseRange(p.Requirement.Kibana.Versions)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid Kibana versions range: %s", p.Requirement.Kibana.Versions)
		}
	}

	p.versionSemVer, err = semver.Parse(p.Version)
	if err != nil {
		return nil, err
	}

	readmePath := basePath + "/" + packageName + "/docs/README.md"
	// Check if readme
	readme, err := os.Stat(readmePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if readme != nil {
		if readme.IsDir() {
			return nil, fmt.Errorf("README.md is a directory")
		}
		readmePathShort := "/package/" + packageName + "/docs/README.md"
		p.Readme = &readmePathShort
	}

	// Assign download path to be part of the output
	p.Download = p.GetDownloadPath()
	p.Path = p.GetUrlPath()

	return p, nil
}

func (p *Package) HasCategory(category string) bool {
	for _, c := range p.Categories {
		if c == category {
			return true
		}
	}

	return false
}

func (p *Package) HasKibanaVersion(version *semver.Version) bool {

	// If the version is not specified, it is for all versions
	if p.Requirement.Kibana.Versions == "" {
		return true
	}

	if version != nil {
		if !p.Requirement.Kibana.semVerRange(*version) {
			return false
		}
	}
	return true
}

func (p *Package) IsNewer(pp Package) bool {
	return p.versionSemVer.GT(pp.versionSemVer)
}

// LoadAssets (re)loads all the assets of the package
// Based on the time when this is called, it might be that not all assets for a package exist yet, so it is reset every time.
func (p *Package) LoadAssets(packagePath string) (err error) {
	// Reset Assets
	p.Assets = nil

	oldDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer func() {
		// use named return to also have an error in case the defer fails
		err = os.Chdir(oldDir)
	}()
	err = os.Chdir(packagePath)
	if err != nil {
		return err
	}

	// Iterates recursively through all the levels to find assets
	// If we need more complex matching a library like https://github.com/bmatcuk/doublestar
	// could be used but the below works and is pretty simple.
	assets, err := collectAssets("*")
	if err != nil {
		return err
	}

	for _, a := range assets {
		// Unfortunately these files keep sneaking in
		if strings.Contains(a, ".DS_Store") {
			continue
		}

		info, err := os.Stat(a)
		if err != nil {
			return err
		}

		if info.IsDir() {
			continue
		}

		a = "/package/" + packagePath + "/" + a
		p.Assets = append(p.Assets, a)
	}
	return nil
}

func collectAssets(pattern string) ([]string, error) {
	assets, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(assets) != 0 {
		a, err := collectAssets(pattern + "/*")
		if err != nil {
			return nil, err
		}
		return append(assets, a...), nil
	}
	return nil, nil
}

func (p *Package) Validate() error {

	if p.FormatVersion == "" {
		return fmt.Errorf("no format_version set: %v", p)
	}

	_, err := semver.New(p.FormatVersion)
	if err != nil {
		return fmt.Errorf("invalid package version: %s, %s", p.FormatVersion, err)
	}

	if p.Title == nil || *p.Title == "" {
		return fmt.Errorf("no title set")
	}

	if p.Description == "" {
		return fmt.Errorf("no description set")
	}

	if p.Requirement.Kibana.Versions != "" {
		_, err := semver.ParseRange(p.Requirement.Kibana.Versions)
		if err != nil {
			return fmt.Errorf("invalid kibana versions: %s, %s", p.Requirement.Kibana.Versions, err)
		}
	}

	for _, c := range p.Categories {
		if _, ok := CategoryTitles[c]; !ok {
			return fmt.Errorf("invalid category: %s", c)
		}
	}

	return nil
}

func (p *Package) LoadDataSets(packagePath string) error {

	// Check if this package has datasets
	_, err := os.Stat(packagePath + "/dataset")
	// If no datasets exist, just return
	if os.IsNotExist(err) {
		return nil
	}
	// An other error happened, report it
	if err != nil {
		return err
	}

	oldDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer func() {
		// use named return to also have an error in case the defer fails
		err = os.Chdir(oldDir)
	}()
	err = os.Chdir(packagePath + "/dataset")
	if err != nil {
		return err
	}

	datasetPaths, err := filepath.Glob("*")
	if err != nil {
		return err
	}

	for _, datasetPath := range datasetPaths {
		// Check if manifest exists
		manifestPath := datasetPath + "/manifest.yml"
		_, err := os.Stat(manifestPath)
		if err != nil && os.IsNotExist(err) {
			return errors.Wrapf(err, "manifest does not exist for package: %s", packagePath)
		}

		manifest, err := yaml.NewConfigWithFile(manifestPath, ucfg.PathSep("."))
		var d = &DataSet{
			Name:    dataSetName,
			Package: p.Name,
		}
		// go-ucfg automatically calls the `Validate` method on the Dataset object here
		err = manifest.Unpack(d)
		if err != nil {
			return errors.Wrapf(err, "error building dataset in package: %s", p.Name)
		}

		// This is the name of the directory of the dataset
		d.Path = datasetPath

		// if id is not set, {package}.{datasetName} is the default
		if d.ID == "" {
			d.ID = p.Name + "." + datasetPath
		}

		if d.Release == "" {
			d.Release = "beta"
		}

		p.DataSets = append(p.DataSets, d)
	}

	return nil
}

func (p *Package) GetPath() string {
	return p.Name + "-" + p.Version
}

func (p *Package) GetDownloadPath() string {
	return "/epr/" + p.Name + "/" + p.Name + "-" + p.Version + ".tar.gz"
}

func (p *Package) GetUrlPath() string {
	return "/package/" + p.Name + "-" + p.Version
}
