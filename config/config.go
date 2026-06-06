package config

import (
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads the configuration from the specified yaml file path.
func LoadConfig(path string, cfg any) error {
	file, err := os.Open(path)
	if err != nil {
		return ErrFileNotFound(err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			slog.Error("failed to close yaml file", slog.Any("error", err))
		}
	}(file)

	if err = yaml.NewDecoder(file).Decode(cfg); err != nil {
		return ErrInvalidYaml(err)
	}

	return nil
}
