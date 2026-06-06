package bank

import (
	"fmt"
	"goblin2/discordid"
	"log/slog"
	"sync"

	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// A Bank is the repository for all bank accounts for a given guild.
type Bank struct {
	ID             bson.ObjectID         `bson:"_id,omitempty"`
	GuildID        discordid.SnowflakeID `bson:"guild_id"`
	Name           string                `bson:"bank_name"`
	Currency       string                `bson:"currency"`
	DefaultBalance int                   `bson:"default_balance"`
	mutex          *sync.RWMutex         `bson:"-"`
}

// GetBank returns the bank for the given guild.
func GetBank(guildID snowflake.ID) *Bank {
	// TODO: implement bank retrieval logic
	b := &Bank{
		GuildID: discordid.NewSnowflakeID(guildID),
		mutex:   &sync.RWMutex{},
	}
	return b
}

// snapshot returns a copy of the bank with an independent mutex.
func (b *Bank) snapshot() *Bank {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.snapshotLocked()
}

// snapshotLocked returns a copy of the bank's persisted fields.
// The caller must already hold b.mutex.
func (b *Bank) snapshotLocked() *Bank {
	return &Bank{
		ID:             b.ID,
		GuildID:        b.GuildID,
		Name:           b.Name,
		Currency:       b.Currency,
		DefaultBalance: b.DefaultBalance,
		mutex:          &sync.RWMutex{},
	}
}

// SetDefaultBalance sets the default balance for the bank.
func (b *Bank) SetDefaultBalance(balance int) {
	b.mutex.Lock()

	if balance == b.DefaultBalance {
		b.mutex.Unlock()
		return
	}

	b.DefaultBalance = balance
	bankCopy := b.snapshotLocked()
	b.mutex.Unlock()

	if err := updateBankDefaultBalance(bankCopy); err != nil {
		slog.Error("error writing bank",
			slog.Any("guildID", bankCopy.GuildID),
			slog.Any("error", err),
		)
		return
	}

	slog.Info("set default balance",
		slog.Any("guildID", bankCopy.GuildID),
		slog.Int("balance", bankCopy.DefaultBalance),
	)
}

// SetName sets the name of the bank.
func (b *Bank) SetName(name string) {
	b.mutex.Lock()

	if name == b.Name {
		b.mutex.Unlock()
		return
	}

	b.Name = name
	bankCopy := b.snapshotLocked()
	b.mutex.Unlock()

	if err := updateBankName(bankCopy); err != nil {
		slog.Error("error writing bank",
			slog.Any("guildID", bankCopy.GuildID),
			slog.Any("error", err),
		)
		return
	}

	slog.Info("set bank name",
		slog.String("name", bankCopy.Name),
		slog.Any("guildID", bankCopy.GuildID),
	)
}

// SetCurrency sets the currency used by the bank.
func (b *Bank) SetCurrency(currency string) {
	b.mutex.Lock()

	if currency == b.Currency {
		b.mutex.Unlock()
		return
	}

	b.Currency = currency
	bankCopy := b.snapshotLocked()
	b.mutex.Unlock()

	if err := updateBankCurrency(bankCopy); err != nil {
		slog.Error("error writing bank",
			slog.Any("guildID", bankCopy.GuildID),
			slog.Any("error", err),
		)
		return
	}

	slog.Info("set currency",
		slog.Any("guildID", bankCopy.GuildID),
		slog.String("currency", bankCopy.Currency),
	)
}

// lock and unlock are used to lock and unlock the bank.
func (b *Bank) lock() {
	b.mutex.Lock()
}

// unlock is used to unlock the bank.
func (b *Bank) unlock() {
	b.mutex.Unlock()
}

// String returns a string representation of the Bank.
func (b *Bank) String() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return fmt.Sprintf("Bank{ID: %s, GuildID: %s, Name: %s, Currency: %s, DefaultBalance: %d}",
		b.ID.Hex(),
		b.GuildID,
		b.Name,
		b.Currency,
		b.DefaultBalance,
	)
}
