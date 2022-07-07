// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const metricsNamespace = "epr"

var ServiceInfo = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "service_info",
		Help:      "Version information about this binary.",
	},
	[]string{"version", "instance"},
)

var (
	NumberIndexedPackages = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "number_indexed_packages",
		Help:      "A gauge for number of indexed packages.",
	})

	StorageRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "storage_requests_total",
			Help:      "A counter for requests performed to the storage.",
		},
		[]string{"location", "component"},
	)

	StorageIndexerUpdateIndexSuccessTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "storage_indexer_update_index_success_total",
			Help:      "A counter for updates of the cursor.",
		},
	)

	StorageIndexerUpdateIndexErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "storage_indexer_update_index_error_total",
			Help:      "A counter for all the update index processes that finished with error.",
		},
	)

	StorageIndexerUpdateIndexDurationSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "storage_indexer_update_index_duration_seconds",
			Help:      "A histogram of latencies for update index processes run by the indexer.",
		},
	)

	StorageIndexerGetDurationSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "storage_indexer_get_duration_seconds",
			Help:      "A histogram of latencies for get processes run by the indexer.",
		},
	)
)

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
				1024,       /* 1KiB */
				64 * 1024,  /* 64KiB */
				256 * 1024, /* 256KiB */
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
				16,
				32,
				64,
				128,
				256,
				512,
				1024,       /* 1KiB */
				64 * 1024,  /* 64KiB */
				256 * 1024, /* 256KiB */
			},
		},
		[]string{"code", "method", "path"},
	)
)
