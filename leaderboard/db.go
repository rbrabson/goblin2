package leaderboard

import (
	"goblin2/database"
	"goblin2/discordid"
	"log/slog"

	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	leaderboardCollection = "leaderboards"
)

var (
	db *database.MongoDB
)

// readLeaderboard reads the leaderboard from the database and returns the value if it exists, or returns nil if the
// bank does not exist in the database
func readLeaderboard(guildID snowflake.ID) *Leaderboard {
	filter := bson.M{"guild_id": discordid.SnowflakeID(guildID)}
	var lb Leaderboard
	err := db.FindOne(leaderboardCollection, filter, &lb)
	if err != nil {
		slog.Debug("leaderboard not found in the database", "guildID", guildID, "error", err)
		return nil
	}

	return &lb
}

// writeBank creates or updates the bank for a guild in the database being used by the Discord bot.
func writeLeaderboard(lb *Leaderboard) error {
	filter := bson.M{"guild_id": lb.GuildID}

	_, err := db.ReplaceOneUpsert(leaderboardCollection, filter, lb)
	if err != nil {
		slog.Error("unable to save leaderboard to the database", "guildID", lb.GuildID, "error", err)
		return err
	}

	return nil
}
