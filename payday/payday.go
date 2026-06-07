package payday

import (
	"errors"
	"fmt"
	"goblin2/bank"
	"goblin2/internal/cache"
	"goblin2/internal/discordid"
	"log/slog"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	paydayCacheTTL             = 30 * time.Minute
	paydayCacheCleanupInterval = 5 * time.Minute
)

type paydayCacheKey struct {
	guildID discordid.SnowflakeID
}

var (
	paydayCache = cache.New[paydayCacheKey, Payday](paydayCacheTTL, paydayCacheCleanupInterval)
	paydayMu    sync.RWMutex
)

// Payday is the daily payment for members of a guild (server).
type Payday struct {
	ID                bson.ObjectID         `bson:"_id,omitempty"`
	GuildID           discordid.SnowflakeID `bson:"guild_id"`
	Amount            int                   `bson:"payday_amount"`
	PaydayFrequency   time.Duration         `bson:"payday_frequency"`
	MaxStreak         int                   `bson:"max_streak"`
	StreakPerDayBonus int                   `bson:"streak_per_day_bonus"`
	Version           int                   `bson:"version"`
}

// GetPayday returns the payday information for a server, creating a new one if necessary.
func GetPayday(guildID discordid.SnowflakeID) *Payday {
	key := paydayCacheKey{
		guildID: guildID,
	}

	if payday, ok := paydayCache.Get(key); ok {
		return copyPayday(&payday)
	}

	payday := readPayday(key.guildID)
	if payday != nil {
		paydayCache.Set(key, *payday)
		return copyPayday(payday)
	}

	payday = createNewPayday(guildID)
	paydayCache.Set(key, *payday)
	return copyPayday(payday)
}

// copyPayday returns a copy of the given paydayHandler. This prevents callers from mutating the cached paydayHandler directly.
func copyPayday(payday *Payday) *Payday {
	if payday == nil {
		return nil
	}

	return new(*payday)
}

// ClosePaydayCache stops the payday cache cleanup goroutine and clears all cached payday entries.
func ClosePaydayCache() {
	paydayCache.Destroy()
}

// UpdatePayday updates the payday configuration with the given mutation, retrying on version conflicts.
func UpdatePayday(guildID discordid.SnowflakeID, mutate func(*Payday) error) error {
	const maxRetries = 3

	paydayMu.RLock()
	defer paydayMu.RUnlock()

	key := paydayCacheKey{
		guildID: guildID,
	}

	for range maxRetries {
		payday := GetPayday(guildID)

		if err := mutate(payday); err != nil {
			return err
		}

		var err error
		if payday.ID.IsZero() {
			err = writePayday(payday)
		} else {
			err = updatePayday(payday)
		}

		if err == nil {
			paydayCache.Set(key, *payday)
			return nil
		}
		if !errors.Is(err, bank.ErrVersionConflict) {
			paydayCache.Delete(key)
			return err
		}

		paydayCache.Delete(key)

		slog.Warn("version conflict on payday, retrying",
			slog.Any("guildID", guildID),
		)
	}

	return fmt.Errorf("failed to update payday after %d retries: %w", maxRetries, bank.ErrVersionConflict)
}

// GetAccount returns an account in the guild (server). If one doesn't exist, then nil is returned.
func (payday *Payday) GetAccount(memberID discordid.SnowflakeID) *Account {
	return GetPaydayAccount(payday.GuildID, memberID)
}

// SetPaydayAmount sets the number of credits a player deposits into their account on a given payday.
func (payday *Payday) SetPaydayAmount(amount int) {
	if err := UpdatePayday(payday.GuildID, func(latest *Payday) error {
		latest.Amount = amount
		return nil
	}); err != nil {
		slog.Error("error writing payday", "guildID", payday.GuildID, "error", err)
	}
}

// SetPaydayFrequency sets the frequency of paydays at which a player can deposit credits into their account.
func (payday *Payday) SetPaydayFrequency(frequency time.Duration) {
	if err := UpdatePayday(payday.GuildID, func(latest *Payday) error {
		latest.PaydayFrequency = frequency
		return nil
	}); err != nil {
		slog.Error("error writing payday", "guildID", payday.GuildID, "error", err)
	}
}

// createNewPayday creates new paydayHandler information for a server/guild.
// If the default paydayHandler configuration file cannot be read or decoded, then a
// default paydayHandler configuration is created.
func createNewPayday(guildID discordid.SnowflakeID) *Payday {
	payday := &Payday{
		GuildID:           guildID,
		Amount:            defaultConfig.PaydayAmount,
		PaydayFrequency:   defaultConfig.PaydayFrequency,
		MaxStreak:         defaultConfig.MaxStreak,
		StreakPerDayBonus: defaultConfig.StreakPerDayBonus,
	}

	slog.Debug("create new payday config",
		slog.Any("guildID", payday.GuildID),
		slog.Any("payday", payday),
	)

	return payday
}

// String returns a string representation of the Payday.
func (payday *Payday) String() string {
	return fmt.Sprintf("Payday{ID=%s, GuildID=%s, Amount=%d, PaydayFrequency=%s}",
		payday.ID.Hex(),
		payday.GuildID,
		payday.Amount,
		payday.PaydayFrequency,
	)
}
