package leaderboard

import (
	"goblin2/bank"
	"goblin2/database"
	"goblin2/internal/discordid"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	leaderboardCollection = "leaderboards"
)

var (
	db *database.MongoDB
)

// readLeaderboard reads the leaderboard from the database and returns the value if it exists, or returns nil if the
// leaderboard does not exist in the database.
func readLeaderboard(guildID discordid.SnowflakeID) *Leaderboard {
	filter := bson.M{"guild_id": guildID}
	var lb Leaderboard
	err := db.FindOne(leaderboardCollection, filter, &lb)
	if err != nil {
		slog.Debug("leaderboard not found in the database", "guildID", guildID, "error", err)
		return nil
	}

	return &lb
}

// writeLeaderboard inserts a new leaderboard into the database.
func writeLeaderboard(lb *Leaderboard) error {
	lb.Version = 0

	result, err := db.InsertOne(leaderboardCollection, lb)
	if err != nil {
		slog.Error("unable to create leaderboard", "guildID", lb.GuildID, "error", err)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		lb.ID = id
	}

	return nil
}

// updateLeaderboard updates an existing leaderboard using optimistic locking via the version field.
// Returns bank.ErrVersionConflict if another writer updated the leaderboard since it was read.
func updateLeaderboard(lb *Leaderboard) error {
	filter := bson.M{
		"guild_id": lb.GuildID,
	}

	if lb.Version == 0 {
		filter["$or"] = bson.A{
			bson.M{"version": lb.Version},
			bson.M{"version": bson.M{"$exists": false}},
		}
	} else {
		filter["version"] = lb.Version
	}

	update := bson.M{
		"$set": bson.M{
			"channel_id":  lb.ChannelID,
			"last_season": lb.LastSeason,
		},
		"$inc": bson.M{
			"version": 1,
		},
	}

	result, err := db.UpdateOne(leaderboardCollection, filter, update)
	if err != nil {
		slog.Error("unable to update leaderboard",
			slog.Any("guildID", lb.GuildID),
			slog.Any("version", lb.Version),
			slog.Any("error", err),
		)
		return err
	}
	if result.MatchedCount == 0 {
		return bank.ErrVersionConflict
	}

	lb.Version++

	return nil
}
