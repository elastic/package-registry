// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"flag"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/mux"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Types of available loggers.
const (
	ECSLogger = "ecs"
	DevLogger = "dev"

	defaultLoggerType = ECSLogger
)

var logLevel = zap.LevelFlag("log-level", zap.InfoLevel, "log level (default \"info\")")
var logType = flag.String("log-type", defaultLoggerType, "log type (ecs, dev)")

var logger *zap.Logger
var loggerMutex sync.Mutex

// UseECSLogger initializes the logger as an JSON ECS logger. It does nothing
// if the logger has been already initialized.
func UseECSLogger() {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	if logger != nil {
		return
	}

	logger = newECSLogger()
}

// UseDevelopmentLogger initializes the logger as a development logger. It does nothing
// if the logger has been already initialized.
func UseDevelopmentLogger() {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	if logger != nil {
		return
	}

	logger = newDevelopmentLogger()
}

var warnInvalidTypeOnce sync.Once

// Logger returns a logger singleton.
func Logger() *zap.Logger {
	switch *logType {
	case ECSLogger:
		UseECSLogger()
	case DevLogger:
		UseDevelopmentLogger()
	default:
		logger = newECSLogger()
		warnInvalidTypeOnce.Do(func() {
			logger.Warn("unknown log type " + *logType + " using default")
		})
	}
	return logger
}

func newECSLogger() *zap.Logger {
	encoderConfig := ecszap.NewDefaultEncoderConfig()
	core := ecszap.NewCore(encoderConfig, os.Stderr, *logLevel)
	return zap.New(core, zap.AddCaller())
}

func newDevelopmentLogger() *zap.Logger {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, os.Stderr, zap.DebugLevel)
	return zap.New(core, zap.AddCaller())
}

// LoggingMiddleware is a middleware used to log requests to the given logger.
func LoggingMiddleware(logger *zap.Logger) mux.MiddlewareFunc {
	// Disable logging of the file and number of the caller, because it will be the
	// one of the helper.
	logger = logger.Named("http").WithOptions(zap.WithCaller(false))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.RequestURI {
			case "/health", "/metrics":
				// Do not log requests to these endpoints
				next.ServeHTTP(w, r)
			default:
				logRequest(logger, next, w, r)
			}
		})
	}
}

// logRequest captures information from a handler handling a request, and generates logs
// using this information.
func logRequest(logger *zap.Logger, handler http.Handler, w http.ResponseWriter, req *http.Request) {
	message, fields := captureZapFieldsForRequest(handler, w, req)
	logger.Info(message, fields...)
}

// captureZapFieldsForRequest handles a request and captures fields for zap logger.
func captureZapFieldsForRequest(handler http.Handler, w http.ResponseWriter, req *http.Request) (string, []zap.Field) {
	resp := httpsnoop.CaptureMetrics(handler, w, req)

	domain, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		domain = req.Host
	}
	if ip := net.ParseIP(domain); ip != nil && ip.To16() != nil && ip.To4() == nil {
		// For ECS, if the host part of an url is an IPv6, it must keep the brackets
		// when stored in `url.domain` (but not when stored in ip fields).
		domain = "[" + domain + "]"
	}
	sourceHost, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		sourceHost = req.RemoteAddr
	}
	fields := []zap.Field{
		// Request fields.
		zap.String("source.address", sourceHost),
		zap.String("http.request.method", req.Method),
		zap.String("url.path", req.URL.Path),
		zap.String("url.domain", domain),

		// Response fields.
		zap.Int("http.response.code", resp.Code),
		zap.Int64("http.response.body.bytes", resp.Written),
		zap.Int64("event.duration", resp.Duration.Nanoseconds()),
	}

	// Fields that are not always available.
	if ip := net.ParseIP(sourceHost); ip != nil {
		fields = append(fields, zap.String("source.ip", sourceHost))
	} else {
		fields = append(fields, zap.String("source.domain", sourceHost))
	}
	if referer := req.Referer(); referer != "" {
		fields = append(fields, zap.String("http.request.referer", referer))
	}
	if userAgent := req.UserAgent(); userAgent != "" {
		fields = append(fields, zap.String("user_agent.original", userAgent))
	}
	if query := req.URL.RawQuery; query != "" {
		fields = append(fields, zap.String("url.query", query))
	}
	if port != "" {
		if intPort, err := strconv.Atoi(port); err == nil && intPort != 0 {
			fields = append(fields, zap.Int("url.port", intPort))
		}
	}

	message := req.Method + " " + req.URL.Path + " " + req.Proto
	return message, fields
}
