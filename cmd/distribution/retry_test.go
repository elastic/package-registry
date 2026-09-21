// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient returns an *http.Client whose retryTransport records backoff
// durations but does not actually sleep, and whose pacer also skips real
// sleeping. This makes retry behaviour deterministic and instant in tests.
// The returned slice accumulates all sleep durations requested by the retry
// logic (not the pacer). Use the slice to assert on Retry-After handling.
func newTestClient() (*http.Client, *[]time.Duration) {
	var mu sync.Mutex
	var sleeps []time.Duration

	rt := newRetryTransport(http.DefaultTransport)
	rt.sleep = func(ctx context.Context, d time.Duration) error {
		mu.Lock()
		sleeps = append(sleeps, d)
		mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	// Deterministic jitter: always return half of cap so tests are predictable.
	rt.jitter = func(max time.Duration) time.Duration {
		return max / 2
	}
	rt.pacer.sleep = func(ctx context.Context, d time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	return &http.Client{Transport: rt}, &sleeps
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
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]packageInfo{{Name: "nginx", Version: "1.0.0"}})
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

// TestRetryTransportRetryAfterHeader checks that a Retry-After header is
// honoured instead of the computed jitter backoff.
func TestRetryTransportRetryAfterHeader(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		wantSeconds float64
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
			name:        "garbage falls back to jitter",
			header:      "not-a-date",
			wantSeconds: 0.5, // jitter = cap/2; cap at attempt 0 = retryBaseWait = 1s; 1s/2=0.5s
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

			client, sleeps := newTestClient()
			resp, err := client.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, int64(2), hits.Load())
			require.Len(t, *sleeps, 1, "expected exactly one sleep")
			assert.InDelta(t, tt.wantSeconds, (*sleeps)[0].Seconds(), 0.001)
		})
	}
}

// TestRetryTransportContextCancelledDuringBackoff checks that cancelling the
// context during a backoff wait stops the retry loop immediately.
func TestRetryTransportContextCancelledDuringBackoff(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	rt := newRetryTransport(http.DefaultTransport)
	// The sleep function cancels the request context before returning, simulating
	// a cancellation that arrives during the backoff wait.
	var cancelFn context.CancelFunc
	rt.sleep = func(ctx context.Context, d time.Duration) error {
		cancelFn() // cancel via the outer context
		return ctx.Err()
	}
	rt.jitter = func(max time.Duration) time.Duration { return 0 }
	rt.pacer.sleep = func(ctx context.Context, d time.Duration) error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	cancelFn = cancel
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	client := &http.Client{Transport: rt}
	_, err = client.Do(req)
	require.Error(t, err)
	assert.Equal(t, int64(1), hits.Load(),
		"should stop after the first attempt when context is cancelled during backoff")
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

// TestPacerPenalizeAndReward checks the interval arithmetic for penalize and
// reward.
func TestPacerPenalizeAndReward(t *testing.T) {
	t.Run("penalize from zero reaches min interval then doubles", func(t *testing.T) {
		p := newPacer()
		p.penalize()
		assert.Equal(t, 500*time.Millisecond, p.interval,
			"first penalize: 0 → pacerMinInterval(250ms) → ×2 = 500ms")
		p.penalize()
		assert.Equal(t, 1*time.Second, p.interval)
		p.penalize()
		assert.Equal(t, 2*time.Second, p.interval)
		p.penalize()
		assert.Equal(t, 4*time.Second, p.interval)
		p.penalize()
		assert.Equal(t, pacerMaxInterval, p.interval, "saturates at pacerMaxInterval")
		p.penalize()
		assert.Equal(t, pacerMaxInterval, p.interval, "stays at pacerMaxInterval")
	})

	t.Run("reward decays to zero", func(t *testing.T) {
		p := newPacer()
		p.interval = pacerMaxInterval
		// Decay: interval = interval*3/4, snapped to 0 below pacerMinInterval/2 = 125ms.
		prev := p.interval
		for p.interval > 0 {
			p.reward()
			if p.interval > 0 {
				assert.Less(t, p.interval, prev, "reward should decrease interval")
				prev = p.interval
			}
		}
		assert.Equal(t, time.Duration(0), p.interval, "reward eventually snaps to zero")
	})
}

// TestPacerWaitsAreSpaced verifies that successive calls to wait are spaced by
// at least interval. The test uses a fixed fake clock (all callers see the same
// wall time) and a no-op sleep, so it is deterministic and instant.
func TestPacerWaitsAreSpaced(t *testing.T) {
	const interval = 200 * time.Millisecond

	// Fixed fake time: all calls to now() return the same instant, simulating
	// callers arriving simultaneously (worst case for spacing correctness).
	fixedNow := time.Now()
	fakeClock := func() time.Time { return fixedNow }

	var slept []time.Duration
	fakeSleep := func(ctx context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}

	p := &pacer{
		interval: interval,
		now:      fakeClock,
		sleep:    fakeSleep,
	}

	// Call wait 3 times (sequential, all seeing the same "now").
	// 1st: p.next is zero (past) → delay=0, p.next advances to now+interval.
	// 2nd: p.next=now+interval > now → delay=interval, p.next→now+2×interval.
	// 3rd: p.next=now+2×interval > now → delay=2×interval.
	for range 3 {
		require.NoError(t, p.wait(context.Background()))
	}

	require.Len(t, slept, 2, "first caller needs no sleep; 2nd and 3rd do")
	assert.Equal(t, interval, slept[0], "2nd caller waits one interval")
	assert.Equal(t, 2*interval, slept[1], "3rd caller waits two intervals")
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
