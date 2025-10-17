// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package proxymode

import (
	"context"
	"errors"

	"github.com/hashicorp/go-retryablehttp"
	"go.elastic.co/apm/module/apmzap/v2"
	"go.uber.org/zap"
)

type zapLoggerAdapter struct {
	target *zap.Logger
	ctx    context.Context
}

var _ retryablehttp.LeveledLogger = new(zapLoggerAdapter)

func newZapLoggerAdapter(ctx context.Context, target *zap.Logger) retryablehttp.LeveledLogger {
	return &zapLoggerAdapter{
		target: target,
		ctx:    ctx,
	}
}

func (a zapLoggerAdapter) Error(msg string, keysAndValues ...interface{}) {
	// Check if the error is an expected context cancellation.
	isCanceled := false
	for i := 0; i < len(keysAndValues); i += 2 {
		if key, ok := keysAndValues[i].(string); ok && key == "error" {
			if err, ok := keysAndValues[i+1].(error); ok && errors.Is(err, context.Canceled) {
				isCanceled = true
				break
			}
		}
	}

	loggerWithContext := a.target.With(apmzap.TraceContext(a.ctx)...)

	// If the error was an expected cancellation, log it as a debug message.
	if isCanceled {
		loggerWithContext.Debug(msg, keysAndValuesAsZapFields(keysAndValues...)...)
		return
	}

	loggerWithContext.Error(msg, keysAndValuesAsZapFields(keysAndValues...)...)
}

func (a zapLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	loggerWithContext := a.target.With(apmzap.TraceContext(a.ctx)...)
	loggerWithContext.Info(msg, keysAndValuesAsZapFields(keysAndValues...)...)
}

func (a zapLoggerAdapter) Debug(msg string, keysAndValues ...interface{}) {
	loggerWithContext := a.target.With(apmzap.TraceContext(a.ctx)...)
	loggerWithContext.Debug(msg, keysAndValuesAsZapFields(keysAndValues...)...)
}

func (a zapLoggerAdapter) Warn(msg string, keysAndValues ...interface{}) {
	loggerWithContext := a.target.With(apmzap.TraceContext(a.ctx)...)
	loggerWithContext.Warn(msg, keysAndValuesAsZapFields(keysAndValues...)...)
}

func keysAndValuesAsZapFields(keysAndValues ...interface{}) []zap.Field {
	fields := make([]zap.Field, len(keysAndValues)/2)
	var j int
	for i := 0; i < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		fields[j] = zap.Any(key, keysAndValues[i+1])
		j++
	}
	return fields
}
