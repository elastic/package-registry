package proxymode

import (
	"github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
)

type zapLoggerAdapter struct {
	target *zap.Logger
}

var _ retryablehttp.LeveledLogger = new(zapLoggerAdapter)

func withZapLoggerAdapter(target *zap.Logger) retryablehttp.LeveledLogger {
	return &zapLoggerAdapter{
		target: target,
	}
}

func (a zapLoggerAdapter) Error(msg string, keysAndValues ...interface{}) {
	a.target.Error(msg, keysAndValuesAsZapFields(keysAndValues...)...)
}

func (a zapLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	a.target.Info(msg, keysAndValuesAsZapFields(keysAndValues...)...)
}

func (a zapLoggerAdapter) Debug(msg string, keysAndValues ...interface{}) {
	a.target.Debug(msg, keysAndValuesAsZapFields(keysAndValues...)...)
}

func (a zapLoggerAdapter) Warn(msg string, keysAndValues ...interface{}) {
	a.target.Warn(msg, keysAndValuesAsZapFields(keysAndValues...)...)
}

// keysAndValuesAsZapFields function transforms the LeveledLogger arguments to the zap.Logger interface.
func keysAndValuesAsZapFields(keysAndValues ...interface{}) []zap.Field {
	fields := make([]zap.Field, len(keysAndValues)/2)
	var j int
	for i := 0; i < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue // something is wrong with the key, string expected
		}
		fields[j] = zap.Any(key, keysAndValues[i+1])
		j++
	}
	return fields
}
