// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"

	ucfg "github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
)

const (
	defaultType = "integration"
	// Prefix used for all assets served for a package
	packagePathPrefix = "/package"
)

var CategoryTitles = map[string]string{
	"aws":               "AWS",
	"azure":             "Azure",
	"cloud":             "Cloud",
	"config_management": "Config management",
	"containers":        "Containers",
	"crm":               "CRM",
	"custom":            "Custom",
	"datastore":         "Datastore",
	"elastic_stack":     "Elastic Stack",
	"google_cloud":      "Google Cloud",
	"kubernetes":        "Kubernetes",
	"languages":         "Languages",
	"message_queue":     "Message Queue",
	"monitoring":        "Monitoring",
	"network":           "Network",
	"notification":      "Notification",
	"os_system":         "OS & System",
	"productivity":      "Productivity",
	"security":          "Security",
	"support":           "Support",
	"ticketing":         "Ticketing",
	"version_control":   "Version Control",
	"web":               "Web",
}

type Package struct {
	BasePackage   `config:",inline" json:",inline" yaml:",inline"`
	FormatVersion string `config:"format_version" json:"format_version" yaml:"format_version"`

	Readme          *string `config:"readme,omitempty" json:"readme,omitempty" yaml:"readme,omitempty"`
	License         string  `config:"license,omitempty" json:"license,omitempty" yaml:"license,omitempty"`
	versionSemVer   *semver.Version
	Categories      []string         `config:"categories" json:"categories"`
	Conditions      *Conditions      `config:"conditions,omitempty" json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Screenshots     []Image          `config:"screenshots,omitempty" json:"screenshots,omitempty" yaml:"screenshots,omitempty"`
	Assets          []string         `config:"assets,omitempty" json:"assets,omitempty" yaml:"assets,omitempty"`
	PolicyTemplates []PolicyTemplate `config:"policy_templates,omitempty" json:"policy_templates,omitempty" yaml:"policy_templates,omitempty"`
	DataStreams     []*DataStream    `config:"data_streams,omitempty" json:"data_streams,omitempty" yaml:"data_streams,omitempty"`
	Owner           *Owner           `config:"owner,omitempty" json:"owner,omitempty" yaml:"owner,omitempty"`
	Vars            []Variable       `config:"vars" json:"vars,omitempty" yaml:"vars,omitempty"`

	// Local path to the package dir
	BasePath string `json:"-" yaml:"-"`
}

// BasePackage is used for the output of the package info in the /search endpoint
type BasePackage struct {
	Name                string               `config:"name" json:"name"`
	Title               *string              `config:"title,omitempty" json:"title,omitempty" yaml:"title,omitempty"`
	Version             string               `config:"version" json:"version"`
	Release             string               `config:"release,omitempty" json:"release,omitempty"`
	Description         string               `config:"description" json:"description"`
	Type                string               `config:"type" json:"type"`
	Download            string               `json:"download" yaml:"download,omitempty"`
	Path                string               `json:"path" yaml:"path,omitempty"`
	Icons               []Image              `config:"icons,omitempty" json:"icons,omitempty" yaml:"icons,omitempty"`
	Internal            bool                 `config:"internal,omitempty" json:"internal,omitempty" yaml:"internal,omitempty"`
	BasePolicyTemplates []BasePolicyTemplate `json:"policy_templates,omitempty"`
}

// BasePolicyTemplate is used for the package policy templates in the /search endpoint
type BasePolicyTemplate struct {
	Name        string  `config:"name" json:"name" validate:"required"`
	Title       string  `config:"title" json:"title" validate:"required"`
	Description string  `config:"description" json:"description" validate:"required"`
	Icons       []Image `config:"icons,omitempty" json:"icons,omitempty" yaml:"icons,omitempty"`
}

type PolicyTemplate struct {
	Name        string   `config:"name" json:"name" validate:"required"`
	Title       string   `config:"title" json:"title" validate:"required"`
	Description string   `config:"description" json:"description" validate:"required"`
	DataStreams []string `config:"data_streams,omitempty" json:"data_streams,omitempty" yaml:"data_streams,omitempty"`
	Inputs      []Input  `config:"inputs" json:"inputs"`
	Multiple    *bool    `config:"multiple" json:"multiple,omitempty" yaml:"multiple,omitempty"`
	Icons       []Image  `config:"icons,omitempty" json:"icons,omitempty" yaml:"icons,omitempty"`
	Categories  []string `config:"categories,omitempty" json:"categories,omitempty" yaml:"categories,omitempty"`
	Screenshots []Image  `config:"screenshots,omitempty" json:"screenshots,omitempty" yaml:"screenshots,omitempty"`
	Readme      *string  `config:"readme,omitempty" json:"readme,omitempty" yaml:"readme,omitempty"`
}

type Conditions struct {
	KibanaVersion    string `config:"kibana.version,omitempty" json:"kibana.version,omitempty" yaml:"kibana.version,omitempty"`
	kibanaConstraint *semver.Constraints
}

type Version struct {
	Min string `config:"min,omitempty" json:"min,omitempty"`
	Max string `config:"max,omitempty" json:"max,omitempty"`
}

type Owner struct {
	Github string `config:"github,omitempty" json:"github,omitempty"`
}

type Image struct {
	// Src is relative inside the package
	Src string `config:"src" json:"src" validate:"required"`
	// Path is the absolute path in the url
	// TODO: remove yaml struct tag once mage ImportBeats is removed from elastic/integrations repo.
	Path  string `config:"path" json:"path" yaml:"path,omitempty"`
	Title string `config:"title" json:"title,omitempty"`
	Size  string `config:"size" json:"size,omitempty"`
	Type  string `config:"type" json:"type,omitempty"`
}

func (i Image) getPath(p *Package) string {
	return path.Join(packagePathPrefix, p.Name, p.Version, i.Src)
}

type Download struct {
	Path string `config:"path" json:"path" validate:"required"`
	Type string `config:"type" json:"type" validate:"required"`
}

func NewDownload(p Package, t string) Download {
	return Download{
		Path: getDownloadPath(p, t),
		Type: t,
	}
}

func getDownloadPath(p Package, t string) string {
	return path.Join("/epr", p.Name, p.Name+"-"+p.Version+".zip")
}

// NewPackage creates a new package instances based on the given base path.
// The path passed goes to the root of the package where the manifest.yml is.
func NewPackage(basePath string) (*Package, error) {

	manifest, err := yaml.NewConfigWithFile(filepath.Join(basePath, "manifest.yml"), ucfg.PathSep("."))
	if err != nil {
		return nil, err
	}

	var p = &Package{
		BasePath: basePath,
	}
	err = manifest.Unpack(p, ucfg.PathSep("."))
	if err != nil {
		return nil, err
	}

	// Default for the multiple flags is true.
	trueValue := true
	for i, _ := range p.PolicyTemplates {
		if p.PolicyTemplates[i].Multiple == nil {
			p.PolicyTemplates[i].Multiple = &trueValue
		}

		// Collect basic information from policy templates and store into the /search endpoint
		t := p.PolicyTemplates[i]
		baseT := BasePolicyTemplate{
			Name:        t.Name,
			Title:       t.Title,
			Description: t.Description,
		}

		for k, i := range p.PolicyTemplates[i].Icons {
			t.Icons[k].Path = i.getPath(p)
		}

		baseT.Icons = t.Icons
		p.BasePolicyTemplates = append(p.BasePolicyTemplates, baseT)

		// Store paths for all screenshots under each policy template
		if p.PolicyTemplates[i].Screenshots != nil {
			for k, s := range p.PolicyTemplates[i].Screenshots {
				p.PolicyTemplates[i].Screenshots[k].Path = s.getPath(p)
			}
		}

		// Store policy template specific README
		readmePath := filepath.Join(p.BasePath, "docs", p.PolicyTemplates[i].Name+".md")
		readme, err := os.Stat(readmePath)
		if err != nil {
			if _, ok := err.(*os.PathError); !ok {
				return nil, fmt.Errorf("failed to find %s file: %s", p.PolicyTemplates[i].Name+".md", err)
			}
		} else if readme != nil {
			if readme.IsDir() {
				return nil, fmt.Errorf("%s.md is a directory", p.PolicyTemplates[i].Name)
			}
			readmePathShort := path.Join(packagePathPrefix, p.Name, p.Version, "docs", p.PolicyTemplates[i].Name+".md")
			p.PolicyTemplates[i].Readme = &readmePathShort
		}
	}

	if p.Type == "" {
		p.Type = defaultType
	}

	// If not license is set, basic is assumed
	if p.License == "" {
		p.License = DefaultLicense
	}

	p.versionSemVer, err = semver.StrictNewVersion(p.Version)
	if err != nil {
		return nil, errors.Wrap(err, "invalid package version")
	}

	if p.Icons != nil {
		for k, i := range p.Icons {
			p.Icons[k].Path = i.getPath(p)
		}
	}

	if p.Screenshots != nil {
		for k, s := range p.Screenshots {
			p.Screenshots[k].Path = s.getPath(p)
		}
	}

	if p.Conditions != nil && p.Conditions.KibanaVersion != "" {
		p.Conditions.kibanaConstraint, err = semver.NewConstraint(p.Conditions.KibanaVersion)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid Kibana versions range: %s", p.Conditions.KibanaVersion)
		}
	}

	if p.Release == "" {
		p.Release = DefaultRelease
	}

	if !IsValidRelease(p.Release) {
		return nil, fmt.Errorf("invalid release: %s", p.Release)
	}

	readmePath := filepath.Join(p.BasePath, "docs", "README.md")
	// Check if readme
	readme, err := os.Stat(readmePath)
	if err != nil {
		return nil, fmt.Errorf("no readme file found, README.md is required: %s", err)
	}

	if readme != nil {
		if readme.IsDir() {
			return nil, fmt.Errorf("README.md is a directory")
		}
		readmePathShort := path.Join(packagePathPrefix, p.Name, p.Version, "docs", "README.md")
		p.Readme = &readmePathShort
	}

	// Assign download path to be part of the output
	p.Download = p.GetDownloadPath()
	p.Path = p.GetUrlPath()

	err = p.LoadAssets()
	if err != nil {
		return nil, errors.Wrapf(err, "loading package assets failed (path '%s')", p.BasePath)
	}

	err = p.LoadDataSets()
	if err != nil {
		return nil, errors.Wrapf(err, "loading package dataStreams failed (path '%s')", p.BasePath)
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

	// If the version is not specified, it is for all versions
	if p.Conditions == nil || version == nil || p.Conditions.kibanaConstraint == nil {
		return true
	}

	return p.Conditions.kibanaConstraint.Check(version)
}

func (p *Package) IsNewerOrEqual(pp Package) bool {
	return !p.versionSemVer.LessThan(pp.versionSemVer)
}

// LoadAssets (re)loads all the assets of the package
// Based on the time when this is called, it might be that not all assets for a package exist yet, so it is reset every time.
func (p *Package) LoadAssets() (err error) {
	// Reset Assets
	p.Assets = nil

	// Iterates recursively through all the levels to find assets
	// If we need more complex matching a library like https://github.com/bmatcuk/doublestar
	// could be used but the below works and is pretty simple.
	assets, err := collectAssets(filepath.Join(p.BasePath, "*"))
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
			if strings.Contains(info.Name(), "-") {
				return fmt.Errorf("directory name inside package %s contains -: %s", p.Name, a)
			}
			continue
		}

		// Strip away the basePath from the local system
		a = a[len(p.BasePath)+1:]
		a = path.Join(packagePathPrefix, p.GetPath(), a)
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
		a, err := collectAssets(filepath.Join(pattern, "*"))
		if err != nil {
			return nil, err
		}
		return append(assets, a...), nil
	}
	return nil, nil
}

// Validate is called during Unpack of the manifest.
// The validation here is only related to the fields directly specified in the manifest itself.
func (p *Package) Validate() error {
	if PackageValidationDisabled {
		return nil
	}

	if p.FormatVersion == "" {
		return fmt.Errorf("no format_version set: %v", p)
	}

	_, err := semver.StrictNewVersion(p.FormatVersion)
	if err != nil {
		return fmt.Errorf("invalid package version: %s, %s", p.FormatVersion, err)
	}

	_, err = semver.StrictNewVersion(p.Version)
	if err != nil {
		return err
	}

	if p.Title == nil || *p.Title == "" {
		return fmt.Errorf("no title set for package: %s", p.Name)
	}

	if p.Description == "" {
		return fmt.Errorf("no description set")
	}

	for _, c := range p.Categories {
		if _, ok := CategoryTitles[c]; !ok {
			return fmt.Errorf("invalid category: %s", c)
		}
	}

	for _, i := range p.Icons {
		_, err := os.Stat(filepath.Join(p.BasePath, i.Src))
		if err != nil {
			return err
		}
	}

	for _, s := range p.Screenshots {
		_, err := os.Stat(filepath.Join(p.BasePath, s.Src))
		if err != nil {
			return err
		}
	}

	err = p.validateVersionConsistency()
	if err != nil {
		return errors.Wrap(err, "version in manifest file is not consistent with path")
	}

	return p.ValidateDataStreams()
}

func (p *Package) validateVersionConsistency() error {
	versionPackage, err := semver.NewVersion(p.Version)
	if err != nil {
		return errors.Wrap(err, "invalid version defined in manifest")
	}

	baseDir := filepath.Base(p.BasePath)
	versionDir, err := semver.NewVersion(baseDir)
	if err != nil {
		// TODO: There should be a flag passed to the registry to accept these kind of packages
		// as otherwise these could hide some errors in the structure of the package-storage
		return nil // package content is not rooted in version directory
	}

	if !versionPackage.Equal(versionDir) {
		return fmt.Errorf("inconsistent versions (path: %s, manifest: %s)", versionDir.String(), p.versionSemVer.String())
	}
	return nil
}

// GetDataStreamPaths returns a list with the dataStream paths inside this package
func (p *Package) GetDataStreamPaths() ([]string, error) {
	dataStreamBasePath := filepath.Join(p.BasePath, "data_stream")

	// Check if this package has dataStreams
	_, err := os.Stat(dataStreamBasePath)
	// If no dataStreams exist, just return
	if os.IsNotExist(err) {
		return nil, nil
	}
	// An other error happened, report it
	if err != nil {
		return nil, err
	}

	paths, err := filepath.Glob(filepath.Join(dataStreamBasePath, "*"))
	if err != nil {
		return nil, err
	}

	for i, _ := range paths {
		paths[i] = paths[i][len(dataStreamBasePath)+1:]
	}

	return paths, nil
}

func (p *Package) LoadDataSets() error {

	dataStreamPaths, err := p.GetDataStreamPaths()
	if err != nil {
		return err
	}

	dataStreamsBasePath := filepath.Join(p.BasePath, "data_stream")

	for _, dataStreamPath := range dataStreamPaths {

		dataStreamBasePath := filepath.Join(dataStreamsBasePath, dataStreamPath)

		d, err := NewDataStream(dataStreamBasePath, p)
		if err != nil {
			return err
		}

		// TODO: Validate that each input specified in a stream also is defined in the package

		p.DataStreams = append(p.DataStreams, d)
	}

	return nil
}

// ValidateDataStreams loads all dataStreams and with it validates them
func (p *Package) ValidateDataStreams() error {
	dataStreamPaths, err := p.GetDataStreamPaths()
	if err != nil {
		return err
	}

	dataStreamsBasePath := filepath.Join(p.BasePath, "data_stream")
	for _, dataStreamPath := range dataStreamPaths {
		dataStreamBasePath := filepath.Join(dataStreamsBasePath, dataStreamPath)

		d, err := NewDataStream(dataStreamBasePath, p)
		if err != nil {
			return errors.Wrapf(err, "building data stream failed (path: %s)", dataStreamBasePath)
		}

		err = d.Validate()
		if err != nil {
			return errors.Wrapf(err, "validating data stream failed (path: %s)", dataStreamBasePath)
		}
	}
	return nil
}

func (p *Package) GetPath() string {
	return p.Name + "/" + p.Version
}

func (p *Package) GetDownloadPath() string {
	return path.Join("/epr", p.Name, p.Name+"-"+p.Version+".zip")
}

func (p *Package) GetUrlPath() string {
	return path.Join(packagePathPrefix, p.Name, p.Version)
}
