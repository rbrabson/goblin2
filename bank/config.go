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

// LoadConfig loads the configuration from the specified YAML file path into the package-level
// cfg, which createNewAccount and the bank setup read from.
func LoadConfig(path string) error {
	var c Config
	filePath := filepath.Join(path, "bank/config.yaml")
	if err := config.LoadConfig(filePath, &c); err != nil {
		return err
	}

	cfg = &c
	return nil
}
