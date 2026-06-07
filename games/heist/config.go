package heist

import (
	"fmt"
	"goblin2/internal/cache"
	"goblin2/internal/config"
	"goblin2/internal/discordid"
	"log/slog"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	heistConfigCacheTTL             = 30 * time.Minute
	heistConfigCacheCleanupInterval = 5 * time.Minute
)

type configCacheKey struct {
	guildID discordid.SnowflakeID
}

var (
	defaultConfig Config
	configCache   = cache.New[configCacheKey, Config](heistConfigCacheTTL, heistConfigCacheCleanupInterval)
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
func GetConfig(guildID discordid.SnowflakeID) *Config {
	key := configCacheKey{
		guildID: guildID,
	}

	if cfg, ok := configCache.Get(key); ok {
		copied := copyConfig(&cfg)
		copied.Theme = GetTheme(guildID)
		copied.Targets = GetTargets(guildID)
		return copied
	}

	cfg := readConfig(key.guildID)
	if cfg == nil {
		cfg = createNewConfig(guildID)
		writeConfig(cfg)
	}

	configCache.Set(key, *cfg)

	cfg = copyConfig(cfg)
	cfg.Theme = GetTheme(guildID)
	cfg.Targets = GetTargets(guildID)

	return cfg
}

// createNewConfig creates a new heist configuration with default values for the specified guild.
func createNewConfig(guildID discordid.SnowflakeID) *Config {
	c := copyConfig(&defaultConfig)
	c.GuildID = guildID

	slog.Info("Created default heist config",
		slog.Any("guild_id", c.GuildID),
	)

	return c
}

// copyConfig creates a deep copy of the heist configuration.
func copyConfig(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}

	copied := new(*cfg)
	if cfg.Targets != nil {
		copied.Targets = copyTargets(cfg.Targets)
	}
	if cfg.Theme != nil {
		copied.Theme = copyTheme(cfg.Theme)
	}

	return copied
}

// CloseConfigCache stops the config cache cleanup goroutine and clears cached config entries.
func CloseConfigCache() {
	configCache.Destroy()
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
