package payday

import (
	"fmt"
	"goblin2/discordid"
	"log/slog"
	"time"

	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Payday is the daily payment for members of a guild (server).
type Payday struct {
	ID                bson.ObjectID         `bson:"_id,omitempty"`
	GuildID           discordid.SnowflakeID `bson:"guild_id"`
	Amount            int                   `bson:"payday_amount"`
	PaydayFrequency   time.Duration         `bson:"payday_frequency"`
	MaxStreak         int                   `bson:"max_streak"`
	StreakPerDayBonus int                   `bson:"streak_per_day_bonus"`
}

// GetPayday returns the payday information for a server, creating a new one if necessary.
func GetPayday(guildID snowflake.ID) *Payday {
	payday := readPayday(discordid.NewSnowflakeID(guildID))
	if payday == nil {
		payday = createNewPayday(guildID)
	}

	return payday
}

// GetAccount returns an account in the guild (server). If one doesn't exist, then nil is returned.
func (payday *Payday) GetAccount(memberID snowflake.ID) *Account {
	account := readAccount(payday, discordid.NewSnowflakeID(memberID))

	if account == nil {
		account = newAccount(payday, memberID)
	}

	return account
}

// SetPaydayAmount sets the number of credits a player deposits into their account on a given payday.
func (payday *Payday) SetPaydayAmount(amount int) {
	payday.Amount = amount

	if err := writePayday(payday); err != nil {
		slog.Error("error writing payday", "guildID", payday.GuildID, "error", err)
	}
}

// SetPaydayFrequency sets the frequency of paydays at which a player can deposit credits into their account.
func (payday *Payday) SetPaydayFrequency(frequency time.Duration) {
	payday.PaydayFrequency = frequency

	if err := writePayday(payday); err != nil {
		slog.Error("error writing payday", "guildID", payday.GuildID, "error", err)
	}
}

// createNewPayday creates new payday information for a server/guild.
// If the default payday configuration file cannot be read or decoded, then a
// default payday configuration is created.
func createNewPayday(guildID snowflake.ID) *Payday {
	payday := &Payday{
		GuildID:           discordid.NewSnowflakeID(guildID),
		Amount:            defaultConfig.PaydayAmount,
		PaydayFrequency:   defaultConfig.PaydayFrequency,
		MaxStreak:         defaultConfig.MaxStreak,
		StreakPerDayBonus: defaultConfig.StreakPerDayBonus,
	}

	if err := writePayday(payday); err != nil {
		slog.Error("error writing payday",
			slog.Any("guildID", payday.GuildID),
			slog.Any("error", err),
		)
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
