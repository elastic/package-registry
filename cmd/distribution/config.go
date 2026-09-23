// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-querystring/query"
	"gopkg.in/yaml.v3"

	"github.com/elastic/package-registry/cmd/distribution/internal/workers"
)

type config struct {
	Address string `yaml:"address"`
	// Keep is the default newest-N window per package per search response.
	// Zero keeps everything. Values > 1 force all=true on the wire.
	// Individual matrix entries and queries can override this.
	Keep     int             `yaml:"keep"`
	Matrix   []configQuery   `yaml:"matrix"`
	Queries  []configQuery   `yaml:"queries"`
	Packages []configPackage `yaml:"packages"`
	Actions  configActions   `yaml:"actions"`
}

// keepFor returns the number of versions to keep for a search. A query
// overrides its matrix entry, which overrides the document default. Zero, and
// any negative value, keep every version.
func (c config) keepFor(m, q configQuery) int {
	keep := c.Keep
	if m.Keep != nil {
		keep = *m.Keep
	}
	if q.Keep != nil {
		keep = *q.Keep
	}
	return max(keep, 0)
}

// searchURLs generates the search URLs required for the given configuration,
// each with the number of newest versions to keep from its response. Duplicate
// URLs — produced when a query key fully overrides its matrix key — are
// collapsed into a single entry whose keep window is merged by mergeKeep.
func (c config) searchURLs() (iter.Seq2[*url.URL, int], error) {
	address := defaultAddress
	if c.Address != "" {
		address = c.Address
	}
	basePath, err := url.JoinPath(address, "search")
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}
	baseURL, err := url.Parse(basePath)
	if err != nil {
		// This should not happen because JoinPath already parses the url.
		fmt.Fprintf(os.Stderr, "invalid url %q: %s\n", basePath, err)
		os.Exit(-1)
	}
	matrix := c.Matrix
	if len(matrix) == 0 {
		matrix = []configQuery{{}}
	}

	type urlKeep struct {
		u    *url.URL
		keep int
	}
	var ordered []urlKeep
	seen := make(map[string]int) // URL string → index in ordered

	for _, m := range matrix {
		for _, q := range c.Queries {
			keep := c.keepFor(m, q)
			values := m.Build()
			for k, v := range q.Build() {
				values[k] = v
			}
			if keep > 1 {
				// The registry returns a single version per package unless all is set,
				// and it has no way to limit the result size.
				values.Set("all", "true")
			}
			ref := ""
			encoded := values.Encode()
			if len(encoded) > 0 {
				ref = "?" + encoded
			}
			u, err := baseURL.Parse(ref)
			if err != nil {
				panic("invalid query " + encoded)
			}
			key := u.String()
			if idx, dup := seen[key]; dup {
				ordered[idx].keep = mergeKeep(ordered[idx].keep, keep)
			} else {
				seen[key] = len(ordered)
				ordered = append(ordered, urlKeep{u: u, keep: keep})
			}
		}
	}

	return func(yield func(*url.URL, int) bool) {
		for _, uk := range ordered {
			if !yield(uk.u, uk.keep) {
				return
			}
		}
	}, nil
}

// mergeKeep combines the keep windows of two identical search URLs. Zero means
// unlimited, so it wins over any bounded window; otherwise the wider window
// wins because its result set is a superset of the narrower one.
func mergeKeep(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	return max(a, b)
}

// downloadPathForPackage returns the paths to download the package with the given name and version and its signature.
func (c config) downloadPathForPackage(name, version string) (string, string) {
	path := path.Join("epr", name, fmt.Sprintf("%s-%s.zip", name, version))
	return path, path + ".sig"
}

func (c config) collect(client *http.Client) ([]packageInfo, error) {
	urls, err := c.searchURLs()
	if err != nil {
		return nil, fmt.Errorf("failed to build URLs: %w", err)
	}

	type key struct {
		Name, Version string
	}
	var mapLock sync.Mutex
	packagesMap := make(map[key]packageInfo)

	pinnedPackages, err := c.pinnedPackages()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare pinned packages: %w", err)
	}
	for _, p := range pinnedPackages {
		packagesMap[key{Name: p.Name, Version: p.Version}] = p
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	taskPool := workers.NewTaskPool(searchConcurrency)
	for u, keep := range urls {
		if ctx.Err() != nil {
			break
		}
		taskPool.Do(func() error {
			if ctx.Err() != nil {
				return nil // another task already failed; do not touch the server
			}
			reqCtx, reqCancel := context.WithTimeout(ctx, searchTimeout)
			defer reqCancel()
			req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u.String(), nil)
			if err != nil {
				cancel()
				return fmt.Errorf("failed to build request for %s: %w", u, err)
			}
			resp, err := client.Do(req)
			if err != nil {
				if ctx.Err() != nil {
					return nil // context was cancelled by another failing task
				}
				cancel()
				return fmt.Errorf("failed to GET %s: %w", u, err)
			}
			defer drainAndClose(resp.Body)
			if resp.StatusCode != http.StatusOK {
				cancel()
				return fmt.Errorf("failed to GET %s (status code %d)", u, resp.StatusCode)
			}

			var packages []packageInfo
			err = json.NewDecoder(resp.Body).Decode(&packages)
			if err != nil {
				cancel()
				return fmt.Errorf("failed to parse search response: %w", err)
			}
			kept := truncateVersions(packages, keep)
			fmt.Fprintf(os.Stderr, "%s %d of %d packages\n", u.String(), len(kept), len(packages))

			mapLock.Lock()
			for _, p := range kept {
				k := key{Name: p.Name, Version: p.Version}
				if _, found := packagesMap[k]; found {
					continue
				}
				packagesMap[k] = p
			}
			mapLock.Unlock()

			return nil
		})
	}
	if err := taskPool.Wait(); err != nil {
		return nil, err
	}

	result := make([]packageInfo, 0, len(packagesMap))
	for _, p := range packagesMap {
		result = append(result, p)
	}

	slices.SortFunc(result, comparePackageInfo)

	return result, nil
}

// comparePackageInfo orders packages by name, and then by version.
func comparePackageInfo(a, b packageInfo) int {
	if n := strings.Compare(a.Name, b.Name); n != 0 {
		return n
	}
	return compareVersions(a.Version, b.Version)
}

// compareVersions orders two version strings.
//
// An invalid semantic version string is considered less than a valid one.
// All invalid semantic version strings compare equal to each other.
// From https://pkg.go.dev/golang.org/x/mod/semver#Compare
func compareVersions(a, b string) int {
	va, errA := semver.NewVersion(a)
	vb, errB := semver.NewVersion(b)
	switch {
	case errA != nil && errB != nil:
		return 0
	case errA != nil:
		return -1
	case errB != nil:
		return 1
	}
	return va.Compare(vb)
}

// truncateVersions keeps at most keep newest versions of each package in
// packages, reordering and compacting it in place. A keep of zero or less
// keeps everything. Versions that are not valid semantic versions sort oldest
// and are dropped first.
//
// The window is per search response on purpose: each matrix entry is a Kibana
// version that needs its own installable versions, so the limit is applied
// before responses are merged.
func truncateVersions(packages []packageInfo, keep int) []packageInfo {
	if keep <= 0 || len(packages) <= keep {
		return packages
	}

	// Stable so that versions comparing equal (invalid ones) are dropped in
	// the order the registry returned them, rather than arbitrarily.
	slices.SortStableFunc(packages, comparePackageInfo)

	n := 0
	for i := 0; i < len(packages); {
		j := i
		for j < len(packages) && packages[j].Name == packages[i].Name {
			j++
		}
		n += copy(packages[n:], packages[max(i, j-keep):j])
		i = j
	}
	return packages[:n]
}

func (c config) pinnedPackages() ([]packageInfo, error) {
	packages := make([]packageInfo, len(c.Packages))
	for i, p := range c.Packages {
		download, signature := c.downloadPathForPackage(p.Name, p.Version)
		packages[i] = packageInfo{
			Name:          p.Name,
			Version:       p.Version,
			Download:      download,
			SignaturePath: signature,
		}
	}
	return packages, nil
}

type packageInfo struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Download      string `json:"download"`
	SignaturePath string `json:"signature_path"`
}

func configActionFactory(name string) (configAction, error) {
	switch name {
	case "print":
		return &printAction{}, nil
	case "download":
		return &downloadAction{}, nil
	default:
		return nil, fmt.Errorf("unknown action %s", name)
	}
}

type configAction interface {
	init(config) error
	perform(packageInfo) error
}

type configQuery struct {
	Package       string `yaml:"package" url:"package,omitempty"`
	All           bool   `yaml:"all" url:"all,omitempty"`
	Prerelease    bool   `yaml:"prerelease" url:"prerelease,omitempty"`
	Type          string `yaml:"type" url:"type,omitempty"`
	KibanaVersion string `yaml:"kibana.version" url:"kibana.version,omitempty"`
	SpecMin       string `yaml:"spec.min" url:"spec.min,omitempty"`
	SpecMax       string `yaml:"spec.max" url:"spec.max,omitempty"`
	// Keep overrides the document default. It is not a registry parameter:
	// /search cannot limit results, so the window is applied in collect.
	Keep *int `yaml:"keep" url:"-"`
}

type configPackage struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

func (q configQuery) Build() url.Values {
	v, err := query.Values(q)
	if err != nil {
		panic(err)
	}
	return v
}

func readConfig(path string) (config, error) {
	var config config
	d, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}

	return config, yaml.Unmarshal(d, &config)
}

type configActions []configAction

var _ yaml.Unmarshaler = &configActions{}

func (actions *configActions) UnmarshalYAML(node *yaml.Node) error {
	var actionsMap []map[string]yaml.Node
	err := node.Decode(&actionsMap)
	if err != nil {
		return fmt.Errorf("failed to decode actions: %w", err)
	}

	*actions = make(configActions, 0, len(actionsMap))
	for _, configMap := range actionsMap {
		if len(configMap) != 1 {
			return errors.New("multiple entries found in action")
		}
		for name, config := range configMap {
			action, err := configActionFactory(name)
			if err != nil {
				return err
			}
			err = config.Decode(action)
			if err != nil {
				return fmt.Errorf("could not decode action %s: %w", name, err)
			}
			*actions = append(*actions, action)
		}
	}

	return nil
}
