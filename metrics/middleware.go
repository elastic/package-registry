// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package metrics

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsMiddleware is a middleware used to measure every request received
func MetricsMiddleware() mux.MiddlewareFunc {
	// Rergister all metrics
	prometheus.MustRegister(ServiceInfo)

	prometheus.MustRegister(httpInFlightRequests)
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDurationSeconds)
	prometheus.MustRegister(httpRequestSizeBytes)
	prometheus.MustRegister(httpResponseSizeBytes)

	prometheus.MustRegister(NumberIndexedPackages)
	prometheus.MustRegister(StorageRequestsTotal)
	prometheus.MustRegister(StorageIndexerGetDurationSeconds)
	prometheus.MustRegister(StorageIndexerUpdateIndexDurationSeconds)
	prometheus.MustRegister(StorageIndexerUpdateIndexSuccessTotal)
	prometheus.MustRegister(StorageIndexerUpdateIndexErrorsTotal)

	return func(next http.Handler) http.Handler {
		handler := next

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			route := mux.CurrentRoute(req)
			path, err := route.GetPathTemplate()
			if err != nil {
				path = "unknown"
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
