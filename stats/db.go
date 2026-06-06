package stats

import (
	"goblin2/database"
	"goblin2/discordid"
	"log/slog"
	"time"

	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	playerStatsCollection = "player_stats"
	gameStatsCollection   = "server_stats"
)

var (
	db *database.MongoDB
)

// readMemberStats retrieves the member statistics for a specific member in a guild for a specific game.
func readPlayerStats(guildID discordid.SnowflakeID, memberID discordid.SnowflakeID, game string) (*PlayerStats, error) {
	var ps PlayerStats
	filter := bson.M{"guild_id": guildID, "member_id": memberID, "game": game}
	err := db.FindOne(playerStatsCollection, filter, &ps)
	if err != nil {
		return nil, err
	}
	return &ps, nil
}

// writeNewPlayerStats inserts new player statistics for a specific member in a guild.
func writeNewPlayerStats(ps *PlayerStats) error {
	ps.Version = 0

	result, err := db.InsertOne(playerStatsCollection, ps)
	if err != nil {
		slog.Debug("writing new player stats", "collection", playerStatsCollection, "PlayerStats", ps, "error", err)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		ps.ID = id
	}

	return nil
}

// updatePlayerStats updates player statistics using optimistic locking via the version field.
// Returns ErrVersionConflict if another writer updated the stats since they were read.
func updatePlayerStats(ps *PlayerStats) error {
	filter := bson.M{
		"guild_id":  ps.GuildID,
		"member_id": ps.MemberID,
		"game":      ps.Game,
	}

	if ps.Version == 0 {
		filter["$or"] = bson.A{
			bson.M{"version": ps.Version},
			bson.M{"version": bson.M{"$exists": false}},
		}
	} else {
		filter["version"] = ps.Version
	}

	update := bson.M{
		"$set": bson.M{
			"first_played":           ps.FirstPlayed,
			"last_played":            ps.LastPlayed,
			"number_of_times_played": ps.NumberOfTimesPlayed,
		},
		"$inc": bson.M{
			"version": 1,
		},
	}

	result, err := db.UpdateOne(playerStatsCollection, filter, update)
	if err != nil {
		slog.Debug("updating player stats", "collection", playerStatsCollection, "PlayerStats", ps, "filter", filter, "error", err)
		return err
	}
	if result.MatchedCount == 0 {
		return ErrVersionConflict
	}

	ps.Version++

	return nil
}

// deletePlayerStats removes the player statistics for a specific member in a guild.
func deletePlayerStats(ps *PlayerStats) error {
	var filter bson.M
	if ps.ID != bson.NilObjectID {
		filter = bson.M{"_id": ps.ID}
	} else {
		filter = bson.M{"guild_id": ps.GuildID, "member_id": ps.MemberID, "game": ps.Game}
	}
	_, err := db.DeleteMany(playerStatsCollection, filter)
	if err != nil {
		return err
	}
	return nil
}

// readGameStats retrieves the game statistics for a specific game in a guild.
func readGameStats(guildID discordid.SnowflakeID, game string, day time.Time) (*GameStats, error) {
	var gs GameStats
	filter := bson.M{"guild_id": guildID, "game": game, "day": day}
	err := db.FindOne(gameStatsCollection, filter, &gs)
	if err != nil {
		return nil, err
	}
	return &gs, nil
}

// writeNewGameStats inserts new game statistics for a guild.
func writeNewGameStats(gs *GameStats) error {
	gs.Version = 0

	result, err := db.InsertOne(gameStatsCollection, gs)
	if err != nil {
		slog.Debug("writing new game stats", "collection", gameStatsCollection, "GameStats", gs, "error", err)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		gs.ID = id
	}

	return nil
}

// updateGameStats updates game statistics using optimistic locking via the version field.
// Returns ErrVersionConflict if another writer updated the stats since they were read.
func updateGameStats(gs *GameStats) error {
	filter := bson.M{
		"guild_id": gs.GuildID,
		"game":     gs.Game,
		"day":      gs.Day,
	}

	if gs.Version == 0 {
		filter["$or"] = bson.A{
			bson.M{"version": gs.Version},
			bson.M{"version": bson.M{"$exists": false}},
		}
	} else {
		filter["version"] = gs.Version
	}

	update := bson.M{
		"$set": bson.M{
			"unique_players": gs.UniquePlayers,
			"total_players":  gs.TotalPlayers,
			"games_played":   gs.GamesPlayed,
		},
		"$inc": bson.M{
			"version": 1,
		},
	}

	result, err := db.UpdateOne(gameStatsCollection, filter, update)
	if err != nil {
		slog.Debug("updating game stats", "collection", gameStatsCollection, "GameStats", gs, "filter", filter, "error", err)
		return err
	}
	if result.MatchedCount == 0 {
		return ErrVersionConflict
	}

	gs.Version++

	return nil
}

// deleteGameStats removes the game statistics for a specific game in a guild.
func deleteGameStats(gs *GameStats) error {
	var filter bson.M
	if gs.ID != bson.NilObjectID {
		filter = bson.M{"_id": gs.ID}
	} else {
		filter = bson.M{"guild_id": gs.GuildID, "game": gs.Game, "day": gs.Day}
	}
	_, err := db.DeleteMany(gameStatsCollection, filter)
	if err != nil {
		return err
	}
	return nil
}

// getLastDatePlayed retrieves the last date a member played a game in a guild.
func getLastDatePlayed(guildID snowflake.ID, memberID snowflake.ID) time.Time {
	// Use aggregation pipeline to find the maximum last_played date for the member
	pipeline := mongo.Pipeline{
		// Stage 1: Match documents for the specific guild and member
		bson.D{
			{Key: "$match", Value: bson.D{
				{Key: "guild_id", Value: guildID},
				{Key: "member_id", Value: memberID},
			}},
		},
		// Stage 2: Group all documents and find the maximum last_played date
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: nil},
				{Key: "last_date_played", Value: bson.D{
					{Key: "$max", Value: "$last_played"},
				}},
			}},
		},
	}

	docs, err := db.Aggregate(playerStatsCollection, pipeline)
	if err != nil {
		slog.Error("failed to get last date played",
			slog.Any("guild_id", guildID),
			slog.Any("member_id", memberID),
			slog.Any("error", err),
		)
		return time.Time{}
	}

	if len(docs) == 0 {
		slog.Debug("no game data found for member",
			slog.Any("guild_id", guildID),
			slog.Any("member_id", memberID),
		)
		return time.Time{}
	}

	result := docs[0]
	lastPlayed := getTimeFromPipeline(result["last_date_played"])

	return lastPlayed
}

// getFirstGameDate retrieves the earliest date a game was played by any member in a guild.
func getFirstGameDate(guildID string, game string) time.Time {
	var pipeline mongo.Pipeline
	if game == "" || game == "all" {
		pipeline = mongo.Pipeline{
			// Stage 1: Match documents for the specific guild for all games
			bson.D{
				{Key: "$match", Value: bson.D{
					{Key: "guild_id", Value: guildID},
				}},
			},
			// Stage 2: Group all documents and find the minimum first_played date
			bson.D{
				{Key: "$group", Value: bson.D{
					{Key: "_id", Value: nil},
					{Key: "first_game_date", Value: bson.D{
						{Key: "$min", Value: "$first_played"},
					}},
				}},
			},
		}
	} else {
		// Use aggregation pipeline to find the minimum first_played date
		pipeline = mongo.Pipeline{
			// Stage 1: Match documents for the specific guild and game
			bson.D{
				{Key: "$match", Value: bson.D{
					{Key: "guild_id", Value: guildID},
					{Key: "game", Value: game},
				}},
			},
			// Stage 2: Group all documents and find the minimum first_played date
			bson.D{
				{Key: "$group", Value: bson.D{
					{Key: "_id", Value: nil},
					{Key: "first_game_date", Value: bson.D{
						{Key: "$min", Value: "$first_played"},
					}},
				}},
			},
		}
	}

	docs, err := db.Aggregate(playerStatsCollection, pipeline)
	if err != nil {
		slog.Error("failed to get first game date",
			slog.String("guild_id", guildID),
			slog.String("game", game),
			slog.Any("error", err),
		)
		return today().AddDate(-1, 0, 0) // Default to 1 years ago if no data found
	}

	if len(docs) == 0 {
		slog.Debug("no game data found",
			slog.String("guild_id", guildID),
			slog.String("game", game),
		)
		return today().AddDate(-1, 0, 0) // Default to 1 years ago if no data found
	}

	result := docs[0]
	firstGameDate := getTimeFromPipeline(result["first_game_date"])

	return firstGameDate
}

// getFirstGameDate retrieves the earliest date a game was played by any member in a guild.
func getFirstServerGameDate(guildID string, game string) time.Time {
	var pipeline mongo.Pipeline
	if game == "" || game == "all" {
		pipeline = mongo.Pipeline{
			// Stage 1: Match documents for the specific guild for all games
			bson.D{
				{Key: "$match", Value: bson.D{
					{Key: "guild_id", Value: guildID},
				}},
			},
			// Stage 2: Group all documents and find the minimum first_played date
			bson.D{
				{Key: "$group", Value: bson.D{
					{Key: "_id", Value: nil},
					{Key: "day", Value: bson.D{
						{Key: "$min", Value: "$day"},
					}},
				}},
			},
		}
	} else {
		// Use aggregation pipeline to find the minimum first_played date
		pipeline = mongo.Pipeline{
			// Stage 1: Match documents for the specific guild and game
			bson.D{
				{Key: "$match", Value: bson.D{
					{Key: "guild_id", Value: guildID},
					{Key: "game", Value: game},
				}},
			},
			// Stage 2: Group all documents and find the minimum first_played date
			bson.D{
				{Key: "$group", Value: bson.D{
					{Key: "_id", Value: nil},
					{Key: "day", Value: bson.D{
						{Key: "$min", Value: "$day"},
					}},
				}},
			},
		}
	}

	docs, err := db.Aggregate(gameStatsCollection, pipeline)
	if err != nil {
		slog.Error("failed to get first game date",
			slog.String("guild_id", guildID),
			slog.String("game", game),
			slog.Any("error", err),
		)
		return today().AddDate(-1, 0, 0) // Default to 1 years ago if no data found
	}

	if len(docs) == 0 {
		slog.Debug("no game data found",
			slog.String("guild_id", guildID),
			slog.String("game", game),
		)
		return today().AddDate(-1, 0, 0) // Default to 1 years ago if no data found
	}

	result := docs[0]
	firstGameDate := getTimeFromPipeline(result["day"])

	return firstGameDate
}
