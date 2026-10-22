// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/elastic/package-registry/internal/util"
)

// PROTOTYPE: integration groups. A group is one tile per technology; each
// schema maps to the package a user installs for it. The taxonomy lives in a
// file in the integrations repo (groups.yml) and is served as /groups.

type groupSchema struct {
	Package string `yaml:"package" json:"package"`
	Default bool   `yaml:"default,omitempty" json:"default,omitempty"`
}

type group struct {
	Name        string                 `yaml:"-" json:"name"`
	Title       string                 `yaml:"title" json:"title"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Icon        string                 `yaml:"icon,omitempty" json:"icon,omitempty"`
	Schemas     map[string]groupSchema `yaml:"schemas" json:"schemas"`
}

type groupsFile struct {
	Groups map[string]group `yaml:"groups"`
}

// loadGroups reads the groups taxonomy file. An empty path means no groups.
func loadGroups(path string) ([]group, error) {
	groups := []group{}
	if path == "" {
		return groups, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading groups file %s: %w", path, err)
	}
	var f groupsFile
	if err := yaml.Unmarshal(content, &f); err != nil {
		return nil, fmt.Errorf("parsing groups file %s: %w", path, err)
	}
	for name, g := range f.Groups {
		g.Name = name
		if g.Schemas == nil {
			g.Schemas = map[string]groupSchema{}
		}
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, nil
}

type groupsHandler struct {
	cacheTime time.Duration
	body      []byte
}

// newGroupsHandler reads the groups file once, at startup. Groups are always
// served locally, also in proxy mode.
func newGroupsHandler(path string, cacheTime time.Duration) (*groupsHandler, error) {
	groups, err := loadGroups(path)
	if err != nil {
		return nil, err
	}
	body, err := util.MarshalJSONPretty(groups)
	if err != nil {
		return nil, fmt.Errorf("marshalling groups: %w", err)
	}
	return &groupsHandler{cacheTime: cacheTime, body: body}, nil
}

func (h *groupsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serveJSONResponse(r.Context(), w, h.cacheTime, h.body)
}
