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
	ID                bson.ObjectID         `json:"id,omitempty" bson:"_id,omitempty"`
	GuildID           discordid.SnowflakeID `json:"guild_id" bson:"guild_id"`
	MaxPlayers        int                   `json:"max_players" bson:"max_players"`
	Decks             int                   `json:"decks" bson:"decks"`
	BetAmount         int                   `json:"bet_amount" bson:"bet_amount"`
	DelayBetweenGames time.Duration         `json:"delay_between_games" bson:"delay_between_games"`
	WaitForPlayers    time.Duration         `json:"wait_for_players" bson:"wait_for_players"`
	PlayerTimeout     time.Duration         `json:"player_timeout" bson:"player_timeout"`
	ShowPlayerTurn    time.Duration         `json:"show_player_turn" bson:"show_player_turn"`
	ShowDealerTurn    time.Duration         `json:"show_dealer_turn" bson:"show_dealer_turn"`
	PayoutPercent     int                   `json:"payout_percent" bson:"payout_percent"`
	SinglePlayerMode  bool                  `json:"single_player_mode" bson:"single_player_mode"`
}

// GetConfig retrieves the blackjack configuration, either from the cache, database, or by creating a new, default configuration.
// The value returned is guaranteed to be non-nil.
func GetConfig(guildID discordid.SnowflakeID) *Config {
	key := configCacheKey{
		guildID: guildID,
	}

	if cfg, ok := configCache.Get(key); ok {
		return runtimeConfig(&cfg)
	}

	cfg := readConfig(key.guildID)
	if cfg == nil {
		cfg = createNewConfig(guildID)
		cfg.GuildID = guildID
		writeConfig(cfg)
	}

	configCache.Set(key, *cfg)
	return runtimeConfig(cfg)
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
