// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCaptureZapFieldsForRequest(t *testing.T) {
	cases := []struct {
		title           string
		method          string
		url             string
		remoteAddress   string
		handler         http.Handler
		expectedMessage string
		expectedFields  []zap.Field
	}{
		{
			title: "Normal GET",
			handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Write([]byte("Hello!"))
			}),
			method:          "GET",
			url:             "http://epr.elastic.co/search?package=foo&all=true",
			remoteAddress:   "233.252.0.252:33442",
			expectedMessage: "GET /search HTTP/1.1",
			expectedFields: []zap.Field{
				zap.Int64("http.response.code", 200),
				zap.Int("http.response.body.bytes", 6),
				zap.String("http.request.method", "GET"),
				zap.String("source.address", "233.252.0.252"),
				zap.String("source.ip", "233.252.0.252"),
				zap.String("url.domain", "epr.elastic.co"),
				zap.String("url.path", "/search"),
				zap.String("url.query", "package=foo&all=true"),
			},
		},
		{
			title:           "404",
			handler:         http.NotFoundHandler(),
			method:          "GET",
			url:             "http://epr.elastic.co/foo",
			remoteAddress:   "233.252.0.252:33442",
			expectedMessage: "GET /foo HTTP/1.1",
			expectedFields: []zap.Field{
				zap.Int64("http.response.code", 404),
				zap.Int("http.response.body.bytes", 19),
				zap.String("http.request.method", "GET"),
				zap.String("source.address", "233.252.0.252"),
				zap.String("source.ip", "233.252.0.252"),
				zap.String("url.domain", "epr.elastic.co"),
				zap.String("url.path", "/foo"),
			},
		},
		{
			title: "Empty response on 500 error",
			handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
			method:          "OPTIONS",
			url:             "http://epr.elastic.co/search",
			remoteAddress:   "233.252.0.252:33442",
			expectedMessage: "OPTIONS /search HTTP/1.1",
			expectedFields: []zap.Field{
				zap.Int64("http.response.code", 500),
				zap.Int("http.response.body.bytes", 0),
				zap.String("http.request.method", "OPTIONS"),
				zap.String("source.address", "233.252.0.252"),
				zap.String("source.ip", "233.252.0.252"),
				zap.String("url.domain", "epr.elastic.co"),
				zap.String("url.path", "/search"),
			},
		},
		{
			title: "IPv4 address",
			handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Write([]byte("Hello!"))
			}),
			method:          "GET",
			url:             "http://233.252.0.10:8080/search?package=foo&all=true",
			remoteAddress:   "233.252.0.252:33442",
			expectedMessage: "GET /search HTTP/1.1",
			expectedFields: []zap.Field{
				zap.Int64("http.response.code", 200),
				zap.Int("http.response.body.bytes", 6),
				zap.String("http.request.method", "GET"),
				zap.String("source.address", "233.252.0.252"),
				zap.String("source.ip", "233.252.0.252"),
				zap.String("url.domain", "233.252.0.10"),
				zap.Int("url.port", 8080),
				zap.String("url.path", "/search"),
				zap.String("url.query", "package=foo&all=true"),
			},
		},
		{
			title: "IPv6 address",
			handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Write([]byte("Hello!"))
			}),
			method:          "GET",
			url:             "http://[2001:0DB8::10]/search?package=foo&all=true",
			remoteAddress:   "[2001:0DB8::CAFE]:33442",
			expectedMessage: "GET /search HTTP/1.1",
			expectedFields: []zap.Field{
				zap.Int64("http.response.code", 200),
				zap.Int("http.response.body.bytes", 6),
				zap.String("http.request.method", "GET"),
				zap.String("source.address", "2001:0DB8::CAFE"),
				zap.String("source.ip", "2001:0DB8::CAFE"),
				zap.String("url.domain", "[2001:0DB8::10]"),
				zap.String("url.path", "/search"),
				zap.String("url.query", "package=foo&all=true"),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			req, err := http.NewRequest(c.method, c.url, nil)
			require.NoError(t, err)
			req.RemoteAddr = c.remoteAddress

			recorder := httptest.NewRecorder()
			message, fields := captureZapFieldsForRequest(c.handler, recorder, req)

			// Check only that event.duration is there, and drop it.
			durationFound := false
			for i, f := range fields {
				if f.Key == "event.duration" {
					fields = append(fields[:i], fields[i+1:]...)
					durationFound = true
				}
			}
			assert.True(t, durationFound, "event.duration expected")

			// Check message.
			assert.Equal(t, c.expectedMessage, message)

			// Check fields.
			for _, expected := range c.expectedFields {
				found := false
				for _, field := range fields {
					if field.Key == expected.Key {
						assert.Equal(t, expected, field, "field "+expected.Key)
						found = true
						break
					}
				}
				assert.True(t, found, expected.Key+" not found")
			}
			for _, field := range fields {
				found := false
				for _, expected := range c.expectedFields {
					if field.Key == expected.Key {
						found = true
						break
					}
				}
				assert.True(t, found, field.Key+" not expected")
			}
		})
	}
}
