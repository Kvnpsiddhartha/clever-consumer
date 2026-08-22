package logging

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New() (*zap.SugaredLogger, func() error, error) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	if level := strings.TrimSpace(os.Getenv("LOG_LEVEL")); level != "" {
		var parsed zapcore.Level
		if err := parsed.Set(level); err != nil {
			return nil, nil, err
		}
		cfg.Level = zap.NewAtomicLevelAt(parsed)
	}

	logger, err := cfg.Build()
	if err != nil {
		return nil, nil, err
	}
	return logger.Sugar(), logger.Sync, nil
}

func Nop() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}
