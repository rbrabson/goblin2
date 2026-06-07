package disgobot

import (
	"goblin2/internal/config"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/disgoorg/snowflake/v2"
)

// Config contains the bot configuration.
type Config struct {
	DevGuilds []snowflake.ID `yaml:"dev_guilds"`
	Token     string         `yaml:"token"`
}

// LoadConfig loads the bot configuration from the specified path.
func LoadConfig(cfgPath string) (*Config, error) {
	cfg := Config{}
	botPath := filepath.Join(cfgPath, "bot/config.yaml")
	if err := config.LoadConfig(botPath, &cfg); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(-1)
	}
	return &cfg, nil
}
