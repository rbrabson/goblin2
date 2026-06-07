package message

import (
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var defaultConfig = Config{
	ButtonsConfig: ButtonsConfig{
		First: &ComponentOption{
			Emoji: &discord.ComponentEmoji{Name: "⏮️"},
			Style: discord.ButtonStylePrimary,
		},
		Back: &ComponentOption{
			Emoji: &discord.ComponentEmoji{Name: "◀️"},
			Style: discord.ButtonStylePrimary,
		},
		Next: &ComponentOption{
			Emoji: &discord.ComponentEmoji{Name: "▶️"},
			Style: discord.ButtonStylePrimary,
		},
		Last: &ComponentOption{
			Emoji: &discord.ComponentEmoji{Name: "⏭️"},
			Style: discord.ButtonStylePrimary,
		},
	},
	CustomIDPrefix: "paginator",
	EmbedColor:     0x4c50c1,
	ItemsPerPage:   5,
	IdleWait:       time.Minute * 5,
}

type Config struct {
	ButtonsConfig  ButtonsConfig
	CustomIDPrefix string
	EmbedColor     int
	ItemsPerPage   int
	DiscordConfig  DiscordConfig
	IdleWait       time.Duration
}

// ComponentOption holds the options for a single navigation button.
type ComponentOption struct {
	Emoji *discord.ComponentEmoji
	Label string
	Style discord.ButtonStyle
}

// ButtonsConfig is the configuration for the pagination buttons.
type ButtonsConfig struct {
	First *ComponentOption
	Back  *ComponentOption
	Stop  *ComponentOption
	Next  *ComponentOption
	Last  *ComponentOption
}

// DiscordConfig holds the disgo-specific wiring for the paginator.
type DiscordConfig struct {
	AddComponentHandler    func(key string, h handler.ComponentHandler)
	RemoveComponentHandler func(key string)

	// Client is used to create regular channel messages and to disable
	// paginated messages outside an interaction context.
	Client *bot.Client
}

func GetDefaultConfig() *Config {
	return &Config{
		ButtonsConfig:  defaultConfig.ButtonsConfig,
		CustomIDPrefix: defaultConfig.CustomIDPrefix,
		EmbedColor:     defaultConfig.EmbedColor,
		ItemsPerPage:   defaultConfig.ItemsPerPage,
		DiscordConfig:  defaultConfig.DiscordConfig,
		IdleWait:       defaultConfig.IdleWait,
	}
}

type ConfigOpt func(config *Config)

func (c *Config) Apply(opts []ConfigOpt) {
	for _, opt := range opts {
		opt(c)
	}
}

func WithButtonsConfig(buttonsConfig ButtonsConfig) ConfigOpt {
	return func(config *Config) { config.ButtonsConfig = buttonsConfig }
}

func WithCustomIDPrefix(prefix string) ConfigOpt {
	return func(config *Config) { config.CustomIDPrefix = prefix }
}

func WithEmbedColor(color int) ConfigOpt {
	return func(config *Config) { config.EmbedColor = color }
}

func WithDiscordConfig(discordConfig DiscordConfig) ConfigOpt {
	return func(config *Config) { config.DiscordConfig = discordConfig }
}

func WithItemsPerPage(itemsPerPage int) ConfigOpt {
	return func(config *Config) { config.ItemsPerPage = itemsPerPage }
}

func WithIdleWait(idleWait time.Duration) ConfigOpt {
	return func(config *Config) { config.IdleWait = idleWait }
}
