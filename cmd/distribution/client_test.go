// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
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

// trustServer makes the transport trust the httptest server's certificate.
func trustServer(server *httptest.Server) func(*http.Transport) {
	return func(tr *http.Transport) {
		if server.Certificate() == nil {
			return
		}
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{}
		}
		tr.TLSClientConfig.RootCAs = x509.NewCertPool()
		tr.TLSClientConfig.RootCAs.AddCert(server.Certificate())
	}
}

// countReusedConns does n sequential requests and returns how many used a
// connection that was already established.
func countReusedConns(t *testing.T, client *http.Client, url string, n int) (total, reused int64) {
	t.Helper()
	for range n {
		trace := &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) {
				total++
				if info.Reused {
					reused++
				}
			},
		}
		ctx := httptrace.WithClientTrace(context.Background(), trace)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)

		var packages []packageInfo
		err = json.NewDecoder(resp.Body).Decode(&packages)
		require.NoError(t, err)
		drainAndClose(resp.Body)
	}
	return total, reused
}

var testServers = []struct {
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
}

var packagesHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`[{"name":"nginx","version":"1.0.0"}]`))
})

func TestHTTPClientReusesConnection(t *testing.T) {
	for _, tc := range testServers {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.newServer(packagesHandler)
			defer server.Close()

			client := newHTTPClient(trustServer(server))

			const requests = 3
			total, reused := countReusedConns(t, client, server.URL, requests)
			assert.Equal(t, int64(requests), total, "expected %d traced connections", requests)
			assert.Equal(t, int64(requests-1), reused, "expected %d reused connections", requests-1)
		})
	}
}

// TestHTTPClientReuseDetectsDisabledKeepAlives proves the reuse check above
// can fail: a transport with keep-alives disabled, applied through
// newHTTPClient's own wrapping, must show no reused connections.
func TestHTTPClientReuseDetectsDisabledKeepAlives(t *testing.T) {
	for _, tc := range testServers {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.newServer(packagesHandler)
			defer server.Close()

			client := newHTTPClient(trustServer(server), func(tr *http.Transport) {
				tr.DisableKeepAlives = true
			})

			const requests = 3
			total, reused := countReusedConns(t, client, server.URL, requests)
			assert.Equal(t, int64(requests), total)
			assert.Zero(t, reused, "no connection should be reused with keep-alives disabled")
		})
	}
}
