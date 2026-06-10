package race

import (
	"goblin2/internal/cache"
	"goblin2/internal/config"
	"goblin2/internal/discordid"
	"log/slog"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	raceConfigCacheTTL             = 30 * time.Minute
	raceConfigCacheCleanupInterval = 5 * time.Minute
)

type configCacheKey struct {
	guildID discordid.SnowflakeID
}

var (
	defaultConfig Config
	configCache   = cache.New[configCacheKey, Config](raceConfigCacheTTL, raceConfigCacheCleanupInterval)
)

// Config represents the configuration for the race game.
type Config struct {
	ID                    bson.ObjectID         `yaml:"-" bson:"_id,omitempty"`
	GuildID               discordid.SnowflakeID `yaml:"-" bson:"guild_id,omitempty"`
	BetAmount             int                   `yaml:"bet_amount" bson:"bet_amount"`
	Currency              string                `yaml:"currency" bson:"currency"`
	MaxPrizeAmount        int                   `yaml:"max_prize_amount" bson:"max_prize_amount"`
	MaxNumRacers          int                   `yaml:"max_num_racers" bson:"max_num_racers"`
	MinNumRacers          int                   `yaml:"min_num_racers" bson:"min_num_racers"`
	MinPrizeAmount        int                   `yaml:"min_price_amount" bson:"min_price_amount"`
	Theme                 string                `yaml:"theme" bson:"theme"`
	WaitBetweenRaces      time.Duration         `yaml:"wait_between_races" bson:"wait_between_races"`
	WaitForBets           time.Duration         `yaml:"wait_for_bets" bson:"wait_for_bets"`
	WaitToStart           time.Duration         `yaml:"wait_to_start" bson:"wait_to_start"`
	StartingLine          string                `yaml:"starting_line" bson:"starting_line"`
	Track                 string                `yaml:"track" bson:"track"`
	EndingLine            string                `yaml:"ending_line" bson:"ending_line"`
	BabyDragonBuffPercent int                   `yaml:"babydragon_buff_percent" bson:"babydragon_buff_percent"`
}

// GetConfig gets the race configuration for the guild. If the configuration does not
// exist, then a new one is created.
func GetConfig(guildID discordid.SnowflakeID) *Config {
	key := configCacheKey{
		guildID: guildID,
	}

	if cfg, ok := configCache.Get(key); ok {
		return copyConfig(&cfg)
	}

	cfg := readConfig(guildID)
	if cfg == nil {
		cfg = createNewConfig(guildID)
	}
	if cfg.BabyDragonBuffPercent == 0 {
		cfg.BabyDragonBuffPercent = defaultConfig.BabyDragonBuffPercent
		writeConfig(cfg)
		slog.Debug("set baby dragon buff percent",
			slog.Any("guildID", guildID),
			slog.Int("babydragon_buff_percent", cfg.BabyDragonBuffPercent),
		)
	}

	cfg.StartingLine = defaultConfig.StartingLine
	cfg.EndingLine = defaultConfig.EndingLine

	configCache.Set(key, *cfg)

	return copyConfig(cfg)
}

// createNewConfig creates a new default configuration for the guild.
func createNewConfig(guildID discordid.SnowflakeID) *Config {
	cfg := &Config{
		GuildID:               guildID,
		BetAmount:             defaultConfig.BetAmount,
		Currency:              defaultConfig.Currency,
		MaxPrizeAmount:        defaultConfig.MaxPrizeAmount,
		MaxNumRacers:          defaultConfig.MaxNumRacers,
		MinNumRacers:          defaultConfig.MinNumRacers,
		MinPrizeAmount:        defaultConfig.MinPrizeAmount,
		Theme:                 defaultConfig.Theme,
		WaitBetweenRaces:      defaultConfig.WaitBetweenRaces,
		WaitForBets:           defaultConfig.WaitForBets,
		WaitToStart:           defaultConfig.WaitToStart,
		StartingLine:          defaultConfig.StartingLine,
		Track:                 defaultConfig.Track,
		EndingLine:            defaultConfig.EndingLine,
		BabyDragonBuffPercent: defaultConfig.BabyDragonBuffPercent,
	}
	writeConfig(cfg)
	return cfg
}

// copyConfig creates a deep copy of the configuration.
func copyConfig(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}

	return new(*cfg)
}

// CloseConfigCache stops the config cache cleanup goroutine and clears cached config entries.
func CloseConfigCache() {
	configCache.Destroy()
}

// LoadConfig loads the configuration from the specified YAML file path.
func LoadConfig(path string) error {
	filePath := filepath.Join(path, "race/config.yaml")
	if err := config.LoadConfig(filePath, &defaultConfig); err != nil {
		return err
	}

	slog.Error("GEMS:",
		slog.Any("gems", defaultConfig.EndingLine),
	)

	return nil
}
