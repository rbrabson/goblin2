package bank

import (
	"goblin2/database"
	"goblin2/internal/discordid"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	bankCollection    = "banks"
	accountCollection = "bank_accounts"
)

var (
	db *database.MongoDB
)

// readBank gets the bank from the database and returns the value if it exists, or returns nil if the
// bank does not exist in the database.
func readBank(guildID discordid.SnowflakeID) *Bank {
	filter := bson.M{"guild_id": guildID}
	var bank Bank
	err := db.FindOne(bankCollection, filter, &bank)
	if err != nil {
		slog.Debug("bank not found in the database",
			slog.Any("guildID", guildID),
			slog.Any("error", err),
		)
		return nil
	}

	return &bank
}

// writeBank inserts a new bank into the database.
func writeBank(bank *Bank) error {
	bank.Version = 0

	result, err := db.InsertOne(bankCollection, bank)
	if err != nil {
		slog.Error("unable to create bank",
			slog.Any("guildID", bank.GuildID),
			slog.Any("error", err),
		)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		bank.ID = id
	}

	return nil
}

// updateBank updates an existing bank using optimistic locking via the version field.
// Returns ErrVersionConflict if another writer updated the bank since it was read.
func updateBank(bank *Bank) error {
	filter := bson.M{
		"guild_id": bank.GuildID,
	}

	if bank.Version == 0 {
		filter["$or"] = bson.A{
			bson.M{"version": bank.Version},
			bson.M{"version": bson.M{"$exists": false}},
		}
	} else {
		filter["version"] = bank.Version
	}

	update := bson.M{
		"$set": bson.M{
			"bank_name":       bank.Name,
			"currency":        bank.Currency,
			"default_balance": bank.DefaultBalance,
		},
		"$inc": bson.M{
			"version": 1,
		},
	}

	result, err := db.UpdateOne(bankCollection, filter, update)
	if err != nil {
		slog.Error("unable to update bank",
			slog.Any("guildID", bank.GuildID),
			slog.Any("version", bank.Version),
			slog.Any("error", err),
		)
		return err
	}
	if result.MatchedCount == 0 {
		return ErrVersionConflict
	}

	bank.Version++

	return nil
}

// readAccount gets the bank account from the database and returns the value if it exists,
// or returns nil if the account does not exist in the database.
func readAccount(guildID discordid.SnowflakeID, memberID discordid.SnowflakeID) *Account {
	filter := bson.M{
		"guild_id":  guildID,
		"member_id": memberID,
	}

	var account Account
	err := db.FindOne(accountCollection, filter, &account)
	if err != nil {
		slog.Debug("bank account not found in the database",
			slog.Any("guildID", guildID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		return nil
	}

	return &account
}

// readAccounts all the matching accounts for the given bank.
func readAccounts(filter interface{}, sortBy interface{}, limit int64) []*Account {
	var accounts []*Account
	err := db.FindMany(accountCollection, filter, &accounts, sortBy, limit)
	if err != nil {
		slog.Error("unable to read accounts from the database", "error", err)
		return nil
	}

	return accounts
}

// writeAccount inserts a new account into the database (for new accounts only).
func writeAccount(account *Account) error {
	account.Version = 0

	result, err := db.InsertOne(accountCollection, account)
	if err != nil {
		slog.Error("unable to create bank account",
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

// updateAccount updates an existing account using optimistic locking via the version field.
// Returns ErrVersionConflict if another writer updated the account since it was read.
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
			"current_balance":  account.CurrentBalance,
			"monthly_balance":  account.MonthlyBalance,
			"lifetime_balance": account.LifetimeBalance,
		},
		"$inc": bson.M{
			"version": 1,
		},
	}

	result, err := db.UpdateOne(accountCollection, filter, update)
	if err != nil {
		slog.Error("unable to update bank account",
			slog.Any("guildID", account.GuildID),
			slog.Any("memberID", account.MemberID),
			slog.Any("version", account.Version),
			slog.Any("error", err),
		)
		return err
	}
	if result.MatchedCount == 0 {
		return ErrVersionConflict
	}

	account.Version++

	return nil
}
