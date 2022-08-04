// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package proxymode

import (
	"net/http"
	"time"

	"github.com/elastic/package-registry/packages"
)

type ProxyMode struct {
	options ProxyOptions

	httpClient *http.Client
}

type ProxyOptions struct {
	Enabled bool
	ProxyTo string
}

func Disabled() *ProxyMode {
	return NewProxyMode(ProxyOptions{})
}

func NewProxyMode(options ProxyOptions) *ProxyMode {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	return &ProxyMode{
		options:    options,
		httpClient: httpClient,
	}
}

func (pm *ProxyMode) Search(r http.Request) ([]packages.BasePackage, error) {
	panic("search: not implemented yet")
}

func (pm *ProxyMode) Categories(r http.Request) (map[string]*packages.Category, error) {
	panic("categories: not implemented yet")
}

func (pm *ProxyMode) Package(r http.Request) (*packages.Package, error) {
	panic("package: not implemented yet")
}
