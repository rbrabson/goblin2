package payday

import (
	"goblin2/database"
	"goblin2/discordid"
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
	var payday *Payday
	err := db.FindOne(paydayCollection, filter, &payday)
	if err != nil {
		slog.Debug("payday not found in the database", "guildID", guildID, "error", err)
		return nil
	}

	return payday
}

// writePayday saves the payday information for the guild into the database.
func writePayday(payday *Payday) error {
	filter := bson.M{"guild_id": payday.GuildID}
	_, err := db.ReplaceOneUpsert(paydayCollection, filter, payday)
	if err != nil {
		slog.Error("unable to save payday to the database", "guildID", payday.GuildID, "error", err)
		return err
	}
	return nil
}

// readAccount loads payday information for a given account in the guild from the database.
func readAccount(payday *Payday, accountID discordid.SnowflakeID) *Account {
	filter := bson.M{"guild_id": payday.GuildID, "member_id": accountID}
	var account *Account
	err := db.FindOne(paydayAccountCollection, filter, &account)
	if err != nil {
		slog.Debug("payday account not found in the database", "guildID", payday.GuildID, "memberID", accountID, "error", err)
		return nil
	}
	account.GuildID = payday.GuildID

	return account
}

// writeAccount saves the payday information for a given account in the guild into the database.
func writeAccount(account *Account) error {
	var filter bson.M
	if account.ID != bson.NilObjectID {
		filter = bson.M{"_id": account.ID}
	} else {
		filter = bson.M{"guild_id": account.GuildID, "member_id": account.MemberID}
	}
	_, err := db.ReplaceOneUpsert(paydayAccountCollection, filter, account)
	if err != nil {
		slog.Debug("unable to write payday account to the database", "guildID", account.GuildID, "memberID", account.MemberID, "filter", filter, "error", err)
		return err
	}

	return nil
}
