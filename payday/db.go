package payday

import (
	"goblin2/bank"
	"goblin2/database"
	"goblin2/internal/discordid"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	paydayCollection        = "paydays"
	paydayAccountCollection = "payday_accounts"
)

var (
	db *database.MongoDB
)

// readPayday loads payday information for the guild from the database.
func readPayday(guildID discordid.SnowflakeID) *Payday {
	filter := bson.M{
		"guild_id": guildID,
	}
	var payday Payday
	err := db.FindOne(paydayCollection, filter, &payday)
	if err != nil {
		slog.Debug("payday not found in the database", "guildID", guildID, "error", err)
		return nil
	}

	return &payday
}

// writePayday inserts new payday information for the guild into the database.
func writePayday(payday *Payday) error {
	payday.Version = 0

	result, err := db.InsertOne(paydayCollection, payday)
	if err != nil {
		slog.Error("unable to create payday",
			slog.Any("guildID", payday.GuildID),
			slog.Any("error", err),
		)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		payday.ID = id
	}

	return nil
}

// updatePayday updates existing payday information using optimistic locking via the version field.
// Returns bank.ErrVersionConflict if another writer updated the payday since it was read.
func updatePayday(payday *Payday) error {
	filter := bson.M{
		"guild_id": payday.GuildID,
	}

	if payday.Version == 0 {
		filter["$or"] = bson.A{
			bson.M{"version": payday.Version},
			bson.M{"version": bson.M{"$exists": false}},
		}
	} else {
		filter["version"] = payday.Version
	}

	update := bson.M{
		"$set": bson.M{
			"payday_amount":        payday.Amount,
			"payday_frequency":     payday.PaydayFrequency,
			"max_streak":           payday.MaxStreak,
			"streak_per_day_bonus": payday.StreakPerDayBonus,
		},
		"$inc": bson.M{
			"version": 1,
		},
	}

	result, err := db.UpdateOne(paydayCollection, filter, update)
	if err != nil {
		slog.Error("unable to update payday",
			slog.Any("guildID", payday.GuildID),
			slog.Any("version", payday.Version),
			slog.Any("error", err),
		)
		return err
	}
	if result.MatchedCount == 0 {
		return bank.ErrVersionConflict
	}

	payday.Version++

	return nil
}

// readAccount loads payday information for a given account in the guild from the database.
func readAccount(guildID discordid.SnowflakeID, accountID discordid.SnowflakeID) *Account {
	filter := bson.M{"guild_id": guildID, "member_id": accountID}
	var account Account
	err := db.FindOne(paydayAccountCollection, filter, &account)
	if err != nil {
		slog.Debug("payday account not found in the database", "guildID", guildID, "memberID", accountID, "error", err)
		return nil
	}

	return &account
}

// writeAccount inserts a new payday account into the database.
func writeAccount(account *Account) error {
	account.Version = 0

	result, err := db.InsertOne(paydayAccountCollection, account)
	if err != nil {
		slog.Debug("unable to create payday account",
			slog.Any("guildID", account.GuildID),
			slog.Any("memberID", account.MemberID),
			slog.Any("error", err),
		)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		account.ID = id
	}

	return nil
}

// updateAccount updates an existing payday account using optimistic locking via the version field.
// Returns bank.ErrVersionConflict if another writer updated the payday account since it was read.
func updateAccount(account *Account) error {
	filter := bson.M{
		"guild_id":  account.GuildID,
		"member_id": account.MemberID,
	}

	if account.Version == 0 {
		filter["$or"] = bson.A{
			bson.M{"version": account.Version},
			bson.M{"version": bson.M{"$exists": false}},
		}
	} else {
		filter["version"] = account.Version
	}

	update := bson.M{
		"$set": bson.M{
			"next_payday":       account.NextPayday,
			"current_streak":    account.CurrentStreak,
			"max_streak":        account.MaxStreak,
			"total_paydays":     account.TotalPaydays,
			"total_amount_paid": account.TotalAmountPaid,
		},
		"$inc": bson.M{
			"version": 1,
		},
	}

	result, err := db.UpdateOne(paydayAccountCollection, filter, update)
	if err != nil {
		slog.Error("unable to update payday account",
			slog.Any("guildID", account.GuildID),
			slog.Any("memberID", account.MemberID),
			slog.Any("version", account.Version),
			slog.Any("error", err),
		)
		return err
	}
	if result.MatchedCount == 0 {
		return bank.ErrVersionConflict
	}

	account.Version++

	return nil
}
