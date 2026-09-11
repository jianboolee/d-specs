package logger

import (
	"log/slog"
	"os"
)

func New(env string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if env == "development" {
		opts.Level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
