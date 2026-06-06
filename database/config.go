package database

import (
	"goblin2/config"
	"path/filepath"
)

// Config represents the configuration for the database.
type Config struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
}

// LoadConfig loads the configuration from the specified yaml file path.
func LoadConfig(path string) (*Config, error) {
	filePath := filepath.Join(path, "database/config.yaml")
	var cfg Config
	if err := config.LoadConfig(filePath, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
