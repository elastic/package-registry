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
	body := "Hello!"
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(body))
	})

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://epr.elastic.co/search?package=foo&all=true", nil)
	req.RemoteAddr = "233.252.0.252:33442"
	require.NoError(t, err)

	message, fields := captureZapFieldsForRequest(handler, recorder, req)
	var duration int64
	for _, f := range fields {
		if f.Key == "event.duration" {
			duration = f.Integer
		}
	}

	expectedMessage := "GET /search HTTP/1.1"
	assert.Equal(t, expectedMessage, message)

	expectedFields := []zap.Field{
		zap.Int64("event.duration", duration),
		zap.Int64("http.response.code", 200),
		zap.Int("http.response.body.bytes", len(body)),
		zap.String("http.request.method", "GET"),
		zap.String("source.address", "233.252.0.252"),
		zap.String("source.ip", "233.252.0.252"),
		zap.String("url.domain", "epr.elastic.co"),
		zap.String("url.path", "/search"),
		zap.String("url.query", "package=foo&all=true"),
	}
	for _, expected := range expectedFields {
		assert.Contains(t, fields, expected)
	}
	assert.Len(t, fields, len(expectedFields))
}
