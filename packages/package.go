// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"go.uber.org/zap"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/package-registry/categories"
	"github.com/elastic/package-registry/internal/ingestionmethod"
)

const (
	defaultType = "integration"
	// Prefix used for all assets served for a package
	packagePathPrefix = "/package"
)

var (
	Categories       = categories.DefaultCategories()
	IngestionMethods = ingestionmethod.DefaultIngestionMethod()

	// Deprecated, keeping for backwards compatibility, Categories should be used instead.
	CategoryTiles = categoryTitles(categories.DefaultCategories())
)

type Package struct {
	BasePackage   `config:",inline" json:",inline" yaml:",inline"`
	FormatVersion string `config:"format_version" json:"format_version" yaml:"format_version"`

	Readme          *string               `config:"readme,omitempty" json:"readme,omitempty" yaml:"readme,omitempty"`
	License         string                `config:"license,omitempty" json:"license,omitempty" yaml:"license,omitempty"`
	Screenshots     []Image               `config:"screenshots,omitempty" json:"screenshots,omitempty" yaml:"screenshots,omitempty"`
	Assets          []string              `config:"assets,omitempty" json:"assets,omitempty" yaml:"assets,omitempty"`
	PolicyTemplates []PolicyTemplate      `config:"policy_templates,omitempty" json:"policy_templates,omitempty" yaml:"policy_templates,omitempty"`
	DataStreams     []*DataStream         `config:"data_streams,omitempty" json:"data_streams,omitempty" yaml:"data_streams,omitempty"`
	Vars            []Variable            `config:"vars" json:"vars,omitempty" yaml:"vars,omitempty"`
	Elasticsearch   *PackageElasticsearch `config:"elasticsearch,omitempty" json:"elasticsearch,omitempty" yaml:"elasticsearch,omitempty"`
	Agent           *PackageAgent         `config:"agent,omitempty" json:"agent,omitempty" yaml:"agent,omitempty"`
	// Local path to the package dir
	BasePath string `json:"-" yaml:"-"`

	versionSemVer *semver.Version

	specMajorMinorSemVer *semver.Version

	fsBuilder FileSystemBuilder
	resolver  RemoteResolver
}

type FileSystemBuilder func(*Package) (PackageFileSystem, error)

// BasePackage is used for the output of the package info in the /search endpoint
type BasePackage struct {
	Name                    string               `config:"name" json:"name"`
	Title                   *string              `config:"title,omitempty" json:"title,omitempty" yaml:"title,omitempty"`
	Version                 string               `config:"version" json:"version"`
	Release                 string               `config:"release,omitempty" json:"release,omitempty"`
	Source                  *Source              `config:"source,omitempty" json:"source,omitempty" yaml:"source,omitempty"`
	Description             string               `config:"description" json:"description"`
	Type                    string               `config:"type" json:"type"`
	Download                string               `json:"download" yaml:"download,omitempty"`
	Path                    string               `json:"path" yaml:"path,omitempty"`
	Icons                   []Image              `config:"icons,omitempty" json:"icons,omitempty" yaml:"icons,omitempty"`
	PolicyTemplatesBehavior string               `config:"policy_templates_behavior,omitempty" json:"policy_templates_behavior,omitempty" yaml:"policy_templates_behavior,omitempty"`
	BasePolicyTemplates     []BasePolicyTemplate `json:"policy_templates,omitempty"`
	Conditions              *Conditions          `config:"conditions,omitempty" json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Owner                   *Owner               `config:"owner,omitempty" json:"owner,omitempty" yaml:"owner,omitempty"`
	Categories              []string             `config:"categories,omitempty" json:"categories,omitempty" yaml:"categories,omitempty"`
	SignaturePath           string               `config:"signature_path,omitempty" json:"signature_path,omitempty" yaml:"signature_path,omitempty"`
	Discovery               *Discovery           `config:"discovery,omitempty" json:"discovery,omitempty,omitzero" yaml:"discovery,omitempty"`
	BaseDataStreams         []*BaseDataStream    `config:"data_streams,omitempty" json:"data_streams,omitempty" yaml:"data_streams,omitempty"`
}

// BasePolicyTemplate is used for the package policy templates in the /search endpoint
type BasePolicyTemplate struct {
	Name            string           `config:"name" json:"name" validate:"required"`
	Title           string           `config:"title" json:"title" validate:"required"`
	Description     string           `config:"description" json:"description" validate:"required"`
	Icons           []Image          `config:"icons,omitempty" json:"icons,omitempty" yaml:"icons,omitempty"`
	Categories      []string         `config:"categories,omitempty" json:"categories,omitempty" yaml:"categories,omitempty"`
	DeploymentModes *DeploymentModes `config:"deployment_modes,omitempty" json:"deployment_modes,omitempty" yaml:"deployment_modes,omitempty"`
	DataStreams     []string         `config:"data_streams,omitempty" json:"data_streams,omitempty" yaml:"data_streams,omitempty"`
}

type PolicyTemplate struct {
	Name            string           `config:"name" json:"name" validate:"required"`
	Title           string           `config:"title" json:"title" validate:"required"`
	Description     string           `config:"description" json:"description" validate:"required"`
	DataStreams     []string         `config:"data_streams,omitempty" json:"data_streams,omitempty" yaml:"data_streams,omitempty"`
	Inputs          []Input          `config:"inputs" json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Multiple        *bool            `config:"multiple" json:"multiple,omitempty" yaml:"multiple,omitempty"`
	Icons           []Image          `config:"icons,omitempty" json:"icons,omitempty" yaml:"icons,omitempty"`
	Categories      []string         `config:"categories,omitempty" json:"categories,omitempty" yaml:"categories,omitempty"`
	Screenshots     []Image          `config:"screenshots,omitempty" json:"screenshots,omitempty" yaml:"screenshots,omitempty"`
	Readme          *string          `config:"readme,omitempty" json:"readme,omitempty" yaml:"readme,omitempty"`
	DeploymentModes *DeploymentModes `config:"deployment_modes,omitempty" json:"deployment_modes,omitempty" yaml:"deployment_modes,omitempty"`

	// For purposes of "input packages"
	Type            string `config:"type,omitempty" json:"type,omitempty" yaml:"type,omitempty"`
	Input           string `config:"input,omitempty" json:"input,omitempty" yaml:"input,omitempty"`
	IngestionMethod string `config:"ingestion_method,omitempty" json:"ingestion_method,omitempty" yaml:"ingestion_method,omitempty"`
	TemplatePath    string `config:"template_path,omitempty" json:"template_path,omitempty" yaml:"template_path,omitempty"`
}

// Source contains metadata about the source of the package and its distribution.
type Source struct {
	License string `config:"license,omitempty" json:"license,omitempty" yaml:"license,omitempty"`
}

type Conditions struct {
	Kibana  *KibanaConditions  `config:"kibana,omitempty" json:"kibana,omitempty" yaml:"kibana,omitempty"`
	Elastic *ElasticConditions `config:"elastic,omitempty" json:"elastic,omitempty" yaml"elastic,omitempty"`
}

// KibanaConditions defines conditions for Kibana (e.g. required version).
type KibanaConditions struct {
	Version    string `config:"version" json:"version" yaml:"version"`
	constraint *semver.Constraints
}

// ElasticConditions defines conditions related to Elastic subscriptions or partnerships.
type ElasticConditions struct {
	Subscription string   `config:"subscription" json:"subscription" yaml:"subscription"`
	Capabilities []string `config:"capabilities,omitempty" json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
}

// Deprecated: Version is not currently used and will be removed in a future release.
type Version struct {
	Min string `config:"min,omitempty" json:"min,omitempty"`
	Max string `config:"max,omitempty" json:"max,omitempty"`
}

type Owner struct {
	Type   string `config:"type,omitempty" json:"type,omitempty"`
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

type PackageAgent struct {
	Privileges *PackageAgentPrivileges `config:"privileges,omitempty" json:"privileges,omitempty" yaml:"privileges,omitempty"`
}

type PackageAgentPrivileges struct {
	Root bool `config:"root,omitempty" json:"root,omitempty" yaml:"root,omitempty"`
}

type PackageElasticsearch struct {
	Privileges *PackageElasticsearchPrivileges `config:"privileges,omitempty" json:"privileges,omitempty" yaml:"privileges,omitempty"`
}

// Discovery define indications for the data this package can be useful with.
type Discovery struct {
	Fields   []DiscoveryField   `config:"fields,omitempty" json:"fields,omitempty" yaml:"fields,omitempty"`
	Datasets []DiscoveryDataset `config:"datasets,omitempty" json:"datasets,omitempty" yaml:"datasets,omitempty"`
}

func (d *Discovery) IsZero() bool {
	if d == nil {
		return true
	}
	return len(d.Fields) == 0 && len(d.Datasets) == 0
}

// DiscoveryField defines a field used for discovery.
type DiscoveryField struct {
	Name string `config:"name" json:"name" yaml:"name" validate:"required"`
}

// DiscoveryDataset defines a dataset used for discovery.
type DiscoveryDataset struct {
	Name string `config:"name" json:"name" yaml:"name" validate:"required"`
}

type PackageElasticsearchPrivileges struct {
	Cluster []string `config:"cluster,omitempty" json:"cluster,omitempty" yaml:"cluster,omitempty"`
}

func (i Image) getPath(p *Package) string {
	return path.Join(packagePathPrefix, p.Name, p.Version, i.Src)
}

// Deprecated: Download is not currently used and will be removed in a future release.
type Download struct {
	Path string `config:"path" json:"path" validate:"required"`
	Type string `config:"type" json:"type" validate:"required"`
}

type DeploymentModes struct {
	Default   *DeploymentMode `config:"default,omitempty" json:"default,omitempty" yaml:"default,omitempty"`
	Agentless *DeploymentMode `config:"agentless,omitempty" json:"agentless,omitempty" yaml:"agentless,omitempty"`
}

type DeploymentMode struct {
	Enabled   bool  `config:"enabled" json:"enabled" yaml:"enabled" validate:"required"`
	IsDefault *bool `config:"is_default" json:"is_default,omitempty" yaml:"is_default,omitempty"`
}

// Deprecated: NewCommand is not currently used and will be removed in a future release.
func NewDownload(p Package, t string) Download {
	return Download{
		Path: getDownloadPath(p, t),
		Type: t,
	}
}

// Deprecated: getDownloadPath is not currently used and will be removed in a future release.
func getDownloadPath(p Package, t string) string {
	return path.Join("/epr", p.Name, p.Name+"-"+p.Version+".zip")
}

// MustParsePackage creates a new package instances based on the given base path.
// The path passed goes to the root of the package where the manifest.yml is.
// It runs more strict validation than NewPackage, e.g. ensuring that all categories are valid.
func MustParsePackage(basePath string, fsBuilder FileSystemBuilder) (*Package, error) {
	p, err := newPackage(basePath, fsBuilder)
	if err != nil {
		return nil, fmt.Errorf("failed to create package from path %s: %w", basePath, err)
	}
	for _, c := range p.Categories {
		if _, ok := Categories[c]; !ok {
			return nil, fmt.Errorf("invalid category in package %s-%s: %s", p.Name, p.Version, c)
		}
	}
	return p, nil
}

// NewPackage creates a new package instances based on the given base path.
// The path passed goes to the root of the package where the manifest.yml is.
func NewPackage(logger *zap.Logger, basePath string, fsBuilder FileSystemBuilder) (*Package, error) {
	p, err := newPackage(basePath, fsBuilder)
	if err != nil {
		return nil, fmt.Errorf("failed to create package from path %s: %w", basePath, err)
	}
	j := 0
	for i, c := range p.Categories {
		if _, ok := Categories[c]; !ok {
			logger.Warn("package uses an unknown category, will be ignored",
				zap.String("package", p.Name),
				zap.String("version", p.Version),
				zap.String("category", c))
			continue
		}
		p.Categories[j] = p.Categories[i]
		j += 1
	}
	p.Categories = p.Categories[:j]
	return p, nil
}

func newPackage(basePath string, fsBuilder FileSystemBuilder) (*Package, error) {
	p := &Package{
		BasePath:  basePath,
		fsBuilder: fsBuilder,
	}
	fs, err := p.fs()
	if err != nil {
		return nil, err
	}
	defer fs.Close()

	manifestBody, err := ReadAll(fs, "manifest.yml")
	if err != nil {
		return nil, err
	}

	manifest, err := yaml.NewConfig(manifestBody, ucfg.PathSep("."))
	if err != nil {
		return nil, err
	}
	err = manifest.Unpack(p, ucfg.PathSep("."))
	if err != nil {
		return nil, err
	}

	// Default for the multiple flags is true.
	trueValue := true
	for i := range p.PolicyTemplates {
		if p.PolicyTemplates[i].Multiple == nil {
			p.PolicyTemplates[i].Multiple = &trueValue
		}

		// Collect basic information from policy templates and store into the /search endpoint
		t := p.PolicyTemplates[i]

		for k, i := range p.PolicyTemplates[i].Icons {
			t.Icons[k].Path = i.getPath(p)
		}

		// Store paths for all screenshots under each policy template
		if p.PolicyTemplates[i].Screenshots != nil {
			for k, s := range p.PolicyTemplates[i].Screenshots {
				p.PolicyTemplates[i].Screenshots[k].Path = s.getPath(p)
			}
		}

		// Store policy template specific README
		readmePath := path.Join("docs", p.PolicyTemplates[i].Name+".md")
		readme, err := fs.Stat(readmePath)
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

		// Fill ingestion method for input packages.
		if p.Type == "input" && p.PolicyTemplates[i].Input != "" {
			p.PolicyTemplates[i].IngestionMethod = IngestionMethods.Get(p.PolicyTemplates[i].Input)
		}
	}

	p.setBasePolicyTemplates()

	if p.Type == "" {
		p.Type = defaultType
	}

	// If not license is set, basic is assumed
	if p.License == "" {
		// Keep compatibility with deprecated license field.
		if p.Conditions != nil && p.Conditions.Elastic != nil && p.Conditions.Elastic.Subscription != "" {
			p.License = p.Conditions.Elastic.Subscription
		} else {
			p.License = DefaultLicense
		}
	}

	err = p.setRuntimeFields()
	if err != nil {
		return nil, err
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

	if p.Release == "" {
		p.Release = releaseForSemVerCompat(p.versionSemVer)
	}

	if !IsValidRelease(p.Release) {
		return nil, fmt.Errorf("invalid release: %q", p.Release)
	}

	readmePath := path.Join("docs", "README.md")
	// Check if readme
	readme, err := fs.Stat(readmePath)
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
		return nil, fmt.Errorf("loading package assets failed (path '%s'): %w", p.BasePath, err)
	}

	err = p.LoadDataSets()
	if err != nil {
		return nil, fmt.Errorf("loading package data streams failed (path '%s'): %w", p.BasePath, err)
	}

	p.setBaseDataStreams()

	// Read path for package signature
	p.SignaturePath, err = p.getSignaturePath()
	if err != nil {
		return nil, fmt.Errorf("can't process the package signature: %w", err)
	}
	return p, nil
}

func (p *Package) setRuntimeFields() error {
	var err error

	p.versionSemVer, err = semver.StrictNewVersion(p.Version)
	if err != nil {
		return fmt.Errorf("invalid package version: %w", err)
	}

	if p.Conditions != nil && p.Conditions.Kibana != nil {
		p.Conditions.Kibana.constraint, err = semver.NewConstraint(p.Conditions.Kibana.Version)
		if err != nil {
			return fmt.Errorf("invalid Kibana versions range %s: %w", p.Conditions.Kibana.Version, err)
		}
	}

	// Packages from proxy mode do not have "format_version" field
	if p.FormatVersion == "" {
		return nil
	}

	specSemVer, err := semver.StrictNewVersion(p.FormatVersion)
	if err != nil {
		return fmt.Errorf("invalid format spec version '%s': %w", p.FormatVersion, err)
	}

	specMajorMinorVersion := fmt.Sprintf("%d.%d.0", specSemVer.Major(), specSemVer.Minor())

	p.specMajorMinorSemVer, err = semver.StrictNewVersion(specMajorMinorVersion)
	if err != nil {
		return fmt.Errorf("invalid format spec version '%s': %w", specMajorMinorVersion, err)
	}

	return nil
}

// setBasePolicyTemplates method mirrors policy_templates from Package to a corresponding property in BasePackage.
// It's required to perform that sync, because PolicyTemplates and BasePolicyTemplates have same JSON annotation
// (policy_template).
func (p *Package) setBasePolicyTemplates() {
	for _, t := range p.PolicyTemplates {
		baseT := BasePolicyTemplate{
			Name:            t.Name,
			Title:           t.Title,
			Description:     t.Description,
			Categories:      t.Categories,
			Icons:           t.Icons,
			DeploymentModes: t.DeploymentModes,
			DataStreams:     t.DataStreams,
		}

		p.BasePolicyTemplates = append(p.BasePolicyTemplates, baseT)
	}
}

// setBaseDataStreams method mirrors data_streams from Package to a corresponding property in BasePackage.
// It's required to perform that sync, because DataStreams and BaseDataStreams have same JSON annotation
// (data_streams).
func (p *Package) setBaseDataStreams() {
	for _, ds := range p.DataStreams {
		baseStream := &BaseDataStream{
			Type:    ds.Type,
			Dataset: ds.Dataset,
			Title:   ds.Title,
		}
		p.BaseDataStreams = append(p.BaseDataStreams, baseStream)
	}
}

func (p *Package) HasCategory(category string) bool {
	return hasCategory(p.Categories, category)
}

func (p *Package) HasPolicyTemplateWithCategory(category string) bool {
	for _, pt := range p.PolicyTemplates {
		if hasCategory(pt.Categories, category) {
			return true
		}
	}
	return false
}

func hasCategory(categories []string, category string) bool {
	if slices.Contains(categories, category) {
		return true
	}

	// Check if this category has subcategories, and the package contains any of them.
	for _, subcategory := range Categories {
		if subcategory.Parent == nil || subcategory.Parent.Name != category {
			continue
		}

		if slices.Contains(categories, subcategory.Name) {
			return true
		}
	}
	return false
}

func (p *Package) HasKibanaVersion(version *semver.Version) bool {
	// If the version is not specified, it is for all versions
	if p.Conditions == nil || p.Conditions.Kibana == nil || p.Conditions.Kibana.constraint == nil || version == nil {
		return true
	}

	return p.Conditions.Kibana.constraint.Check(version)
}

func (p *Package) WorksWithCapabilities(capabilities []string) bool {
	if p.Conditions == nil || p.Conditions.Elastic == nil || p.Conditions.Elastic.Capabilities == nil || capabilities == nil {
		return true
	}

	for _, requiredCapability := range p.Conditions.Elastic.Capabilities {
		if !slices.Contains(capabilities, requiredCapability) {
			return false
		}
	}
	return true
}

func (p *Package) HasCompatibleSpec(specMin, specMax, kibanaVersion *semver.Version) (bool, error) {
	// FIXME: kibanaVersion parameter is not used, it should be removed.
	if specMin == nil && specMax == nil {
		return true, nil
	}

	constraints := []string{}
	if specMin != nil {
		constraints = append(constraints, fmt.Sprintf(">=%s", specMin.String()))
	}
	if specMax != nil {
		constraints = append(constraints, fmt.Sprintf("<=%s", specMax.String()))
	}

	fullConstraint := strings.Join(constraints, ",")
	constraint, err := semver.NewConstraint(fullConstraint)
	if err != nil {
		return false, fmt.Errorf("cannot create constraint %s: %w", fullConstraint, err)
	}

	if p.specMajorMinorSemVer == nil {
		return false, errors.New("package spec version is not set")
	}

	return constraint.Check(p.specMajorMinorSemVer), nil
}

func (p *Package) IsNewerOrEqual(pp *Package) bool {
	return !p.versionSemVer.LessThan(pp.versionSemVer)
}

func (p *Package) IsPrerelease() bool {
	return isPrerelease(p.versionSemVer)
}

func isPrerelease(version *semver.Version) bool {
	if version.Major() < 1 {
		return true
	}
	return version.Prerelease() != ""
}

// LoadAssets (re)loads all the assets of the package
// Based on the time when this is called, it might be that not all assets for a package exist yet, so it is reset every time.
func (p *Package) LoadAssets() (err error) {
	fs, err := p.fs()
	if err != nil {
		return err
	}
	defer fs.Close()

	// Reset Assets
	p.Assets = nil

	// Iterates recursively through all the levels to find assets
	// If we need more complex matching a library like https://github.com/bmatcuk/doublestar
	// could be used but the below works and is pretty simple.
	assets, err := collectAssets(fs, "*")
	if err != nil {
		return err
	}
	for _, a := range assets {
		// Unfortunately these files keep sneaking in
		if strings.Contains(a, ".DS_Store") {
			continue
		}

		info, err := fs.Stat(a)
		if err != nil {
			return err
		}

		if info.IsDir() {
			if strings.Contains(info.Name(), "-") {
				return fmt.Errorf("directory name inside package %s contains -: %s", p.Name, a)
			}
			continue
		}

		a = path.Join(packagePathPrefix, p.GetPath(), a)
		p.Assets = append(p.Assets, a)
	}
	return nil
}

func collectAssets(fs PackageFileSystem, pattern string) ([]string, error) {
	assets, err := fs.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(assets) != 0 {
		a, err := collectAssets(fs, path.Join(pattern, "*"))
		if err != nil {
			return nil, err
		}
		return append(assets, a...), nil
	}
	return nil, nil
}

func (p *Package) fs() (PackageFileSystem, error) {
	if p.fsBuilder == nil {
		return NewVirtualPackageFileSystem()
	}

	return p.fsBuilder(p)
}

// Validate is called during Unpack of the manifest.
// The validation here is only related to the fields directly specified in the manifest itself.
func (p *Package) Validate() error {
	if ValidationDisabled {
		return nil
	}

	if p.FormatVersion == "" {
		return fmt.Errorf("no format_version set: %v", p)
	}

	_, err := semver.StrictNewVersion(p.FormatVersion)
	if err != nil {
		return fmt.Errorf("invalid package version: %s, %s", p.FormatVersion, err)
	}

	p.versionSemVer, err = semver.StrictNewVersion(p.Version)
	if err != nil {
		return err
	}

	if p.Release == "" {
		p.Release = releaseForSemVerCompat(p.versionSemVer)
	}

	if p.Title == nil || *p.Title == "" {
		return fmt.Errorf("no title set for package: %s", p.Name)
	}

	if p.Description == "" {
		return fmt.Errorf("no description set")
	}

	fs, err := p.fs()
	if err != nil {
		return err
	}
	defer fs.Close()

	for _, i := range p.Icons {
		_, err := fs.Stat(i.Src)
		if err != nil {
			return err
		}
	}

	for _, s := range p.Screenshots {
		_, err := fs.Stat(s.Src)
		if err != nil {
			return err
		}
	}

	err = p.validateVersionConsistency()
	if err != nil {
		return fmt.Errorf("version in manifest file is not consistent with path: %w", err)
	}

	return p.ValidateDataStreams()
}

func (p *Package) validateVersionConsistency() error {
	versionPackage, err := semver.NewVersion(p.Version)
	if err != nil {
		return fmt.Errorf("invalid version defined in manifest: %w", err)
	}

	baseDir := path.Base(p.BasePath)
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
	fs, err := p.fs()
	if err != nil {
		return nil, err
	}
	defer fs.Close()

	dataStreamBasePath := "data_stream"

	// Look for a file here that a data_stream must have, some file systems as Zip files
	// may not have entries for directories.
	paths, err := fs.Glob(path.Join(dataStreamBasePath, "*", "manifest.yml"))
	if err != nil {
		return nil, err
	}

	for i := range paths {
		if !strings.HasPrefix(paths[i], dataStreamBasePath) && !strings.HasPrefix(paths[i], "/data_stream") {
			return nil, fmt.Errorf("failed to get data stream path inside package: cannot make %q relative to %q", paths[i], dataStreamBasePath)
		}
		relPath := strings.TrimPrefix(paths[i], dataStreamBasePath)
		paths[i] = path.Dir(relPath)
	}

	return paths, nil
}

func (p *Package) LoadDataSets() error {
	dataStreamPaths, err := p.GetDataStreamPaths()
	if err != nil {
		return err
	}

	dataStreamsBasePath := "data_stream"
	for _, dataStreamPath := range dataStreamPaths {
		dataStreamBasePath := path.Join(dataStreamsBasePath, dataStreamPath)

		d, err := NewDataStream(dataStreamBasePath, p)
		if err != nil {
			return err
		}

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

	dataStreamsBasePath := "data_stream"
	for _, dataStreamPath := range dataStreamPaths {
		dataStreamBasePath := path.Join(dataStreamsBasePath, dataStreamPath)

		d, err := NewDataStream(dataStreamBasePath, p)
		if err != nil {
			return fmt.Errorf("building data stream failed (path: %s): %w", dataStreamBasePath, err)
		}

		err = d.Validate()
		if err != nil {
			return fmt.Errorf("validating data stream failed (path: %s): %w", dataStreamBasePath, err)
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

func (p *Package) getSignaturePath() (string, error) {
	_, err := os.Stat(p.BasePath + ".sig")
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("can't stat signature file: %w", err)
	}
	return p.GetDownloadPath() + ".sig", nil
}

func (p *Package) SetRemoteResolver(r RemoteResolver) {
	p.resolver = r
}

func (p *Package) RemoteResolver() RemoteResolver {
	return p.resolver
}

func categoryTitles(categories categories.Categories) map[string]string {
	titles := make(map[string]string)
	for _, category := range categories {
		titles[category.Name] = category.Title
	}
	return titles
}
