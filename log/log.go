package log

import (
	"log/slog"
	"os"
)

var logger *slog.Logger
var Debug func(msg string, ars ...any)
var Info func(msg string, ars ...any)
var Warn func(msg string, ars ...any)
var Error func(msg string, ars ...any)

func init() {
	opts := slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}
	logger = slog.New(slog.NewTextHandler(os.Stdout, &opts))
	Debug = logger.Debug
	Info = logger.Info
	Warn = logger.Warn
	Error = logger.Error
}
