// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	// maxAttempts is the total number of attempts per request (1 initial + 3 retries).
	maxAttempts = 4

	retryBaseWait = 1 * time.Second
	retryMaxWait  = 30 * time.Second
)

const (
	pacerMinInterval = 250 * time.Millisecond
	pacerMaxInterval = 5 * time.Second
)

// retryTransport wraps an http.RoundTripper and adds automatic retry with
// full-jitter exponential backoff for transient server errors (429, 5xx) and
// transport failures. It also coordinates a shared pacer that throttles all
// workers when the server signals distress, reducing origin memory pressure
// without requiring the caller to change anything.
type retryTransport struct {
	next  http.RoundTripper
	pacer *pacer

	// sleep is called between retry attempts. The field is a test seam;
	// the production value is sleepCtx.
	sleep func(ctx context.Context, d time.Duration) error
	// jitter returns a draw from [0, max). The field is a test seam;
	// the production value calls rand.N.
	jitter func(max time.Duration) time.Duration
}

func newRetryTransport(next http.RoundTripper) *retryTransport {
	return &retryTransport{
		next:  next,
		pacer: newPacer(),
		sleep: sleepCtx,
		jitter: func(max time.Duration) time.Duration {
			return rand.N(max)
		},
	}
}

// RoundTrip executes the request and retries on retriable errors up to
// maxAttempts times in total. Context cancellation cuts retries short
// immediately — context errors are never retried, so a fail-fast cancel in
// collect does not amplify the request count.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only retry idempotent, body-less requests. All registry endpoints are
	// GET with no body; this guard keeps the transport correct if a non-GET
	// is ever added.
	if req.Body != nil {
		return t.next.RoundTrip(req)
	}

	for attempt := 0; ; attempt++ {
		if err := t.pacer.wait(req.Context()); err != nil {
			return nil, err
		}

		resp, err := t.next.RoundTrip(req)

		if shouldRetry(resp, err) {
			t.pacer.penalize()
		} else {
			t.pacer.reward()
			return resp, err
		}

		// Exhausted attempts — return whatever we have and let the caller
		// surface the error. Do not drain here: the caller may still read
		// the body (e.g. to decode an error payload).
		if attempt == maxAttempts-1 {
			return resp, err
		}

		wait := t.backoff(attempt, resp)
		// Emit to stderr: the tool's normal output (progress lines, package
		// names) goes to stdout and may be piped to a consumer; retry noise
		// belongs on stderr.
		if resp != nil {
			fmt.Fprintf(os.Stderr, "retrying %s in %.1fs (status %d)\n",
				req.URL, wait.Seconds(), resp.StatusCode)
			drainAndClose(resp.Body)
		} else {
			fmt.Fprintf(os.Stderr, "retrying %s in %.1fs (%v)\n",
				req.URL, wait.Seconds(), err)
		}
		if err := t.sleep(req.Context(), wait); err != nil {
			return nil, err
		}
	}
}

// shouldRetry reports whether the response or error warrants another attempt.
func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		// Context errors are not retried: they indicate either a per-request
		// deadline expiry (no point retrying) or a fail-fast cancellation from
		// collect (retrying would amplify the exact storm we are trying to avoid).
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		return true // transport error (network, DNS, TLS, …)
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// backoff computes the wait before the next attempt. If the response carries a
// valid Retry-After header it is used, clamped to retryMaxWait. Otherwise full
// jitter is applied: a random value in [0, min(cap, retryMaxWait)) where
// cap = retryBaseWait << attempt.
func (t *retryTransport) backoff(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if d, ok := parseRetryAfter(ra); ok {
				if d > retryMaxWait {
					return retryMaxWait
				}
				return d
			}
		}
	}
	cap := retryBaseWait << attempt
	if cap > retryMaxWait {
		cap = retryMaxWait
	}
	if cap == 0 {
		return 0
	}
	return t.jitter(cap)
}

// parseRetryAfter parses a Retry-After header value, which is either a
// non-negative integer number of seconds or an HTTP-date.
func parseRetryAfter(s string) (time.Duration, bool) {
	if secs, err := strconv.Atoi(s); err == nil {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(s); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}

// sleepCtx sleeps for d, returning ctx.Err() if the context is done first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// pacer serialises inter-request gaps when the registry signals distress.
// All workers share one instance, so a 502 received by one goroutine slows
// all of them down — not just the one that got the error. Starting at an
// interval of zero means a healthy registry incurs no delay at all.
type pacer struct {
	mu       sync.Mutex
	interval time.Duration // starts at 0
	next     time.Time     // earliest time the next slot may fire

	// now and sleep are test seams; the production values are time.Now and
	// sleepCtx.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

func newPacer() *pacer {
	return &pacer{
		now:   time.Now,
		sleep: sleepCtx,
	}
}

// wait blocks until the caller's slot is available, then reserves the next
// slot for the following caller. The lock is held only long enough to compute
// the delay and advance next; the actual sleep happens outside the lock so
// concurrent callers are spaced by interval rather than all waking together.
func (p *pacer) wait(ctx context.Context) error {
	p.mu.Lock()
	now := p.now()
	var delay time.Duration
	if p.next.After(now) {
		delay = p.next.Sub(now)
	}
	fire := now.Add(delay)
	p.next = fire.Add(p.interval)
	p.mu.Unlock()

	if delay <= 0 {
		return nil
	}
	return p.sleep(ctx, delay)
}

// penalize increases the inter-request interval toward pacerMaxInterval.
// Called after a retriable error.
func (p *pacer) penalize() {
	p.mu.Lock()
	defer p.mu.Unlock()
	interval := p.interval
	if interval < pacerMinInterval {
		interval = pacerMinInterval
	}
	interval *= 2
	if interval > pacerMaxInterval {
		interval = pacerMaxInterval
	}
	p.interval = interval
}

// reward decays the inter-request interval back toward zero.
// Called after a successful response.
func (p *pacer) reward() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.interval = p.interval * 3 / 4
	// Snap to zero below the effective threshold to avoid a long tail where
	// the interval shrinks but never quite reaches zero.
	if p.interval < pacerMinInterval/2 {
		p.interval = 0
	}
}
