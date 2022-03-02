// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"flag"
	"net"
	"net/http"
	"os"
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

// InitLogger initializes the logger, this is ignored afer a logger has
// been created for this process.
func InitLogger(loggerType string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	if logger != nil {
		return
	}

	if loggerType == "" {
		loggerType = *logType
	}

	switch loggerType {
	case ECSLogger:
		logger = newECSLogger()
	case DevLogger:
		logger = newDevelopmentLogger()
	default:
		logger = newECSLogger()
		logger.Warn("unknown log type " + loggerType + " using default")
	}
}

// Logger returns a logger singleton.
func Logger() *zap.Logger {
	InitLogger("")
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
			// Do not log requests to the health endpoint
			if r.RequestURI == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			LogRequest(logger, next, w, r)
		})
	}
}

// LogRequest captures information from a handler handling a request, and generates logs
// using this information.
func LogRequest(logger *zap.Logger, handler http.Handler, w http.ResponseWriter, req *http.Request) {
	resp := httpsnoop.CaptureMetrics(handler, w, req)

	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	fields := []zap.Field{
		zap.String("source.address", host),
		zap.String("http.request.method", req.Method),

		zap.Int("http.response.code", resp.Code),
		zap.Int64("http.response.body.bytes", resp.Written),
	}
	if ip := net.ParseIP(host); ip != nil {
		fields = append(fields, zap.String("source.ip", host))
	} else {
		fields = append(fields, zap.String("source.domain", host))
	}
	if referer := req.Referer(); referer != "" {
		fields = append(fields, zap.String("http.request.referer", referer))
	}
	if userAgent := req.UserAgent(); userAgent != "" {
		fields = append(fields, zap.String("user_agent.original", userAgent))
	}
	if user := req.URL.User; user != nil {
		if username := user.Username(); username != "" {
			fields = append(fields, zap.String("user.name", username))
		}
	}

	message := req.Method + " " + req.URL.Path + " " + req.Proto
	logger.Info(message, fields...)
}
