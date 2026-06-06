package log

import (
	"log/slog"
	"os"
)

// Initialize initializes the logger with the provided configuration.
func Initialize(cfg Config) {
	opts := &slog.HandlerOptions{
		AddSource: cfg.AddSource,
		Level:     cfg.Level,
	}

	var sHandler slog.Handler
	switch cfg.Format {
	case "json":
		sHandler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		sHandler = slog.NewTextHandler(os.Stdout, opts)
	default:
		slog.Error("unknown log format", slog.String("format", cfg.Format))
		os.Exit(-1)
	}
	slog.SetDefault(slog.New(sHandler))
}

func Parse(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		slog.Error("unknown log level", slog.String("level", level))
		return slog.LevelInfo
	}
}

// SetLevel sets the log level.
func SetLevel(level slog.Level) {
	slog.SetLogLoggerLevel(level)
}
