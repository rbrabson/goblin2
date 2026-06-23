package blackjack

import (
	"goblin2/internal/config"
	"goblin2/internal/discordid"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	defaultConfig Config
)

// Config holds the configuration settings for the blackjack game.
type Config struct {
	ID                bson.ObjectID         `bson:"_id,omitempty" yaml:"-"`
	GuildID           discordid.SnowflakeID `bson:"guild_id" yaml:"guild_id"`
	MaxPlayers        int                   `bson:"max_players" yaml:"max_players"`
	Decks             int                   `bson:"decks" yaml:"decks"`
	BetAmount         int                   `bson:"bet_amount" yaml:"bet_amount"`
	DelayBetweenGames time.Duration         `bson:"delay_between_games" yaml:"delay_between_games"`
	WaitForPlayers    time.Duration         `bson:"wait_for_players" yaml:"wait_for_players"`
	PlayerTimeout     time.Duration         `bson:"player_timeout" yaml:"player_timeout"`
	ShowPlayerTurn    time.Duration         `bson:"show_player_turn" yaml:"show_player_turn"`
	ShowDealerTurn    time.Duration         `bson:"show_dealer_turn" yaml:"show_dealer_turn"`
	PayoutPercent     int                   `bson:"payout_percent" yaml:"payout_percent"`
	SinglePlayerMode  bool                  `bson:"single_player_mode" yaml:"single_player_mode"`
}

// GetConfig retrieves the blackjack configuration, either from the cache, database, or by creating a new, default configuration.
// The value returned is guaranteed to be non-nil.
func GetConfig(guildID discordid.SnowflakeID) *Config {
	key := configCacheKey{
		guildID: guildID,
	}

	if cfg, ok := configCache.Get(key); ok {
		applyLegacyConfigDefaults(&cfg)
		return runtimeConfig(&cfg)
	}

	cfg := readConfig(key.guildID)
	if cfg == nil {
		cfg = createNewConfig(guildID)
		cfg.GuildID = guildID
		writeConfig(cfg)
	} else {
		applyLegacyConfigDefaults(cfg)
	}

	configCache.Set(key, *cfg)
	return runtimeConfig(cfg)
}

// applyLegacyConfigDefaults fills in values that older persisted configs may not have stored.
func applyLegacyConfigDefaults(cfg *Config) {
	if cfg == nil {
		return
	}

	if cfg.WaitForPlayers == 0 && !cfg.SinglePlayerMode {
		cfg.WaitForPlayers = defaultConfig.WaitForPlayers
	}
}

// runtimeConfig returns a copy of the persisted config with runtime-only adjustments applied.
func runtimeConfig(cfg *Config) *Config {
	cfgCopy := copyConfig(cfg)

	if cfgCopy.SinglePlayerMode {
		cfgCopy.MaxPlayers = 1
		cfgCopy.WaitForPlayers = 0
		cfgCopy.ShowPlayerTurn = 0
		cfgCopy.ShowDealerTurn = 0
	}

	return cfgCopy
}

// createNewConfig creates a new configuration with default values.
func createNewConfig(guildID discordid.SnowflakeID) *Config {
	return &Config{
		GuildID:           guildID,
		MaxPlayers:        defaultConfig.MaxPlayers,
		Decks:             defaultConfig.Decks,
		BetAmount:         defaultConfig.BetAmount,
		DelayBetweenGames: defaultConfig.DelayBetweenGames,
		WaitForPlayers:    defaultConfig.WaitForPlayers,
		PlayerTimeout:     defaultConfig.PlayerTimeout,
		ShowPlayerTurn:    defaultConfig.ShowPlayerTurn,
		ShowDealerTurn:    defaultConfig.ShowDealerTurn,
		PayoutPercent:     defaultConfig.PayoutPercent,
		SinglePlayerMode:  defaultConfig.SinglePlayerMode,
	}
}

// LoadConfig loads the configuration from the specified YAML file path.
func LoadConfig(path string) error {
	filePath := filepath.Join(path, "blackjack/config.yaml")
	if err := config.LoadConfig(filePath, &defaultConfig); err != nil {
		return err
	}

	return nil
}
