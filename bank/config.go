package bank

import (
	"goblin2/internal/config"
	"path/filepath"
)

var (
	cfg *Config
)

// Config represents the configuration for the bank.
type Config struct {
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

	return nil
}
