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
	prometheus.MustRegister(serviceInfo)

	prometheus.MustRegister(httpInFlightRequests)
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDurationSeconds)
	prometheus.MustRegister(httpRequestSizeBytes)
	prometheus.MustRegister(httpResponseSizeBytes)

	prometheus.MustRegister(SearchProcessDurationSeconds)
	prometheus.MustRegister(NumberIndexedPackages)
	prometheus.MustRegister(CursorUpdatesTotal)
	prometheus.MustRegister(StorageRequestsTotal)

	return func(next http.Handler) http.Handler {
		handler := next

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			route := mux.CurrentRoute(req)
			path, _ := route.GetPathTemplate()
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
