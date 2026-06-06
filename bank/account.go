package bank

import (
	"errors"
	"fmt"
	"goblin2/discordid"
	"goblin2/internal/cache"
	"log/slog"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	accountCacheTTL             = 30 * time.Minute
	accountCacheCleanupInterval = 5 * time.Minute
)

type accountCacheKey struct {
	guildID  discordid.SnowflakeID
	memberID discordid.SnowflakeID
}

var (
	accountCache = cache.New[accountCacheKey, Account](accountCacheTTL, accountCacheCleanupInterval)
	accountMu    sync.RWMutex
)

// An Account represents the "bank" account for a given user. This keeps track of the
// in-game currency for the given member of a guild.
type Account struct {
	ID              bson.ObjectID         `bson:"_id,omitempty"`
	GuildID         discordid.SnowflakeID `bson:"guild_id"`
	MemberID        discordid.SnowflakeID `bson:"member_id"`
	CreatedAt       time.Time             `bson:"created_at"`
	CurrentBalance  int                   `bson:"current_balance"`
	MonthlyBalance  int                   `bson:"monthly_balance"`
	LifetimeBalance int                   `bson:"lifetime_balance"`
	Version         int                   `bson:"version"`
}

// GetAccount returns the account for the given guild and member.
func GetAccount(guildID, memberID discordid.SnowflakeID) *Account {
	key := accountCacheKey{
		guildID:  guildID,
		memberID: memberID,
	}

	if account, ok := accountCache.Get(key); ok {
		return copyAccount(&account)
	}

	account := readAccount(key.guildID, key.memberID)
	if account != nil {
		accountCache.Set(key, *account)
		return copyAccount(account)
	}

	account = createNewAccount(guildID, memberID)
	accountCache.Set(key, *account)
	return copyAccount(account)
}

// createNewAccount creates a new account for the given guild and member.
func createNewAccount(guildID, memberID discordid.SnowflakeID) *Account {
	return &Account{
		GuildID:         guildID,
		MemberID:        memberID,
		CreatedAt:       time.Now(),
		CurrentBalance:  theme.DefaultBalance,
		MonthlyBalance:  theme.DefaultBalance,
		LifetimeBalance: theme.DefaultBalance,
	}
}

// copyAccount returns a copy of the given account. This is necessary to prevent external code from modifying the
// cached account directly, which could lead to race conditions and data inconsistencies.
func copyAccount(account *Account) *Account {
	if account == nil {
		return nil
	}

	return new(*account)
}

// CloseAccountCache stops the account cache cleanup goroutine and clears all
// cached account entries.
func CloseAccountCache() {
	accountCache.Destroy()
}

// GetAccounts returns a list of all accounts for the given bank
func GetAccounts(filter interface{}, sortBy interface{}, limit int64) []*Account {
	return readAccounts(filter, sortBy, limit)
}

// Deposit adds the given amount to the account.
func (a *Account) Deposit(amount int) error {
	return UpdateAccount(a.GuildID, a.MemberID, func(acc *Account) error {
		acc.CurrentBalance += amount
		acc.MonthlyBalance += amount
		acc.LifetimeBalance += amount
		slog.Debug("deposit to account",
			slog.Any("guildID", acc.GuildID),
			slog.Any("memberID", acc.MemberID),
			slog.Int("amount", amount),
			slog.Int("balance", acc.CurrentBalance),
		)
		return nil
	})
}

// DepositIntoCurrent adds the given amount to the current balance of the account. This does not affect the
// monthly or lifetime balances.
func (a *Account) DepositIntoCurrent(amount int) error {
	return UpdateAccount(a.GuildID, a.MemberID, func(acc *Account) error {
		acc.CurrentBalance += amount
		slog.Debug("deposit to the current balance for the account",
			slog.Any("guildID", acc.GuildID),
			slog.Any("memberID", acc.MemberID),
			slog.Int("amount", amount),
			slog.Int("balance", acc.CurrentBalance),
		)
		return nil
	})
}

// Withdraw withdraws the given amount from the account.
func (a *Account) Withdraw(amount int) error {
	return UpdateAccount(a.GuildID, a.MemberID, func(a *Account) error {
		if a.CurrentBalance < amount {
			slog.Debug("insufficient funds to withdraw from account",
				slog.Any("guildID", a.GuildID),
				slog.Any("memberID", a.MemberID),
				slog.Int("balance", a.CurrentBalance),
				slog.Int("amount", amount),
			)
			return ErrInsufficientFunds
		}
		a.CurrentBalance -= amount
		a.MonthlyBalance -= amount
		a.LifetimeBalance -= amount

		slog.Debug("withdraw from account",
			slog.Any("guildID", a.GuildID),
			slog.Any("memberID", a.MemberID),
			slog.Int("balance", a.CurrentBalance),
			slog.Int("amount", amount),
		)

		return nil
	})
}

// WithdrawFromCurrent withdraws the given amount from the account. This only updates the current balance
// and does not affect the monthly or lifetime balances.
func (a *Account) WithdrawFromCurrent(amount int) error {
	return UpdateAccount(a.GuildID, a.MemberID, func(a *Account) error {
		if a.CurrentBalance < amount {
			slog.Debug("insufficient funds to withdraw from the account",
				slog.Any("guildID", a.GuildID),
				slog.Any("memberID", a.MemberID),
				slog.Int("balance", a.CurrentBalance),
				slog.Int("amount", amount),
			)
			return ErrInsufficientFunds
		}
		a.CurrentBalance -= amount

		slog.Debug("withdraw from the current balance for the account",
			slog.Any("guildID", a.GuildID),
			slog.Any("memberID", a.MemberID),
			slog.Int("balance", a.CurrentBalance),
			slog.Int("amount", amount),
		)

		return nil
	})
}

// SetBalance sets the current balance of the account to the given amount.
func (a *Account) SetBalance(amount int) error {
	if amount < 0 {
		return ErrInvalidAmount
	}
	return UpdateAccount(a.GuildID, a.MemberID, func(a *Account) error {
		a.CurrentBalance = amount
		if a.LifetimeBalance < amount {
			a.LifetimeBalance = amount
		}
		if a.MonthlyBalance < amount {
			a.MonthlyBalance = amount
		}
		slog.Debug("set account balance",
			slog.Any("guildID", a.GuildID),
			slog.Any("memberID", a.MemberID),
			slog.Int("balance", a.CurrentBalance),
		)
		return nil
	})
}

// GetLifetimeRanking returns the lifetime ranking of the account in the guild (server). The ranking is based on the
// lifetime balance of the account, with the highest balance being ranked first.
func (a *Account) GetLifetimeRanking() (int64, error) {
	filter := bson.M{
		"guild_id":         a.GuildID,
		"lifetime_balance": bson.M{"$gt": a.LifetimeBalance},
	}
	return a.getRank(filter)
}

// GetMonthlyRanking returns the monthly global ranking on the server for a given player. The ranking is based on the
// monthly balance of the account, with the highest balance being ranked first.
func (a *Account) GetMonthlyRanking() (int64, error) {
	filter := bson.M{
		"guild_id":        a.GuildID,
		"monthly_balance": bson.M{"$gt": a.MonthlyBalance},
	}
	return a.getRank(filter)
}

// GetCurrentRanking returns the current ranking of the account in the guild (server). The ranking is based on the
// current balance of the account, with the highest balance being ranked first.
func (a *Account) GetCurrentRanking() (int64, error) {
	filter := bson.M{
		"guild_id":        a.GuildID,
		"current_balance": bson.M{"$gt": a.CurrentBalance},
	}
	return a.getRank(filter)
}

// getRank returns the rank of the account in the guild (server) based on the given filter. The filter should specify
// the guild ID and the balance field to compare against.
func (a *Account) getRank(filter bson.M) (int64, error) {
	rank, err := db.Count(accountCollection, filter)
	if err != nil {
		return 0, err
	}
	return rank + 1, nil
}

// String returns a string representation of the Account.
func (a *Account) String() string {
	return fmt.Sprintf(
		"Account{ID=%s, GuildID=%v, MemberID=%v, CreatedAt=%s, Current=%d, Monthly=%d, Lifetime=%d}",
		a.ID.Hex(),
		a.GuildID,
		a.MemberID,
		a.CreatedAt.Format(time.RFC3339),
		a.CurrentBalance,
		a.MonthlyBalance,
		a.LifetimeBalance,
	)
}

// UpdateAccount updates the account with the given mutation, retrying on version conflicts.
func UpdateAccount(guildID, memberID discordid.SnowflakeID, mutate func(*Account) error) error {
	const maxRetries = 3

	accountMu.RLock()
	defer accountMu.RUnlock()

	key := accountCacheKey{
		guildID:  guildID,
		memberID: memberID,
	}

	for range maxRetries {
		account := GetAccount(guildID, memberID)

		if err := mutate(account); err != nil {
			return err
		}

		var err error
		if account.ID.IsZero() {
			err = writeAccount(account)
		} else {
			err = updateAccount(account)
		}

		if err == nil {
			accountCache.Set(key, *account)
			return nil
		}
		if !errors.Is(err, ErrVersionConflict) {
			accountCache.Delete(key)
			return err
		}

		accountCache.Delete(key)

		slog.Warn("version conflict on bank account, retrying",
			slog.Any("guildID", guildID),
			slog.Any("memberID", memberID),
		)
	}

	return fmt.Errorf("failed to update account after %d retries: %w", maxRetries, ErrVersionConflict)
}

// ResetMonthlyBalances resets the monthly balances for all accounts in all banks.
func ResetMonthlyBalances() {
	accountMu.Lock()
	defer accountMu.Unlock()

	filter := bson.M{}
	update := bson.M{
		"$set": bson.M{
			"monthly_balance": 0,
		},
	}

	_, err := db.UpdateMany(accountCollection, filter, update)
	if err != nil {
		slog.Error("unable to reset monthly balances for all accounts", "error", err)
		return
	}

	accountCache.Clear()
}
