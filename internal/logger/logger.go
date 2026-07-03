// Package logger builds a configured *zap.Logger. It exposes no package-level
// global: callers construct a logger and inject it where needed (DI-friendly).
package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config controls how the logger is built. It is populated from the
// application configuration (see internal/config).
type Config struct {
	Level  string // "debug", "info", "warn", "error"
	Format string // "json" or "console"
}

// New constructs a production-grade structured logger from cfg.
func New(cfg Config) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("logger: invalid level %q: %w", cfg.Level, err)
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encCfg.EncodeLevel = zapcore.LowercaseLevelEncoder

	var encoder zapcore.Encoder
	switch cfg.Format {
	case "console":
		encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encCfg)
	case "json", "":
		encoder = zapcore.NewJSONEncoder(encCfg)
	default:
		return nil, fmt.Errorf("logger: invalid format %q", cfg.Format)
	}

	core := zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), level)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}

// Nop returns a no-op logger, handy for tests.
func Nop() *zap.Logger { return zap.NewNop() }
