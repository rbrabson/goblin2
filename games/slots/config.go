package slots

import (
	"goblin2/config"
	"path/filepath"
	"time"
)

var (
	defaultConfig Config
)

// Config represents the configuration for the slots game.
type Config struct {
	Cooldown time.Duration `json:"cooldown"`
}

// GetConfig retrieves the configuration for the slots game.
func GetConfig() *Config {
	return createNewConfig()
}

// createNewConfig reads the configuration from a JSON file and returns a Config instance.
func createNewConfig() *Config {
	return &Config{
		Cooldown: defaultConfig.Cooldown,
	}
}

// LoadConfig loads the configuration from a YAML file.
func LoadConfig(path string) error {
	filePath := filepath.Join(path, "slots/config.yaml")
	if err := config.LoadConfig(filePath, &defaultConfig); err != nil {
		return err
	}

	return nil
}
