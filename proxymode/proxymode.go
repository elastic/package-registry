// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package proxymode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/packages"
	"github.com/elastic/package-registry/util"
)

type ProxyMode struct {
	options ProxyOptions

	httpClient     *retryablehttp.Client
	destinationURL *url.URL
	resolver       *proxyResolver
}

type ProxyOptions struct {
	Enabled bool
	ProxyTo string
}

func NoProxy() *ProxyMode {
	proxyMode, err := NewProxyMode(ProxyOptions{Enabled: false})
	if err != nil {
		panic(errors.Wrapf(err, "unexpected error"))
	}
	return proxyMode
}

func NewProxyMode(options ProxyOptions) (*ProxyMode, error) {
	var pm ProxyMode
	pm.options = options

	if !options.Enabled {
		return &pm, nil
	}

	pm.httpClient = &retryablehttp.Client{
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		Logger:       withZapLoggerAdapter(util.Logger()),
		RetryWaitMin: 1 * time.Second,
		RetryWaitMax: 15 * time.Second,
		RetryMax:     4,
		CheckRetry:   proxyRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
	}

	var err error
	pm.destinationURL, err = url.Parse(pm.options.ProxyTo)
	if err != nil {
		return nil, errors.Wrap(err, "can't create proxy destination URL")
	}

	pm.resolver = &proxyResolver{destinationURL: *pm.destinationURL}
	return &pm, nil
}

// proxyRetryPolicy function extends the DefaultRetryPolicy to check if the HTTP response content-type
// is application/json. We found occurrences of requests being rejected by an intermittent proxy and causing
// the json.Decoder to fail.
func proxyRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	shouldRetry, err := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	if shouldRetry {
		return shouldRetry, err
	}

	// Chaining Package Registry servers (proxies) is allowed. HTTP client must get to the end of the chain.
	locationHeader := resp.Header.Get("location")
	if locationHeader != "" {
		return false, nil
	}

	contentType := resp.Header.Get("content-type")
	if strings.HasPrefix(contentType, "application/json") {
		return false, nil
	}
	return true, fmt.Errorf("unexpected content type: %s", contentType)
}

func (pm *ProxyMode) Enabled() bool {
	return pm.options.Enabled
}

func (pm *ProxyMode) Search(r *http.Request) (packages.Packages, error) {
	logger := util.Logger()

	proxyURL := *r.URL
	proxyURL.Host = pm.destinationURL.Host
	proxyURL.Scheme = pm.destinationURL.Scheme
	proxyURL.User = pm.destinationURL.User

	proxyRequest, err := retryablehttp.NewRequest(http.MethodGet, proxyURL.String(), nil)
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

func (pm *ProxyMode) Categories(r *http.Request) ([]packages.Category, error) {
	logger := util.Logger()

	proxyURL := *r.URL
	proxyURL.Host = pm.destinationURL.Host
	proxyURL.Scheme = pm.destinationURL.Scheme
	proxyURL.User = pm.destinationURL.User

	proxyRequest, err := retryablehttp.NewRequest(http.MethodGet, proxyURL.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "can't create proxy request")
	}

	logger.Debug("Proxy /categories request", zap.String("request.uri", proxyURL.String()))
	response, err := pm.httpClient.Do(proxyRequest)
	if err != nil {
		return nil, errors.Wrap(err, "can't proxy categories request")
	}
	defer response.Body.Close()
	var cats []packages.Category
	err = json.NewDecoder(response.Body).Decode(&cats)
	if err != nil {
		return nil, errors.Wrap(err, "can't proxy categories request")
	}
	return cats, nil
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
	proxyRequest, err := retryablehttp.NewRequest(http.MethodGet, proxyURL.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "can't create proxy request")
	}

	logger.Debug("Proxy /package request", zap.String("request.uri", proxyURL.String()))
	response, err := pm.httpClient.Do(proxyRequest)
	if err != nil {
		return nil, errors.Wrap(err, "can't proxy package request")
	}
	defer response.Body.Close()
	var pkg packages.Package
	err = json.NewDecoder(response.Body).Decode(&pkg)
	if err != nil {
		return nil, errors.Wrap(err, "can't proxy package request")
	}
	pkg.SetRemoteResolver(pm.resolver)
	return &pkg, nil
}
