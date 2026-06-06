package log

import "log/slog"

// Config contains the log configuration.
type Config struct {
	Level     slog.Level `yaml:"level"`
	Format    string     `yaml:"format"`
	AddSource bool       `yaml:"add_source"`
}
