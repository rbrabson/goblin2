package bank

import (
	"fmt"
	"goblin2/internal/config"
	"path/filepath"
)

var (
	cfg   Config
	theme *Theme
)

// Config represents the configuration for the bank.
type Config struct {
	DefaultTheme string            `yaml:"default_theme"`
	Themes       map[string]*Theme `yaml:"themes"`
}

// Theme represents the theme configuration for the bank.
type Theme struct {
	BankName       string `yaml:"bank_name"`
	Currency       string `yaml:"currency"`
	DefaultBalance int    `yaml:"default_balance"`
}

// LoadConfig loads the configuration from the specified YAML file path.
func LoadConfig(path string) error {
	var cfg Config
	filePath := filepath.Join(path, "bank/config.yaml")
	if err := config.LoadConfig(filePath, &cfg); err != nil {
		return err
	}
	var ok bool
	if theme, ok = cfg.Themes[cfg.DefaultTheme]; !ok {
		return fmt.Errorf("default theme '%s' not found in configuration", cfg.DefaultTheme)
	}

	return nil
}
