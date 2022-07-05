// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsNamespace = "epr"

// storage metrics
var (
	NumberIndexedPackages = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "number_indexed_packages",
		Help:      "A gauge for number of indexed packages",
	})
)

// common metrics for http requests
var (
	httpInFlightRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "in_flight_requests",
		Help:      "A gauge of requests currently being served by the http server.",
	})

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "http_requests_total",
			Help:      "A counter for requests to the http server.",
		},
		[]string{"code", "method", "path"},
	)

	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "http_request_duration_seconds",
			Help:      "A histogram of latencies for requests to the http server.",
		},
		[]string{"code", "method", "path"},
	)

	httpRequestSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "http_request_size_bytes",
			Help:      "A histogram of sizes of requests to the http server.",
			Buckets: []float64{
				16,
				32,
				64,
				128,
				256,
				512,
				1024,             /* 1KiB */
				64 * 1024,        /* 64KiB */
				256 * 1024,       /* 256KiB */
				1024 * 1024,      /* 1MiB */
				64 * 1024 * 1024, /* 64MiB */
			},
		},
		[]string{"code", "method", "path"},
	)

	httpResponseSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "http_response_size_bytes",
			Help:      "A histogram of response sizes for requests to the http server.",
			Buckets: []float64{
				10,
				64,
				256,
				512,
				1024,              /* 1KiB */
				64 * 1024,         /* 64KiB */
				256 * 1024,        /* 256KiB */
				512 * 1024,        /* 512KiB */
				1024 * 1024,       /* 1MiB */
				64 * 1024 * 1024,  /* 64MiB */
				512 * 1024 * 1024, /* 512MiB */
			},
		},
		[]string{"code", "method", "path"},
	)
)

// MetricsMiddleware is a middleware used to measure every request received
func MetricsMiddleware() mux.MiddlewareFunc {
	// Rergister all metrics
	prometheus.MustRegister(httpInFlightRequests)
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDurationSeconds)
	prometheus.MustRegister(httpRequestSizeBytes)
	prometheus.MustRegister(httpResponseSizeBytes)

	prometheus.MustRegister(NumberIndexedPackages)

	return func(next http.Handler) http.Handler {
		handler := next

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			route := mux.CurrentRoute(req)
			path, _ := route.GetPathTemplate()
			if req.RequestURI == "/metrics" {
				next.ServeHTTP(w, req)
				return
			}
			labels := prometheus.Labels{"path": path}
			handler = promhttp.InstrumentHandlerCounter(httpRequestsTotal.MustCurryWith(labels), handler)
			handler = promhttp.InstrumentHandlerDuration(httpRequestDurationSeconds.MustCurryWith(labels), handler)
			handler = promhttp.InstrumentHandlerInFlight(httpInFlightRequests, handler)
			handler = promhttp.InstrumentHandlerRequestSize(httpRequestSizeBytes.MustCurryWith(labels), handler)
			handler = promhttp.InstrumentHandlerResponseSize(httpResponseSizeBytes.MustCurryWith(labels), handler)
			handler.ServeHTTP(w, req)
		})
	}
}
