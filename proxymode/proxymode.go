// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package proxymode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/hashicorp/go-retryablehttp"
	"go.elastic.co/apm/module/apmhttp/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/packages"
)

type ProxyMode struct {
	options ProxyOptions

	httpClient     *retryablehttp.Client
	destinationURL *url.URL
	resolver       *proxyResolver

	logger *zap.Logger
}

type ProxyOptions struct {
	Enabled bool
	ProxyTo string
}

func NoProxy(logger *zap.Logger) *ProxyMode {
	proxyMode, err := NewProxyMode(logger, ProxyOptions{Enabled: false})
	if err != nil {
		panic(fmt.Errorf("unexpected error: %w", err))
	}
	return proxyMode
}

func NewProxyMode(logger *zap.Logger, options ProxyOptions) (*ProxyMode, error) {
	var pm ProxyMode
	pm.options = options
	pm.logger = logger

	if !options.Enabled {
		return &pm, nil
	}

	pm.httpClient = &retryablehttp.Client{
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: apmhttp.WrapRoundTripper(&http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			}),
		},
		Logger:       withZapLoggerAdapter(logger),
		RetryWaitMin: 1 * time.Second,
		RetryWaitMax: 15 * time.Second,
		RetryMax:     4,
		CheckRetry:   proxyRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
	}

	var err error
	pm.destinationURL, err = url.Parse(pm.options.ProxyTo)
	if err != nil {
		return nil, fmt.Errorf("can't create proxy destination URL: %w", err)
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

	// Expect json content type only for success statuses.
	if code := resp.StatusCode; code >= 200 && code < 300 {
		contentType := resp.Header.Get("content-type")
		if !strings.HasPrefix(contentType, "application/json") {
			return true, fmt.Errorf("unexpected content type: %s", contentType)
		}
	}

	return false, nil
}

func (pm *ProxyMode) Enabled() bool {
	if pm == nil {
		return false
	}
	return pm.options.Enabled
}

func (pm *ProxyMode) Search(r *http.Request) (packages.Packages, error) {

	proxyURL := *r.URL
	proxyURL.Host = pm.destinationURL.Host
	proxyURL.Scheme = pm.destinationURL.Scheme
	proxyURL.User = pm.destinationURL.User

	proxyRequest, err := retryablehttp.NewRequest(http.MethodGet, proxyURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("can't create proxy request: %w", err)
	}

	proxyRequest.Request = proxyRequest.Request.WithContext(r.Context())

	pm.logger.Debug("Proxy /search request", zap.String("request.uri", proxyURL.String()))
	response, err := pm.httpClient.Do(proxyRequest)
	if err != nil {
		return nil, fmt.Errorf("can't proxy search request: %w", err)
	}
	defer response.Body.Close()
	var pkgs packages.Packages
	err = json.NewDecoder(response.Body).Decode(&pkgs)
	if err != nil {
		return nil, fmt.Errorf("can't proxy search request: %w", err)
	}
	for i := 0; i < len(pkgs); i++ {
		pkgs[i].SetRemoteResolver(pm.resolver)
	}
	return pkgs, nil
}

func (pm *ProxyMode) Categories(r *http.Request) ([]packages.Category, error) {

	proxyURL := *r.URL
	proxyURL.Host = pm.destinationURL.Host
	proxyURL.Scheme = pm.destinationURL.Scheme
	proxyURL.User = pm.destinationURL.User

	proxyRequest, err := retryablehttp.NewRequest(http.MethodGet, proxyURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("can't create proxy request: %w", err)
	}

	proxyRequest.Request = proxyRequest.Request.WithContext(r.Context())

	pm.logger.Debug("Proxy /categories request", zap.String("request.uri", proxyURL.String()))
	response, err := pm.httpClient.Do(proxyRequest)
	if err != nil {
		return nil, fmt.Errorf("can't proxy categories request: %w", err)
	}
	defer response.Body.Close()
	var cats []packages.Category
	err = json.NewDecoder(response.Body).Decode(&cats)
	if err != nil {
		return nil, fmt.Errorf("can't proxy categories request: %w", err)
	}
	return cats, nil
}

func (pm *ProxyMode) Package(r *http.Request) (*packages.Package, error) {

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
		return nil, fmt.Errorf("can't create proxy request: %w", err)
	}

	proxyRequest.Request = proxyRequest.Request.WithContext(r.Context())

	pm.logger.Debug("Proxy /package request", zap.String("request.uri", proxyURL.String()))
	response, err := pm.httpClient.Do(proxyRequest)
	if err != nil {
		return nil, fmt.Errorf("can't proxy package request: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		// Package found, all good.
	case http.StatusNotFound:
		// Package doesn't exist, don't try to parse the response, just return an empty package.
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected status code %d received", response.StatusCode)
	}

	var pkg packages.Package
	err = json.NewDecoder(response.Body).Decode(&pkg)
	if err != nil {
		return nil, fmt.Errorf("can't proxy package request: %w", err)
	}
	pkg.SetRemoteResolver(pm.resolver)
	return &pkg, nil
}
