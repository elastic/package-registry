// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/google/go-querystring/query"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

const defaultAddress = "https://epr.elastic.co"

func main() {
	if len(os.Args) != 2 {
		usageAndExit(-1)
	}
	config, err := readConfig(os.Args[1])
	if err != nil {
		fmt.Printf("failed to read configuration from %s: %s\n", os.Args[1], err)
		os.Exit(-1)
	}
	for _, action := range config.Actions {
		err := action.init(config)
		if err != nil {
			fmt.Printf("failed to initialize actions: %s", err)
			os.Exit(-1)
		}
	}

	packages, err := config.collect(&http.Client{})
	if err != nil {
		fmt.Printf("failed to collect packages: %s", err)
		os.Exit(-1)
	}

	taskpool := newTaskPool(runtime.GOMAXPROCS(0))
	for _, info := range packages {
		taskpool.Do(func() error {
			for _, action := range config.Actions {
				err := action.perform(info)
				if err != nil {
					return fmt.Errorf("failed to collect packages: %w", err)
				}
			}
			return nil
		})
	}
	if err := taskpool.Wait(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	fmt.Println(len(packages), "packages total")
}

func usageAndExit(status int) {
	fmt.Println(os.Args[0], "[config.yaml]")
	os.Exit(status)
}

type config struct {
	Address string `yaml:"address"`
	Matrix  []configQuery
	Queries []configQuery `yaml:"queries"`
	Actions configActions `yaml:"actions"`
}

func (c config) urls() (iter.Seq[*url.URL], error) {
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
		panic("invalid url")
	}
	matrix := c.Matrix
	if len(matrix) == 0 {
		matrix = []configQuery{{}}
	}
	return func(yield func(*url.URL) bool) {
		for _, m := range matrix {
			for _, q := range c.Queries {
				values := m.Build()
				for k, v := range q.Build() {
					values[k] = v
				}
				ref := ""
				encoded := values.Encode()
				if len(encoded) > 0 {
					ref = "?" + encoded
				}
				url, err := baseURL.Parse(ref)
				if err != nil {
					panic("invalid query " + encoded)
				}
				if !yield(url) {
					return
				}
			}
		}
	}, nil
}

func (c config) collect(client *http.Client) ([]packageInfo, error) {
	urls, err := c.urls()
	if err != nil {
		return nil, fmt.Errorf("failed to build URLs: %w", err)
	}

	type key struct {
		Name, Version string
	}
	packagesMap := make(map[key]packageInfo)
	taskPool := newTaskPool(runtime.GOMAXPROCS(0))
	for u := range urls {
		taskPool.Do(func() error {
			resp, err := client.Get(u.String())
			if err != nil {
				return fmt.Errorf("failed to GET %s: %w", u, err)
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return fmt.Errorf("failed to GET %s (status code %d)", u, resp.StatusCode)
			}

			var packages []packageInfo
			err = json.NewDecoder(resp.Body).Decode(&packages)
			if err != nil {
				resp.Body.Close()
				return fmt.Errorf("failed to parse search response: %w", err)
			}
			resp.Body.Close()
			fmt.Println(u.String(), len(packages), "packages")

			for _, p := range packages {
				k := key{Name: p.Name, Version: p.Version}
				if _, found := packagesMap[k]; found {
					continue
				}
				packagesMap[k] = p
			}

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

	slices.SortFunc(result, func(a, b packageInfo) int {
		if n := strings.Compare(a.Name, b.Name); n != 0 {
			return n
		}

		return semver.Compare(a.Version, b.Version)
	})

	return result, nil
}

type configQuery struct {
	Package       string `yaml:"package" url:"package,omitempty"`
	All           bool   `yaml:"all" url:"all,omitempty"`
	Prerelease    bool   `yaml:"prerelease" url:"prerelease,omitempty"`
	Type          string `yaml:"type" url:"type,omitempty"`
	KibanaVersion string `yaml:"kibana.version" url:"kibana.version,omitempty"`
	SpecMin       string `yaml:"spec.min" url:"spec.min,omitempty"`
	SpecMax       string `yaml:"spec.max" url:"spec.max,omitempty"`
}

func (q configQuery) Build() url.Values {
	v, err := query.Values(q)
	if err != nil {
		panic(err)
	}
	return v
}

type packageInfo struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Download      string `json:"download"`
	SignaturePath string `json:"signature_path"`
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

type printAction struct{}

func (a *printAction) init(c config) error {
	return nil
}

func (a *printAction) perform(i packageInfo) error {
	fmt.Println("- ", i.Name, i.Version)
	return nil
}

type downloadAction struct {
	client *http.Client

	Address     string `yaml:"address"`
	Destination string `yaml:"destination"`
}

func (a *downloadAction) init(c config) error {
	a.client = &http.Client{}
	if a.Address == "" {
		a.Address = c.Address
	}
	return os.MkdirAll(a.Destination, 0755)
}

func (a *downloadAction) perform(i packageInfo) error {
	if err := a.download(i.Download); err != nil {
		return fmt.Errorf("failed to download package %s: %w", i.Download, err)
	}
	if err := a.download(i.SignaturePath); err != nil {
		return fmt.Errorf("failed to download signature %s: %w", i.SignaturePath, err)
	}
	return nil
}

func (a *downloadAction) download(urlPath string) error {
	p, err := url.JoinPath(a.Address, urlPath)
	if err != nil {
		return fmt.Errorf("failed to build url: %w", err)
	}
	resp, err := a.client.Get(p)
	if err != nil {
		return fmt.Errorf("failed to get %s: %w", urlPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get %s (status code %d)", urlPath, resp.StatusCode)
	}

	f, err := os.Create(filepath.Join(a.Destination, path.Base(urlPath)))
	if err != nil {
		return fmt.Errorf("failed to open %s in %s: %w", path.Base(urlPath), a.Destination, err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
