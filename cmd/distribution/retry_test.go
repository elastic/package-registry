// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// newTestClient returns an *http.Client backed by retryablehttp for tests.
// The rate limiter is set to infinite so tests aren't rate-limited.
// The backoff records computed durations but does not actually sleep.
// Use the returned slice to inspect backoff values.
func newTestClient() (*http.Client, *[]time.Duration) {
	var mu sync.Mutex
	var waits []time.Duration

	inner := &limiterTransport{
		next:    http.DefaultTransport,
		limiter: rate.NewLimiter(rate.Inf, registryBurst),
	}

	rc := retryablehttp.NewClient()
	rc.RetryMax = maxAttempts - 1
	rc.RetryWaitMin = retryBaseWait
	rc.RetryWaitMax = retryMaxWait
	rc.Backoff = func(minW, maxW time.Duration, attemptNum int, resp *http.Response) time.Duration {
		d := retryAfterBackoff(minW, maxW, attemptNum, resp)
		mu.Lock()
		waits = append(waits, d)
		mu.Unlock()
		return 0 // skip actual sleep
	}
	rc.ErrorHandler = retryablehttp.PassthroughErrorHandler
	rc.Logger = nil
	rc.HTTPClient = &http.Client{Transport: inner}

	return rc.StandardClient(), &waits
}

// TestRetryTransportRetriesTransientStatuses checks that each 5xx/429 status
// is retried and the request eventually succeeds.
func TestRetryTransportRetriesTransientStatuses(t *testing.T) {
	retryStatuses := []int{
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}

	for _, status := range retryStatuses {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			var hits atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := hits.Add(1)
				if n == 1 {
					w.WriteHeader(status)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client, _ := newTestClient()
			resp, err := client.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, int64(2), hits.Load(), "expected 2 hits: one failure then one success")
		})
	}
}

// TestRetryTransportNoRetryOnClientErrors checks that 4xx errors (other than
// 429) are not retried.
func TestRetryTransportNoRetryOnClientErrors(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusBadRequest} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			var hits atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				w.WriteHeader(status)
			}))
			defer server.Close()

			client, _ := newTestClient()
			resp, err := client.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, status, resp.StatusCode)
			assert.Equal(t, int64(1), hits.Load(), "4xx errors must not be retried")
		})
	}
}

// TestRetryTransportExhaustsMaxAttempts checks that a permanently failing
// server is hit exactly maxAttempts times.
func TestRetryTransportExhaustsMaxAttempts(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client, _ := newTestClient()
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.Equal(t, int64(maxAttempts), hits.Load(),
		"expected exactly maxAttempts=%d total requests", maxAttempts)
}

// TestExhaustedRetriesSurfaceStatusCode guards the PassthroughErrorHandler
// decision: after all retries are spent the real HTTP status code is returned
// rather than an error wrapping "giving up after N attempt(s)".
func TestExhaustedRetriesSurfaceStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, _ := newTestClient()
	resp, err := client.Get(server.URL)
	require.NoError(t, err, "PassthroughErrorHandler must not convert the response to an error")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode,
		"real status code must be surfaced after exhausted retries")
}

// TestRetryTransportRetryAfterHeader checks that a Retry-After header is
// honoured instead of the computed jitter backoff.
func TestRetryTransportRetryAfterHeader(t *testing.T) {
	tests := []struct {
		name         string
		header       string
		wantSeconds  float64
		wantInRange  bool    // true when the result is a range, not an exact value
		wantRangeMax float64 // upper bound when wantInRange is true
	}{
		{
			name:        "integer seconds",
			header:      "2",
			wantSeconds: 2,
		},
		{
			name:        "clamped to retryMaxWait",
			header:      "120",
			wantSeconds: retryMaxWait.Seconds(),
		},
		{
			name:         "garbage falls back to jitter",
			header:       "not-a-date",
			wantInRange:  true,
			wantRangeMax: retryBaseWait.Seconds(), // jitter in [0, retryBaseWait) for attempt 0
		},
		{
			name:        "HTTP-date in the past gives zero delay",
			header:      "Thu, 01 Jan 1970 00:00:00 GMT",
			wantSeconds: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := hits.Add(1)
				if n == 1 {
					w.Header().Set("Retry-After", tt.header)
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client, waits := newTestClient()
			resp, err := client.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, int64(2), hits.Load())
			require.Len(t, *waits, 1, "expected exactly one backoff computation")
			if tt.wantInRange {
				assert.GreaterOrEqual(t, (*waits)[0].Seconds(), 0.0)
				assert.Less(t, (*waits)[0].Seconds(), tt.wantRangeMax,
					"jitter backoff must be below cap")
			} else {
				assert.InDelta(t, tt.wantSeconds, (*waits)[0].Seconds(), 0.001)
			}
		})
	}
}

// TestRetryTransportContextCancelledDuringBackoff checks that cancelling the
// context during a retry wait stops the retry loop without waiting the full
// backoff duration.
func TestRetryTransportContextCancelledDuringBackoff(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	// Use real, long retry waits so that context cancellation wins the race.
	inner := &limiterTransport{
		next:    http.DefaultTransport,
		limiter: rate.NewLimiter(rate.Inf, registryBurst),
	}
	rc := retryablehttp.NewClient()
	rc.RetryMax = maxAttempts - 1
	rc.RetryWaitMin = 10 * time.Second
	rc.RetryWaitMax = 10 * time.Second
	rc.Backoff = func(minW, maxW time.Duration, attemptNum int, resp *http.Response) time.Duration {
		return minW
	}
	rc.ErrorHandler = retryablehttp.PassthroughErrorHandler
	rc.Logger = nil
	rc.HTTPClient = &http.Client{Transport: inner}
	client := rc.StandardClient()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel during the backoff wait — after the first hit.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	start := time.Now()
	_, err = client.Do(req)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Equal(t, int64(1), hits.Load(),
		"should stop after the first attempt when context is cancelled during backoff")
	assert.Less(t, elapsed, 5*time.Second, "should not wait the full retry duration")
}

// TestRetryTransportAlreadyCancelledContext checks that a pre-cancelled context
// causes the transport to return a context error without triggering a retry storm.
func TestRetryTransportAlreadyCancelledContext(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := newTestClient()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the request

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.Error(t, err)
	assert.Equal(t, int64(0), hits.Load(),
		"a pre-cancelled context must produce no server hits")
}

// TestRateLimitedOnSuccess verifies that the limiterTransport bounds throughput
// on fast 200 responses — the case the former pacer never covered.
func TestRateLimitedOnSuccess(t *testing.T) {
	const rps = 50.0
	const burst = 4
	const n = burst + 6 // requests beyond the burst must be spaced by 1/rps

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	inner := &limiterTransport{
		next:    http.DefaultTransport,
		limiter: rate.NewLimiter(rps, burst),
	}
	client := &http.Client{Transport: inner}

	start := time.Now()
	for range n {
		resp, err := client.Get(server.URL)
		require.NoError(t, err)
		resp.Body.Close()
	}
	elapsed := time.Since(start)

	// After exhausting the burst, the remaining (n-burst) requests must each
	// wait at least 1/rps seconds.
	minExpected := time.Duration(float64(n-burst) / rps * float64(time.Second))
	assert.GreaterOrEqual(t, elapsed, minExpected,
		"rate limiter must slow down requests beyond the burst")
}

// TestRateLimitedOnFailure verifies that the limiterTransport bounds throughput
// even when every response is a failure, preventing a retry storm.
func TestRateLimitedOnFailure(t *testing.T) {
	const rps = 50.0
	const burst = 4
	const n = burst + 4

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	inner := &limiterTransport{
		next:    http.DefaultTransport,
		limiter: rate.NewLimiter(rps, burst),
	}
	client := &http.Client{Transport: inner}

	start := time.Now()
	for range n {
		resp, err := client.Get(server.URL)
		require.NoError(t, err)
		resp.Body.Close()
	}
	elapsed := time.Since(start)

	minExpected := time.Duration(float64(n-burst) / rps * float64(time.Second))
	assert.GreaterOrEqual(t, elapsed, minExpected,
		"rate limiter must bound request rate on failure too")
}

// TestContextCancelledDuringLimiterWait checks that cancelling the context
// while waiting for a rate-limiter token returns promptly without sending a request.
func TestContextCancelledDuringLimiterWait(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Rate effectively zero after the burst so the second request blocks indefinitely.
	limiter := rate.NewLimiter(rate.Limit(0.001), 1)
	inner := &limiterTransport{next: http.DefaultTransport, limiter: limiter}
	client := &http.Client{Transport: inner}

	// First request consumes the single burst token.
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, int64(1), hits.Load())

	// Second request: cancel context before the limiter grants a token.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	start := time.Now()
	_, err = client.Do(req)
	elapsed := time.Since(start)

	require.Error(t, err, "limiter Wait must return an error on context cancellation")
	assert.Less(t, elapsed, 1*time.Second, "should return promptly after context cancellation")
	assert.Equal(t, int64(1), hits.Load(), "no request must be sent after context cancellation")
}

// TestRetryAfterBackoffJitter verifies that jitter values differ across calls
// and all stay within the expected range.
func TestRetryAfterBackoffJitter(t *testing.T) {
	const attemptNum = 0
	cap := retryBaseWait << attemptNum // 1 s for attempt 0

	seen := make(map[time.Duration]bool)
	for range 20 {
		d := retryAfterBackoff(retryBaseWait, retryMaxWait, attemptNum, nil)
		assert.GreaterOrEqual(t, d, time.Duration(0))
		assert.Less(t, d, cap)
		seen[d] = true
	}
	assert.Greater(t, len(seen), 1, "jitter values must not all be identical")
}

// TestParseRetryAfter checks the Retry-After parsing helpers.
func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		input  string
		want   time.Duration
		wantOK bool
	}{
		{"0", 0, true},
		{"5", 5 * time.Second, true},
		{"120", 120 * time.Second, true},
		{"not-a-number", 0, false},
		{"Thu, 01 Jan 1970 00:00:00 GMT", 0, true}, // in the past → 0
		{"garbage date", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseRetryAfter(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			if ok && tt.want > 0 {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
