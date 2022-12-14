// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/mux"
	"go.elastic.co/apm/module/apmzap/v2"
	"go.elastic.co/apm/v2"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Types of available loggers.
const (
	ECSLogger = "ecs"
	DevLogger = "dev"

	DefaultLoggerType  = ECSLogger
	DefaultLoggerLevel = zapcore.InfoLevel
)

type LoggerOptions struct {
	Type      string
	Level     *zapcore.Level
	APMTracer *apm.Tracer
}

func NewLogger(options LoggerOptions) (*zap.Logger, error) {
	if options.Type == "" {
		options.Type = DefaultLoggerType
	}
	if options.Level == nil {
		level := DefaultLoggerLevel
		options.Level = &level
	}

	core, err := newLoggerCore(options)
	if err != nil {
		return nil, err
	}

	if options.APMTracer != nil {
		apmCore := apmzap.Core{
			Tracer: options.APMTracer,
		}
		core = apmCore.WrapCore(core)
	}

	return zap.New(core, zap.AddCaller()), nil
}

func NewTestLogger() *zap.Logger {
	level := zap.DebugLevel
	logger, err := NewLogger(LoggerOptions{
		Type:  DevLogger,
		Level: &level,
	})
	if err != nil {
		panic("failed to initialize logger")
	}
	return logger
}

func newLoggerCore(options LoggerOptions) (zapcore.Core, error) {
	switch options.Type {
	case ECSLogger:
		encoderConfig := ecszap.NewDefaultEncoderConfig()
		return ecszap.NewCore(encoderConfig, os.Stderr, *options.Level), nil
	case DevLogger:
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoder := zapcore.NewConsoleEncoder(encoderConfig)
		return zapcore.NewCore(encoder, os.Stderr, *options.Level), nil
	}

	return nil, fmt.Errorf("invalid logger type %q", options.Type)
}

// LoggingMiddleware is a middleware used to log requests to the given logger.
func LoggingMiddleware(logger *zap.Logger) mux.MiddlewareFunc {
	// Disable logging of the file and number of the caller, because it will be the
	// one of the helper.
	logger = logger.Named("http").WithOptions(zap.WithCaller(false))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.RequestURI {
			case "/health":
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

// LoggerAdapter adapts a zap logger so it can be used as logger of other features as APM.
type LoggerAdapter struct {
	*zap.Logger
}

// Debugf logs a message at debug level.
func (a *LoggerAdapter) Debugf(format string, args ...interface{}) {
	if a.Logger.Level() > zapcore.DebugLevel {
		return
	}
	a.Logger.Debug(fmt.Sprintf(format, args...))
}

// Errorf logs a message at error level.
func (a *LoggerAdapter) Errorf(format string, args ...interface{}) {
	if a.Logger.Level() > zapcore.ErrorLevel {
		return
	}
	a.Logger.Error(fmt.Sprintf(format, args...))
}

// Warningf logs a message at warning level.
func (a *LoggerAdapter) Warningf(format string, args ...interface{}) {
	if a.Logger.Level() > zapcore.WarnLevel {
		return
	}
	a.Logger.Warn(fmt.Sprintf(format, args...))
}
