// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package proxymode

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/storage"
	"github.com/elastic/package-registry/util"
)

type ProxyMode struct {
	options ProxyOptions

	httpClient *http.Client
	destinationURL *url.URL
	resolver *proxyResolver
}

type ProxyOptions struct {
	Enabled bool
	ProxyTo string
}

func NoProxy() *ProxyMode {
	proxyMode, err := NewProxyMode(ProxyOptions{})
	if err != nil {
		log.Fatalf("no proxy mode should not return an error: %v", err)
	}
	return proxyMode
}

func NewProxyMode(options ProxyOptions) (*ProxyMode, error) {
	var pm ProxyMode
	pm.options = options

	if !options.Enabled {
		return &pm, nil
	}

	pm.httpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	var err error
	pm.destinationURL, err = url.Parse(pm.options.ProxyTo)
	if err != nil {
		return nil, errors.Wrap(err, "can't create proxy destination URL")
	}

	pm.resolver = &proxyResolver{
		artifactsPackagesURL: *pm.destinationURL.ResolveReference(&url.URL{Path: storage.ArtifactsPackagesStoragePath + "/"}),
		artifactsStaticURL:   *pm.destinationURL.ResolveReference(&url.URL{Path: storage.ArtifactsStaticStoragePath + "/"}),
	}
	return &pm, nil
}

func (pm *ProxyMode) Enabled() bool {
	return pm.options.Enabled
}

func (pm *ProxyMode) Search(r *http.Request) (packages.Packages, error) {
	if !pm.options.Enabled {
		return packages.Packages{}, nil
	}
	logger := util.Logger()

	proxyURL := *r.URL
	proxyURL.Host = pm.destinationURL.Host
	proxyURL.Scheme = pm.destinationURL.Scheme
	proxyURL.User = pm.destinationURL.User

	proxyRequest, err := http.NewRequest("GET", proxyURL.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "can't create proxy request")
	}

	logger.Debug("Proxy /search request", zap.String("request.uri", proxyURL.String()))
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
	for i := 0; i < len(pkgs); i++ {
		pkgs[i].SetRemoteResolver(pm.resolver)
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
	logger := util.Logger()

	vars := mux.Vars(r)
	packageName, ok := vars["packageName"]
	if !ok {
		return nil, errors.New("missing package name")
	}

	packageVersion, ok := vars["packageVersion"]
	if !ok {
		return nil, errors.New("missing package version")
	}

	urlPath := fmt.Sprintf("/package/%s/%s/", packageName, packageVersion)
	proxyURL := pm.destinationURL.ResolveReference(&url.URL{Path: urlPath})
	proxyRequest, err := http.NewRequest("GET", proxyURL.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "can't create proxy request")
	}

	logger.Debug("Proxy /package request", zap.String("request.uri", proxyURL.String()))
	response, err := pm.httpClient.Do(proxyRequest)
	if err != nil {
		return nil, errors.Wrap(err, "can't proxy search request")
	}
	defer response.Body.Close()
	var pkg packages.Package
	err = json.NewDecoder(response.Body).Decode(&pkg)
	if err != nil {
		return nil, errors.Wrap(err, "can't proxy search request")
	}
	pkg.SetRemoteResolver(pm.resolver)
	return &pkg, nil
}
