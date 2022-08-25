// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package proxymode

import (
	"encoding/json"
	"go.uber.org/zap"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/util"
)

type ProxyMode struct {
	options ProxyOptions

	httpClient *http.Client
}

type ProxyOptions struct {
	Enabled bool
	ProxyTo string
}

func NoProxy() *ProxyMode {
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

func (pm *ProxyMode) Enabled() bool {
	return pm.options.Enabled
}

func (pm *ProxyMode) Search(r *http.Request) (packages.Packages, error) {
	if !pm.options.Enabled {
		return packages.Packages{}, nil
	}
	logger := util.Logger()

	destinationURL, err := url.Parse(pm.options.ProxyTo)
	if err != nil {
		return nil, errors.Wrap(err, "can't create proxy destination url")
	}

	proxyURL := *r.URL
	proxyURL.Host = destinationURL.Host
	proxyURL.Scheme = destinationURL.Scheme
	proxyURL.User = destinationURL.User

	proxyRequest, err := http.NewRequest("GET", proxyURL.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "can't create proxy request")
	}

	logger.Debug("Proxy search request", zap.String("request.uri", proxyURL.String()))
	response, err := pm.httpClient.Do(proxyRequest)
	if err != nil {
		return nil, errors.Wrap(err, "can't proxy search request")
	}
	defer response.Body.Close()
	var pkgs packages.Packages
	err = json.NewDecoder(response.Body).Decode(&pkgs)
	if err != nil {
		return nil, errors.Wrap(err, "can't proxy search request")
	}
	return pkgs, nil
}

func (pm *ProxyMode) Categories(r *http.Request) (map[string]*packages.Category, error) {
	if !pm.options.Enabled {
		return map[string]*packages.Category{}, nil
	}

	panic("categories: not implemented yet")
}

func (pm *ProxyMode) Package(r *http.Request) (*packages.Package, error) {
	if !pm.options.Enabled {
		return nil, nil
	}

	panic("package: not implemented yet")
}
