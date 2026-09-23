// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"io"
	"net/http"
	"runtime/debug"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"golang.org/x/time/rate"
)

// searchConcurrency bounds how many search requests are in flight at once.
// A single all=true search response is ~22 MB on the origin before gzip; two
// concurrent requests cost ~44 MB instead of ~90 MB at concurrency 4.
// Deliberately a constant and not a config knob: it exists to protect the
// registry, not to be tuned per run.
const searchConcurrency = 2

// downloadConcurrency bounds how many package downloads are in flight at once.
// Downloads are static files served from object storage, so a higher
// concurrency than search is safe.
// Deliberately a constant and not a config knob.
const downloadConcurrency = 4

const (
	// searchTimeout covers up to maxAttempts attempts (ResponseHeaderTimeout
	// 30 s each) plus up to maxAttempts-1 backoff sleeps capped at retryMaxWait:
	// 4×30 s + 3×30 s = 210 s < 5 min with headroom to spare.
	searchTimeout   = 5 * time.Minute
	downloadTimeout = 15 * time.Minute
)

// httpClient is shared by the search and the download phases.
var httpClient = newHTTPClient()

func newHTTPClient() *http.Client {
	// Clone keeps DefaultTransport's proxy, dial and TLS timeouts, and its
	// HTTP/2 support. HTTP/2 must stay enabled: the registry serves h2 and a
	// single multiplexed connection is what keeps this tool cheap for it.
	// Do not set DialContext or TLSClientConfig without also keeping
	// ForceAttemptHTTP2: Transport.protocols() disables automatic h2 when a
	// custom dialer is present. Clone() preserves the flag.
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// HTTP/1.1 only: the default of 2 closes connections while other workers
	// are still running. No effect on the h2 path.
	transport.MaxIdleConnsPerHost = max(searchConcurrency, downloadConcurrency)
	transport.ResponseHeaderTimeout = 30 * time.Second

	// ResponseHeaderTimeout is not propagated to HTTP/2, so ping a quiet
	// connection instead: without this a dead h2 conn hangs a worker forever.
	transport.HTTP2 = &http.HTTP2Config{
		SendPingTimeout: 15 * time.Second,
		PingTimeout:     15 * time.Second,
	}

	inner := &limiterTransport{
		next:    &userAgentTransport{next: transport, userAgent: buildUserAgent()},
		limiter: rate.NewLimiter(registryRate, registryBurst),
	}

	rc := retryablehttp.NewClient()
	rc.RetryMax = maxAttempts - 1
	rc.RetryWaitMin = retryBaseWait
	rc.RetryWaitMax = retryMaxWait
	rc.Backoff = retryAfterBackoff
	rc.ErrorHandler = retryablehttp.PassthroughErrorHandler
	rc.Logger = nil
	rc.HTTPClient = &http.Client{Transport: inner}

	return rc.StandardClient()
}

type userAgentTransport struct {
	next      http.RoundTripper
	userAgent string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context()) // mutating the caller's headers is a contract violation
	req.Header.Set("User-Agent", t.userAgent)
	return t.next.RoundTrip(req)
}

// drainAndClose returns the connection to the pool: net/http only reuses an
// HTTP/1.1 connection whose body was read to EOF, and json.Decoder stops at
// the closing bracket. The limit keeps an abandoned body from being read in
// full.
func drainAndClose(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 64<<10))
	_ = body.Close()
}

func buildUserAgent() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "elastic-package-registry-distribution/dev"
	}
	version := info.Main.Version
	if version == "" || version == "(devel)" {
		version = "dev"
	}
	return "elastic-package-registry-distribution/" + version
}
