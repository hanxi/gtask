package log

import (
	"log/slog"
	"os"
	"strings"
)

var Debug func(msg string, ars ...any)
var Info func(msg string, ars ...any)
var Warn func(msg string, ars ...any)
var Error func(msg string, ars ...any)

// parseLevel parses a level string into a logger Level value.
func parseLevel(s string) slog.Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	}
	return slog.LevelInfo
}

func SetLevel(s string) {
	opts := slog.HandlerOptions{
		AddSource: true,
		Level:     parseLevel(s),
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &opts))
	slog.SetDefault(logger)
}

func init() {
	Debug = slog.Debug
	Info = slog.Info
	Warn = slog.Warn
	Error = slog.Error
}
