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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/hashicorp/go-retryablehttp"
	"go.elastic.co/apm/module/apmhttp/v2"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/packages"
)

// backend holds the parsed information for a single proxy destination.
type backend struct {
	URL      *url.URL
	Priority int
	Resolver *proxyResolver
}

// ProxyMode now stores the shared transport, not a shared client.
type ProxyMode struct {
	options       ProxyOptions
	httpTransport http.RoundTripper
	backends      []backend
	logger        *zap.Logger
}

// ProxyOptions now supports multiple backends.
type ProxyOptions struct {
	Enabled bool
	ProxyTo []string
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

	// Create one shared, instrumented transport for all clients to use.
	pm.httpTransport = apmhttp.WrapRoundTripper(&http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	})

	// Parse all configured backends.
	pm.backends = make([]backend, 0, len(options.ProxyTo))
	for _, proxyAddr := range options.ProxyTo {
		parts := strings.Split(proxyAddr, ";")
		addr := parts[0]
		priority := 0 // Default priority

		if len(parts) > 1 {
			p, err := strconv.Atoi(parts[1])
			if err == nil {
				priority = p
			} else {
				logger.Warn("invalid priority format in proxy address, using default", zap.String("address", proxyAddr))
			}
		}

		destURL, err := url.Parse(addr)
		if err != nil {
			return nil, fmt.Errorf("can't create proxy destination URL from '%s': %w", addr, err)
		}

		pm.backends = append(pm.backends, backend{
			URL:      destURL,
			Priority: priority,
			Resolver: &proxyResolver{destinationURL: *destURL},
		})
	}

	if len(pm.backends) == 0 {
		return nil, errors.New("proxy mode is enabled but no backends are configured in 'proxy_to'")
	}

	return &pm, nil
}

// proxyRetryPolicy function remains unchanged.
func proxyRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	shouldRetry, err := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	if shouldRetry {
		return shouldRetry, err
	}
	locationHeader := resp.Header.Get("location")
	if locationHeader != "" {
		return false, nil
	}
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

// Search now queries all backends in parallel and merges the results.
func (pm *ProxyMode) Search(r *http.Request) (packages.Packages, error) {
	type searchResult struct {
		Packages packages.Packages
		Backend  backend
		Err      error
	}

	var wg sync.WaitGroup
	resultsChan := make(chan searchResult, len(pm.backends))

	for _, b := range pm.backends {
		wg.Add(1)
		go func(backend backend) {
			defer wg.Done()

			// Create a new, lightweight client for each goroutine.
			httpClient := &retryablehttp.Client{
				HTTPClient: &http.Client{
					Timeout:   10 * time.Second,
					Transport: pm.httpTransport, // All clients share the same transport.
				},
				Logger:       withZapLoggerAdapter(pm.logger),
				RetryWaitMin: 1 * time.Second,
				RetryWaitMax: 15 * time.Second,
				RetryMax:     4,
				CheckRetry:   proxyRetryPolicy,
				Backoff:      retryablehttp.DefaultBackoff,
			}

			proxyURL := *r.URL
			proxyURL.Host = backend.URL.Host
			proxyURL.Scheme = backend.URL.Scheme
			proxyURL.User = backend.URL.User

			proxyRequest, err := retryablehttp.NewRequestWithContext(r.Context(), http.MethodGet, proxyURL.String(), nil)
			if err != nil {
				resultsChan <- searchResult{Err: fmt.Errorf("can't create proxy request for %s: %w", backend.URL, err)}
				return
			}

			pm.logger.Debug("Proxying /search request", zap.String("request.uri", proxyURL.String()))
			response, err := httpClient.Do(proxyRequest)
			if err != nil {
				resultsChan <- searchResult{Err: fmt.Errorf("can't proxy search request to %s: %w", backend.URL, err)}
				return
			}
			defer response.Body.Close()

			var pkgs packages.Packages
			if err := json.NewDecoder(response.Body).Decode(&pkgs); err != nil {
				resultsChan <- searchResult{Err: fmt.Errorf("can't decode search response from %s: %w", backend.URL, err)}
				return
			}
			resultsChan <- searchResult{Packages: pkgs, Backend: backend}
		}(b)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var mergedPackages packages.Packages
	for result := range resultsChan {
		if result.Err != nil {
			pm.logger.Warn("Failed to fetch from proxy backend", zap.Error(result.Err))
			continue
		}
		// Set the resolver for each package to its backend of origin.
		for i := range result.Packages {
			result.Packages[i].SetRemoteResolver(result.Backend.Resolver)
		}
		mergedPackages = mergedPackages.Join(result.Packages)
	}

	return mergedPackages, nil
}

// Categories also queries all backends in parallel.
func (pm *ProxyMode) Categories(r *http.Request) ([]packages.Category, error) {
	type categoriesResult struct {
		Categories []packages.Category
		Err        error
	}

	var wg sync.WaitGroup
	resultsChan := make(chan categoriesResult, len(pm.backends))

	for _, b := range pm.backends {
		wg.Add(1)
		go func(backend backend) {
			defer wg.Done()

			httpClient := &retryablehttp.Client{
				HTTPClient: &http.Client{
					Timeout:   10 * time.Second,
					Transport: pm.httpTransport,
				},
				Logger:       withZapLoggerAdapter(pm.logger),
				RetryWaitMin: 1 * time.Second,
				RetryWaitMax: 15 * time.Second,
				RetryMax:     4,
				CheckRetry:   proxyRetryPolicy,
				Backoff:      retryablehttp.DefaultBackoff,
			}

			proxyURL := *r.URL
			proxyURL.Host = backend.URL.Host
			proxyURL.Scheme = backend.URL.Scheme
			proxyURL.User = backend.URL.User

			proxyRequest, err := retryablehttp.NewRequestWithContext(r.Context(), http.MethodGet, proxyURL.String(), nil)
			if err != nil {
				resultsChan <- categoriesResult{Err: fmt.Errorf("can't create proxy request for %s: %w", backend.URL, err)}
				return
			}

			pm.logger.Debug("Proxying /categories request", zap.String("request.uri", proxyURL.String()))
			response, err := httpClient.Do(proxyRequest)
			if err != nil {
				resultsChan <- categoriesResult{Err: fmt.Errorf("can't proxy categories request to %s: %w", backend.URL, err)}
				return
			}
			defer response.Body.Close()

			var cats []packages.Category
			if err := json.NewDecoder(response.Body).Decode(&cats); err != nil {
				resultsChan <- categoriesResult{Err: fmt.Errorf("can't decode categories response from %s: %w", backend.URL, err)}
				return
			}
			resultsChan <- categoriesResult{Categories: cats}
		}(b)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Merge categories and sum counts for duplicates.
	mergedCategories := make(map[string]packages.Category)
	for result := range resultsChan {
		if result.Err != nil {
			pm.logger.Warn("Failed to fetch categories from proxy backend", zap.Error(result.Err))
			continue
		}
		for _, cat := range result.Categories {
			if existing, ok := mergedCategories[cat.Id]; ok {
				existing.Count += cat.Count
				mergedCategories[cat.Id] = existing
			} else {
				mergedCategories[cat.Id] = cat
			}
		}
	}

	finalList := make([]packages.Category, 0, len(mergedCategories))
	for _, cat := range mergedCategories {
		finalList = append(finalList, cat)
	}

	return finalList, nil
}

// Package queries all backends in parallel and returns the first successful response.
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

	type packageResult struct {
		Package *packages.Package
		Err     error
	}

	// We use a context that can be canceled once we find a result.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	resultsChan := make(chan packageResult, len(pm.backends))
	var wg sync.WaitGroup

	for _, b := range pm.backends {
		wg.Add(1)
		go func(backend backend) {
			defer wg.Done()

			httpClient := &retryablehttp.Client{
				HTTPClient: &http.Client{
					Timeout:   10 * time.Second,
					Transport: pm.httpTransport,
				},
				Logger:       withZapLoggerAdapter(pm.logger),
				RetryWaitMin: 1 * time.Second,
				RetryWaitMax: 15 * time.Second,
				RetryMax:     4,
				CheckRetry:   proxyRetryPolicy,
				Backoff:      retryablehttp.DefaultBackoff,
			}

			urlPath := fmt.Sprintf("/package/%s/%s/", packageName, packageVersion)
			proxyURL := backend.URL.ResolveReference(&url.URL{Path: urlPath})

			proxyRequest, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, proxyURL.String(), nil)
			if err != nil {
				resultsChan <- packageResult{Err: fmt.Errorf("can't create proxy request for %s: %w", backend.URL, err)}
				return
			}

			pm.logger.Debug("Proxying /package request", zap.String("request.uri", proxyURL.String()))
			response, err := httpClient.Do(proxyRequest)
			if err != nil {
				resultsChan <- packageResult{Err: fmt.Errorf("can't proxy package request to %s: %w", backend.URL, err)}
				return
			}
			defer response.Body.Close()

			switch response.StatusCode {
			case http.StatusOK:
				var pkg packages.Package
				if err := json.NewDecoder(response.Body).Decode(&pkg); err != nil {
					resultsChan <- packageResult{Err: fmt.Errorf("can't decode package response from %s: %w", backend.URL, err)}
					return
				}
				pkg.SetRemoteResolver(backend.Resolver)
				resultsChan <- packageResult{Package: &pkg}
			case http.StatusNotFound:
				// Not an error, just not found on this backend.
				resultsChan <- packageResult{Package: nil}
			default:
				resultsChan <- packageResult{Err: fmt.Errorf("unexpected status code %d from %s", response.StatusCode, backend.URL)}
			}
		}(b)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var lastErr error
	for result := range resultsChan {
		if result.Err != nil {
			lastErr = result.Err
			pm.logger.Warn("Failed to fetch package from proxy backend", zap.Error(result.Err))
			continue
		}
		// If we found a package, cancel other requests and return immediately.
		if result.Package != nil {
			cancel() // Signal other goroutines to stop.
			return result.Package, nil
		}
	}

	// If we get here, no package was found on any backend.
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil // Not found
}
