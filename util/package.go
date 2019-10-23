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

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
)

const defaultType = "integration"

var CategoryTitles = map[string]string{
	"logs":    "Logs",
	"metrics": "Metrics",
}

type Package struct {
	Name          string  `yaml:"name" json:"name"`
	Title         *string `yaml:"title,omitempty" json:"title,omitempty"`
	Version       string  `yaml:"version" json:"version"`
	Readme        *string `yaml:"readme,omitempty" json:"readme,omitempty"`
	versionSemVer semver.Version
	Description   string      `yaml:"description" json:"description"`
	Type          string      `yaml:"type" json:"type"`
	Categories    []string    `yaml:"categories" json:"categories"`
	Requirement   Requirement `yaml:"requirement" json:"requirement"`
	Screenshots   []Image     `yaml:"screenshots,omitempty" json:"screenshots,omitempty"`
	Icons         []Image     `yaml:"icons,omitempty" json:"icons,omitempty"`
	Assets        []string    `yaml:"assets,omitempty" json:"assets,omitempty"`
	Internal      bool        `yaml:"internal,omitempty" json:"internal,omitempty"`
}

type Requirement struct {
	Kibana Kibana `yaml:"kibana" json:"kibana"`
}

type Kibana struct {
	Version   Version `yaml:"version,omitempty" json:"version,omitempty"`
	minSemVer semver.Version
	maxSemVer semver.Version
}

type Version struct {
	Min string `yaml:"min,omitempty" json:"min,omitempty"`
	Max string `yaml:"max,omitempty" json:"max,omitempty"`
}

type Image struct {
	Src   string `yaml:"src" json:"src,omitempty"`
	Title string `yaml:"title" json:"title,omitempty"`
	Size  string `yaml:"size" json:"size,omitempty"`
	Type  string `yaml:"type" json:"type,omitempty"`
}

func (i Image) getPath(p *Package) string {
	return "/package/" + p.Name + "-" + p.Version + i.Src
}

// NewPackage creates a new package instances based on the given base path + package name.
// The package name passed contains the version of the package.
func NewPackage(basePath, packageName string) (*Package, error) {

	manifest, err := yaml.NewConfigWithFile(basePath+"/"+packageName+"/manifest.yml", ucfg.PathSep("."))

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

	if p.Requirement.Kibana.Version.Max != "" {
		p.Requirement.Kibana.maxSemVer, err = semver.Parse(p.Requirement.Kibana.Version.Max)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid Kibana max version: %s", p.Requirement.Kibana.Version.Max)
		}
	}

	if p.Requirement.Kibana.Version.Min != "" {
		p.Requirement.Kibana.minSemVer, err = semver.Parse(p.Requirement.Kibana.Version.Min)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid Kibana min version: %s", p.Requirement.Kibana.Version.Min)
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
	if version != nil {
		if p.Requirement.Kibana.Version.Max != "" {
			if version.GT(p.Requirement.Kibana.maxSemVer) {
				return false
			}
		}

		if p.Requirement.Kibana.Version.Min != "" {
			if version.LT(p.Requirement.Kibana.minSemVer) {
				return false
			}
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

	assets, err := filepath.Glob("*")
	if err != nil {
		return err
	}

	a, err := filepath.Glob("*/*")
	if err != nil {
		return err
	}
	assets = append(assets, a...)

	a, err = filepath.Glob("*/*/*")
	if err != nil {
		return err
	}
	assets = append(assets, a...)

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

func (p *Package) Validate() error {

	if p.Title == nil || *p.Title == "" {
		return fmt.Errorf("no title set")
	}

	if p.Description == "" {
		return fmt.Errorf("no description set")
	}

	if p.Requirement.Kibana.Version.Max != "" {
		_, err := semver.Parse(p.Requirement.Kibana.Version.Max)
		if err != nil {
			return fmt.Errorf("invalid max kibana version: %s, %s", p.Requirement.Kibana.Version.Max, err)
		}
	}

	if p.Requirement.Kibana.Version.Min != "" {
		_, err := semver.Parse(p.Requirement.Kibana.Version.Min)
		if err != nil {
			return fmt.Errorf("invalid min Kibana version: %s, %s", p.Requirement.Kibana.Version.Min, err)
		}
	}

	for _, c := range p.Categories {
		if _, ok := CategoryTitles[c]; !ok {
			return fmt.Errorf("invalid category: %s", c)
		}
	}

	return nil
}

func (p *Package) GetPath() string {
	return p.Name + "-" + p.Version
}
