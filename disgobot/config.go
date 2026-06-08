package disgobot

import (
	"github.com/disgoorg/snowflake/v2"
)

// Config contains the bot configuration.
type Config struct {
	DevGuilds []snowflake.ID `yaml:"dev_guilds"`
	Token     string         `yaml:"token"`
}

// LoadConfig loads the bot configuration from the specified path.
func LoadConfig(botToken string, devGuilds []snowflake.ID) (*Config, error) {
	return &Config{
		Token:     botToken,
		DevGuilds: devGuilds,
	}, nil
}
