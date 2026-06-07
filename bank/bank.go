package bank

import (
	"errors"
	"fmt"
	"goblin2/internal/cache"
	"goblin2/internal/discordid"
	"log/slog"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	bankCacheTTL             = 30 * time.Minute
	bankCacheCleanupInterval = 5 * time.Minute
)

type bankCacheKey struct {
	guildID discordid.SnowflakeID
}

var (
	bankCache = cache.New[bankCacheKey, Bank](bankCacheTTL, bankCacheCleanupInterval)
	bankMu    sync.RWMutex
)

// A Bank is the repository for all bank accounts for a given guild.
type Bank struct {
	ID             bson.ObjectID         `bson:"_id,omitempty"`
	GuildID        discordid.SnowflakeID `bson:"guild_id"`
	Name           string                `bson:"bank_name"`
	Currency       string                `bson:"currency"`
	DefaultBalance int                   `bson:"default_balance"`
	Version        int                   `bson:"version"`
}

// GetBank returns the bank for the given guild.
func GetBank(guildID discordid.SnowflakeID) *Bank {
	key := bankCacheKey{
		guildID: guildID,
	}

	if bank, ok := bankCache.Get(key); ok {
		return copyBank(&bank)
	}

	bank := readBank(key.guildID)
	if bank != nil {
		bankCache.Set(key, *bank)
		return copyBank(bank)
	}

	bank = createDefaultBank(guildID)
	bankCache.Set(key, *bank)
	return copyBank(bank)
}

// createDefaultBank creates a new bank for the given guild using the configured theme defaults.
func createDefaultBank(guildID discordid.SnowflakeID) *Bank {
	return &Bank{
		GuildID:        guildID,
		Name:           theme.BankName,
		Currency:       theme.Currency,
		DefaultBalance: theme.DefaultBalance,
	}
}

// copyBank returns a copy of the given bank. This prevents callers from mutating the cached bank directly.
func copyBank(bank *Bank) *Bank {
	if bank == nil {
		return nil
	}

	return new(*bank)
}

// CloseBankCache stops the bank cache cleanup goroutine and clears all cached bank entries.
func CloseBankCache() {
	bankCache.Destroy()
}

// UpdateBank updates the bank with the given mutation, retrying on version conflicts.
func UpdateBank(guildID discordid.SnowflakeID, mutate func(*Bank) error) error {
	const maxRetries = 3

	bankMu.RLock()
	defer bankMu.RUnlock()

	key := bankCacheKey{
		guildID: guildID,
	}

	for range maxRetries {
		bank := GetBank(guildID)

		if err := mutate(bank); err != nil {
			return err
		}

		var err error
		if bank.ID.IsZero() {
			err = writeBank(bank)
		} else {
			err = updateBank(bank)
		}

		if err == nil {
			bankCache.Set(key, *bank)
			return nil
		}
		if !errors.Is(err, ErrVersionConflict) {
			bankCache.Delete(key)
			return err
		}

		bankCache.Delete(key)

		slog.Warn("version conflict on bank, retrying",
			slog.Any("guildID", guildID),
		)
	}

	return fmt.Errorf("failed to update bank after %d retries: %w", maxRetries, ErrVersionConflict)
}

// SetDefaultBalance sets the default balance for the bank.
func (b *Bank) SetDefaultBalance(balance int) {
	if err := UpdateBank(b.GuildID, func(bank *Bank) error {
		if balance == bank.DefaultBalance {
			return nil
		}

		bank.DefaultBalance = balance
		slog.Info("set default balance",
			slog.Any("guildID", bank.GuildID),
			slog.Int("balance", bank.DefaultBalance),
		)
		return nil
	}); err != nil {
		slog.Error("error writing bank",
			slog.Any("guildID", b.GuildID),
			slog.Any("error", err),
		)
	}
}

// SetName sets the name of the bank.
func (b *Bank) SetName(name string) {
	if err := UpdateBank(b.GuildID, func(bank *Bank) error {
		if name == bank.Name {
			return nil
		}

		bank.Name = name
		slog.Info("set bank name",
			slog.String("name", bank.Name),
			slog.Any("guildID", bank.GuildID),
		)
		return nil
	}); err != nil {
		slog.Error("error writing bank",
			slog.Any("guildID", b.GuildID),
			slog.Any("error", err),
		)
	}
}

// SetCurrency sets the currency used by the bank.
func (b *Bank) SetCurrency(currency string) {
	if err := UpdateBank(b.GuildID, func(bank *Bank) error {
		if currency == bank.Currency {
			return nil
		}

		bank.Currency = currency
		slog.Info("set currency",
			slog.Any("guildID", bank.GuildID),
			slog.String("currency", bank.Currency),
		)
		return nil
	}); err != nil {
		slog.Error("error writing bank",
			slog.Any("guildID", b.GuildID),
			slog.Any("error", err),
		)
	}
}

// String returns a string representation of the Bank.
func (b *Bank) String() string {
	return fmt.Sprintf("Bank{ID: %s, GuildID: %s, Name: %s, Currency: %s, DefaultBalance: %d}",
		b.ID.Hex(),
		b.GuildID,
		b.Name,
		b.Currency,
		b.DefaultBalance,
	)
}
