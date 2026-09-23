// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

const (
	// maxAttempts is the total number of attempts per request (1 initial + 6 retries).
	maxAttempts = 7

	retryBaseWait = 1 * time.Second
	retryMaxWait  = 30 * time.Second
)

const (
	// registryRate caps sustained requests per second across all workers,
	// regardless of outcome. A failing request is cheaper and therefore
	// faster than a successful one, so rate — not concurrency — is what
	// actually protects the origin.
	// Sized for the search phase (large slow responses) and download phase
	// combined: on a healthy registry the natural throughput of 22 MB search
	// responses keeps the actual rate well below this ceiling.
	registryRate  = 20
	registryBurst = 4 // matches downloadConcurrency
)

// limiterTransport bounds the request rate to the registry. It sits inside the
// retry client, so retries consume budget too.
type limiterTransport struct {
	next    http.RoundTripper
	limiter *rate.Limiter
}

func (t *limiterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return t.next.RoundTrip(req)
}

// retryAfterBackoff is a go-retryablehttp Backoff function that honours
// Retry-After headers and applies full-jitter exponential backoff otherwise.
// It emits a retry notice to stderr before each retry wait.
func retryAfterBackoff(minWait, maxWait time.Duration, attemptNum int, resp *http.Response) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if d, ok := parseRetryAfter(ra); ok {
				d = min(d, maxWait)
				fmt.Fprintf(os.Stderr, "retrying in %.1fs (status %d, Retry-After)\n",
					d.Seconds(), resp.StatusCode)
				return d
			}
		}
	}
	cap := min(minWait<<attemptNum, maxWait)
	var wait time.Duration
	if cap > 0 {
		wait = rand.N(cap)
	}
	if resp != nil {
		fmt.Fprintf(os.Stderr, "retrying in %.1fs (status %d)\n", wait.Seconds(), resp.StatusCode)
	} else {
		fmt.Fprintf(os.Stderr, "retrying in %.1fs\n", wait.Seconds())
	}
	return wait
}

// parseRetryAfter parses a Retry-After header value, which is either a
// non-negative integer number of seconds or an HTTP-date.
func parseRetryAfter(s string) (time.Duration, bool) {
	if secs, err := strconv.Atoi(s); err == nil {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(s); err == nil {
		return max(time.Until(t), 0), true
	}
	return 0, false
}
