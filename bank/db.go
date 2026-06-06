package bank

import (
	"goblin2/database"
	"goblin2/discordid"
	"log/slog"
	"sync"

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
func readBank(guildID string) *Bank {
	filter := bson.M{"guild_id": guildID}
	var bank Bank
	err := db.FindOne(bankCollection, filter, &bank)
	if err != nil {
		slog.Debug("bank not found in the database", "guildID", guildID, "error", err)
		return nil
	}

	bank.mutex = &sync.RWMutex{}
	return &bank
}

// writeBank creates or replaces the bank data in the database being used by the Discord bot.
func writeBank(bank *Bank) error {
	filter := bankFilter(bank)

	_, err := db.ReplaceOneUpsert(bankCollection, filter, bank)
	if err != nil {
		slog.Error("unable to save bank to the database",
			slog.Any("guildID", bank.GuildID),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// updateBankDefaultBalance updates only the default balance for a bank.
func updateBankDefaultBalance(bank *Bank) error {
	filter := bankFilter(bank)
	update := bson.M{
		"$set": bson.M{
			"default_balance": bank.DefaultBalance,
		},
		"$setOnInsert": bson.M{
			"guild_id":  bank.GuildID,
			"bank_name": bank.Name,
			"currency":  bank.Currency,
		},
	}

	_, err := db.UpdateOneUpsert(bankCollection, filter, update)
	if err != nil {
		slog.Error("unable to update bank default balance",
			slog.Any("guildID", bank.GuildID),
			slog.Int("defaultBalance", bank.DefaultBalance),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// updateBankName updates only the name for a bank.
func updateBankName(bank *Bank) error {
	filter := bankFilter(bank)
	update := bson.M{
		"$set": bson.M{
			"bank_name": bank.Name,
		},
		"$setOnInsert": bson.M{
			"guild_id":        bank.GuildID,
			"currency":        bank.Currency,
			"default_balance": bank.DefaultBalance,
		},
	}

	_, err := db.UpdateOneUpsert(bankCollection, filter, update)
	if err != nil {
		slog.Error("unable to update bank name",
			slog.Any("guildID", bank.GuildID),
			slog.String("name", bank.Name),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// updateBankCurrency updates only the currency for a bank.
func updateBankCurrency(bank *Bank) error {
	filter := bankFilter(bank)
	update := bson.M{
		"$set": bson.M{
			"currency": bank.Currency,
		},
		"$setOnInsert": bson.M{
			"guild_id":        bank.GuildID,
			"bank_name":       bank.Name,
			"default_balance": bank.DefaultBalance,
		},
	}

	_, err := db.UpdateOneUpsert(bankCollection, filter, update)
	if err != nil {
		slog.Error("unable to update bank currency",
			slog.Any("guildID", bank.GuildID),
			slog.String("currency", bank.Currency),
			slog.Any("error", err),
		)
		return err
	}

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

// bankFilter returns the filter used to locate a bank document.
func bankFilter(bank *Bank) bson.M {
	return bson.M{"guild_id": bank.GuildID}
}
