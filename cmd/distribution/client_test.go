// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientSetsUserAgent(t *testing.T) {
	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newHTTPClient()
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	resp.Body.Close()

	assert.True(t, strings.HasPrefix(gotUA, "elastic-package-registry-distribution/"),
		"expected User-Agent prefix, got %q", gotUA)
}

func TestHTTPClientReusesConnection(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"name":"nginx","version":"1.0.0"}]`))
	})

	for _, tc := range []struct {
		name      string
		newServer func(http.Handler) *httptest.Server
	}{
		{
			name:      "h1",
			newServer: httptest.NewServer,
		},
		{
			name: "h2",
			newServer: func(h http.Handler) *httptest.Server {
				s := httptest.NewUnstartedServer(h)
				s.EnableHTTP2 = true
				s.StartTLS()
				return s
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.newServer(handler)
			defer server.Close()

			client := server.Client()

			var reused atomic.Int64
			var total atomic.Int64

			const requests = 3
			for range requests {
				trace := &httptrace.ClientTrace{
					GotConn: func(info httptrace.GotConnInfo) {
						total.Add(1)
						if info.Reused {
							reused.Add(1)
						}
					},
				}
				ctx := httptrace.WithClientTrace(context.Background(), trace)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
				require.NoError(t, err)

				resp, err := client.Do(req)
				require.NoError(t, err)

				var packages []packageInfo
				err = json.NewDecoder(resp.Body).Decode(&packages)
				require.NoError(t, err)
				drainAndClose(resp.Body)
			}

			assert.Equal(t, int64(requests), total.Load(), "expected %d traced connections", requests)
			assert.Equal(t, int64(requests-1), reused.Load(), "expected %d reused connections", requests-1)
		})
	}
}
