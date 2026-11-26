package logger

import (
	"log/slog"
	"os"
)

func New(mode string) *slog.Logger {
	level := slog.LevelInfo
	if mode == "debug" {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
