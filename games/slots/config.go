package slots

import (
	"fmt"
	"goblin2/config"
	"path/filepath"
	"time"
)

var (
	defaultConfig Config
)

// Config represents the configuration for the slots game.
type Config struct {
	Cooldown time.Duration `yaml:"cooldown"`
	Symbols  string        `yaml:"symbols"`
}

type configFile struct {
	Cooldown string `yaml:"cooldown"`
	Symbols  string `yaml:"symbols"`
}

// GetConfig retrieves the configuration for the slots game.
func GetConfig() *Config {
	return createNewConfig()
}

// createNewConfig reads the configuration from a JSON file and returns a Config instance.
func createNewConfig() *Config {
	return &Config{
		Cooldown: defaultConfig.Cooldown,
		Symbols:  defaultConfig.Symbols,
	}
}

// LoadConfig loads the configuration from a YAML file.
func LoadConfig(path string) error {
	filePath := filepath.Join(path, "slots/config.yaml")

	var cfg configFile
	if err := config.LoadConfig(filePath, &cfg); err != nil {
		return err
	}

	cooldown, err := time.ParseDuration(cfg.Cooldown)
	if err != nil {
		return fmt.Errorf("invalid slots cooldown %q: %w", cfg.Cooldown, err)
	}

	defaultConfig = Config{
		Cooldown: cooldown,
		Symbols:  cfg.Symbols,
	}

	return nil
}
