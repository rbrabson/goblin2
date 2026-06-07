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
	paydayAccountCacheTTL             = 30 * time.Minute
	paydayAccountCacheCleanupInterval = 5 * time.Minute
)

type paydayAccountCacheKey struct {
	guildID  discordid.SnowflakeID
	memberID discordid.SnowflakeID
}

var (
	paydayAccountCache = cache.New[paydayAccountCacheKey, Account](paydayAccountCacheTTL, paydayAccountCacheCleanupInterval)
	paydayAccountMu    sync.RWMutex
)

// Account is a user on the server that can a payday every 23 hours
type Account struct {
	ID              bson.ObjectID         `json:"_id,omitempty" bson:"_id,omitempty"`
	GuildID         discordid.SnowflakeID `json:"guild_id" bson:"guild_id"`
	MemberID        discordid.SnowflakeID `json:"member_id" bson:"member_id"`
	NextPayday      time.Time             `json:"next_payday" bson:"next_payday"`
	CurrentStreak   int                   `json:"current_streak" bson:"current_streak"`
	MaxStreak       int                   `json:"max_streak" bson:"max_streak"`
	TotalPaydays    int                   `json:"total_paydays" bson:"total_paydays"`
	TotalAmountPaid int                   `json:"total_amount_paid" bson:"total_amount_paid"`
	Version         int                   `json:"version" bson:"version"`
}

// GetPaydayAccount returns the payday account for the given guild and member.
func GetPaydayAccount(guildID, memberID discordid.SnowflakeID) *Account {
	key := paydayAccountCacheKey{
		guildID:  guildID,
		memberID: memberID,
	}

	if account, ok := paydayAccountCache.Get(key); ok {
		return copyPaydayAccount(&account)
	}

	account := readAccount(key.guildID, key.memberID)
	if account != nil {
		paydayAccountCache.Set(key, *account)
		return copyPaydayAccount(account)
	}

	account = newAccount(guildID, memberID)
	paydayAccountCache.Set(key, *account)
	return copyPaydayAccount(account)
}

// newAccount creates new payday information for a server/guild.
func newAccount(guildID, memberID discordid.SnowflakeID) *Account {
	return &Account{
		MemberID: memberID,
		GuildID:  guildID,
	}
}

// copyPaydayAccount returns a copy of the given account. This prevents callers from mutating the cached account directly.
func copyPaydayAccount(account *Account) *Account {
	if account == nil {
		return nil
	}

	return new(*account)
}

// ClosePaydayAccountCache stops the payday account cache cleanup goroutine and clears all cached payday account entries.
func ClosePaydayAccountCache() {
	paydayAccountCache.Destroy()
}

// UpdatePaydayAccount updates the payday account with the given mutation, retrying on version conflicts.
func UpdatePaydayAccount(guildID, memberID discordid.SnowflakeID, mutate func(*Account) error) error {
	const maxRetries = 3

	paydayAccountMu.RLock()
	defer paydayAccountMu.RUnlock()

	key := paydayAccountCacheKey{
		guildID:  guildID,
		memberID: memberID,
	}

	for range maxRetries {
		account := GetPaydayAccount(guildID, memberID)

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
			paydayAccountCache.Set(key, *account)
			return nil
		}
		if !errors.Is(err, bank.ErrVersionConflict) {
			paydayAccountCache.Delete(key)
			return err
		}

		paydayAccountCache.Delete(key)

		slog.Warn("version conflict on payday account, retrying",
			slog.Any("guildID", guildID),
			slog.Any("memberID", memberID),
		)
	}

	return fmt.Errorf("failed to update payday account after %d retries: %w", maxRetries, bank.ErrVersionConflict)
}

// getNextPayday returns the next payday for the user.
func (a *Account) getNextPayday() time.Time {
	return a.NextPayday
}

// setNextPayday sets the next payday for the user.
func (a *Account) setNextPayday(minWait time.Duration) {
	if err := UpdatePaydayAccount(a.GuildID, a.MemberID, func(latest *Account) error {
		latest.NextPayday = time.Now().Add(minWait)
		latest.CurrentStreak = a.CurrentStreak
		latest.MaxStreak = a.MaxStreak
		latest.TotalPaydays = a.TotalPaydays
		latest.TotalAmountPaid = a.TotalAmountPaid
		return nil
	}); err != nil {
		slog.Error("unable to save account to the database",
			slog.Any("guildID", a.GuildID),
			slog.Any("memberID", a.MemberID),
			slog.Any("error", err),
		)
		return
	}

	slog.Debug("set next payday",
		slog.Any("guildID", a.GuildID),
		slog.Any("memberID", a.MemberID),
		slog.Int("paydayStreak", a.CurrentStreak),
		slog.Int("maxStreak", a.MaxStreak),
		slog.Time("nextPayday", a.NextPayday),
	)
}

// getPayAmount returns the number of credits the user will receive on their next payday.
func (a *Account) getPayAmount() int {
	payday := GetPayday(a.GuildID)
	a.updateStreak(payday.PaydayFrequency)
	basePay := payday.Amount
	bonus := payday.StreakPerDayBonus
	streakReset := payday.MaxStreak
	streak := a.CurrentStreak

	var pay int
	if streak != 0 && streakReset != 0 && bonus != 0.0 {
		multiplier := (streak - 1) % streakReset
		pay = basePay + (bonus * multiplier)
	} else {
		pay = basePay
	}

	return pay
}

// updateStreak updates the user's current streak based on their last payday.
func (a *Account) updateStreak(minWait time.Duration) {
	if a.NextPayday.After(time.Now()) {
		return
	}

	previousPayday := a.NextPayday.Add(-minWait)
	if time.Since(previousPayday) > (2 * 24 * time.Hour) {
		a.CurrentStreak = 1
	} else {
		a.CurrentStreak++
	}
	a.MaxStreak = max(a.MaxStreak, a.CurrentStreak)
	a.TotalPaydays++
	a.TotalPaydays = max(a.TotalPaydays, a.MaxStreak) // To handle adding TotalPaydays to existing accounts
}

// String returns a string representation of the Account.
func (a *Account) String() string {
	return fmt.Sprintf("PaydayAccount{ID=%s, GuildID=%s, MemberID=%s, CurrentStreak=%d, MaxStreak=%d, NextPayday=%s}",
		a.ID.Hex(),
		a.GuildID,
		a.MemberID,
		a.CurrentStreak,
		a.MaxStreak,
		a.NextPayday,
	)
}
