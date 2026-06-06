package heist

import (
	"fmt"
	"goblin2/config"
	"goblin2/discordid"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	defaultConfig Config
)

// Config is the configuration data for new heists.
type Config struct {
	ID                 bson.ObjectID         `bson:"_id,omitempty"`
	GuildID            discordid.SnowflakeID `bson:"guild_id"`
	BailBase           int                   `bson:"bail_base"`
	BoostPercentage    float64               `bson:"boost_percentage"`
	BoostEnabled       bool                  `bson:"boost_enabled"`
	CrewOutput         string                `bson:"crew_output"`
	DeathTimer         time.Duration         `bson:"death_timer"`
	HeistCost          int                   `bson:"heist_cost"`
	PoliceAlert        time.Duration         `bson:"police_alert"`
	SentenceBase       time.Duration         `bson:"sentence_base"`
	BaseVaultRecovery  float64               `bson:"base_vault_recovery"`
	BoostVaultRecovery float64               `json:"boost_vault_recovery"`
	WaitTime           time.Duration         `bson:"wait_time"`
	Theme              *Theme                `bson:"-"`
	Targets            []*Target             `bson:"-"`
}

// LoadConfig loads the configuration from the specified YAML file path.
func LoadConfig(path string) error {
	filePath := filepath.Join(path, "heist/config.yaml")
	if err := config.LoadConfig(filePath, &defaultConfig); err != nil {
		return err
	}

	return nil
}

// GetConfig retrieves the heist configuration for the specified guild.
func GetConfig(guildID snowflake.ID) *Config {
	cfg := readConfig(discordid.SnowflakeID(guildID))
	if cfg == nil {
		cfg = createNewConfig(guildID)
	}
	writeConfig(cfg)

	cfg.Theme = GetTheme(guildID)
	cfg.Targets = GetTargets(guildID)

	return cfg
}

func createNewConfig(guildID snowflake.ID) *Config {
	c := &Config{
		GuildID:            discordid.NewSnowflakeID(guildID),
		BailBase:           defaultConfig.BailBase,
		BoostPercentage:    defaultConfig.BoostPercentage,
		BoostEnabled:       defaultConfig.BoostEnabled,
		CrewOutput:         defaultConfig.CrewOutput,
		DeathTimer:         defaultConfig.DeathTimer,
		HeistCost:          defaultConfig.HeistCost,
		PoliceAlert:        defaultConfig.PoliceAlert,
		SentenceBase:       defaultConfig.SentenceBase,
		BaseVaultRecovery:  defaultConfig.BaseVaultRecovery,
		BoostVaultRecovery: defaultConfig.BoostVaultRecovery,
		WaitTime:           defaultConfig.WaitTime,
	}

	slog.Info("Created default heist config",
		slog.Any("guild_id", c.GuildID),
	)

	return c
}

func defaultOrInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func defaultOrFloat(value float64, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}

func defaultOrDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

// String returns a string representation of the heist configuration.
func (c *Config) String() string {
	if c == nil {
		return "Config<nil>"
	}

	return fmt.Sprintf(
		"Config{GuildID: %s, BailBase: %d, BoostPercentage: %.2f, BoostEnabled: %t, CrewOutput: %q, DeathTimer: %s, HeistCost: %d, PoliceAlert: %s, SentenceBase: %s, BaseVaultRecovery: %.2f, BoostVaultRecovery: %.2f, WaitTime: %s}",
		c.GuildID,
		c.BailBase,
		c.BoostPercentage,
		c.BoostEnabled,
		c.CrewOutput,
		c.DeathTimer,
		c.HeistCost,
		c.PoliceAlert,
		c.SentenceBase,
		c.BaseVaultRecovery,
		c.BoostVaultRecovery,
		c.WaitTime,
	)
}
