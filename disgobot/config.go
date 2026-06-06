package disgobot

import "github.com/disgoorg/snowflake/v2"

// Config contains the bot configuration.
type Config struct {
	DevGuilds []snowflake.ID `yaml:"dev_guilds"`
	Token     string         `yaml:"token"`
}
