package race

import (
	"goblin2/database"
	"goblin2/internal/discordid"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	configCollection = "race_configs"
	memberCollection = "race_members"
)

var (
	db *database.MongoDB
)

// readConfig loads the race configuration from the database. If it does not exist, then
// a `nil` value is returned.
func readConfig(guildID discordid.SnowflakeID) *Config {
	filter := bson.M{"guild_id": guildID}
	var config Config
	err := db.FindOne(configCollection, filter, &config)
	if err != nil {
		slog.Debug("race configuration not found in the database", slog.Any("guildID", guildID), slog.Any("error", err))
		return nil
	}

	return &config
}

// writeConfig stores the race configuration in the database.
func writeConfig(config *Config) {
	var filter bson.M
	if config.ID != bson.NilObjectID {
		filter = bson.M{"_id": config.ID}
	} else {
		filter = bson.M{"guild_id": config.GuildID}
	}
	if _, err := db.ReplaceOneUpsert(configCollection, filter, config); err != nil {
		slog.Error("failed to write the race configuration to the database", slog.Any("guildID", config.GuildID), slog.Any("error", err))
	}
}

// readConfig loads the race member from the database. If it does not exist, then
// a `nil` value is returned.
func readRaceMember(guildID, memberID discordid.SnowflakeID) *Member {
	filter := bson.D{{Key: "guild_id", Value: guildID}, {Key: "member_id", Value: memberID}}
	var member Member
	err := db.FindOne(memberCollection, filter, &member)
	if err != nil {
		slog.Debug("race member not found in the database",
			slog.Any("guildID", guildID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		return nil
	}

	return &member
}

// writeRaceMember creates or updates the race member in the database
func writeRaceMember(member *Member) {
	var filter bson.M
	if member.ID != bson.NilObjectID {
		filter = bson.M{"_id": member.ID}
	} else {
		filter = bson.M{"guild_id": member.GuildID, "member_id": member.MemberID}
	}
	if _, err := db.ReplaceOneUpsert(memberCollection, filter, member); err != nil {
		slog.Error("failed to write the race member to the database",
			slog.Any("guildID", member.GuildID),
			slog.Any("memberID", member.MemberID),
			slog.Any("error", err),
		)
	}
}
