package stats

import (
	"goblin2/discordid"
	"log/slog"
	"time"

	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// GameStats represents the statistics for a specific game in a guild on a specific day.
type GameStats struct {
	ID            bson.ObjectID         `bson:"_id,omitempty"`
	GuildID       discordid.SnowflakeID `bson:"guild_id"`
	Game          string                `bson:"game"`
	Day           time.Time             `bson:"day"`
	UniquePlayers int                   `bson:"unique_players"`
	TotalPlayers  int                   `bson:"total_players"`
	GamesPlayed   int                   `bson:"games_played"`
}

// GamesPlayed represents the statistics for games played in a guild on a specific day.
type GamesPlayed struct {
	NumberOfDays          float64
	TotalUniquePlayers    int
	UniquePlayers         int
	UniquePlayersPerDay   float64
	TotalPlayers          int
	TotalPlayersPerDay    float64
	TotalGamesPlayed      int
	AverageGamesPerDay    float64
	AveragePlayersPerGame float64
	AverageGamesPerPlayer float64
}

// getGameStats retrieves the game statistics for a specific game in a guild on a specific day.
func getGameStats(guildID snowflake.ID, game string, day time.Time) *GameStats {
	gs, err := readGameStats(discordid.NewSnowflakeID(guildID), game, day)
	if err != nil || gs == nil {
		gs = newGameStats(guildID, game, day)
	}
	return gs
}

// newGameStats creates a new GameStats instance for a specific game in a guild on a specific day.
func newGameStats(guildID snowflake.ID, game string, day time.Time) *GameStats {
	return &GameStats{
		GuildID: discordid.NewSnowflakeID(guildID),
		Game:    game,
		Day:     day,
	}
}

// UpdateGameStats updates the game statistics for a specific game in a guild.
func UpdateGameStats(guildID snowflake.ID, game string, memberIDs []snowflake.ID) {
	statsLock.Lock()
	defer statsLock.Unlock()

	todayTime := today()

	var newUniquePlayersForGame, newUniquePlayersForAllGames int
	for _, memberID := range memberIDs {
		ps := getPlayerStats(guildID, memberID, game)
		if ps == nil {
			slog.Error("unable to get or create player stats",
				slog.Any("guild_id", guildID),
				slog.Any("member_id", memberID),
				slog.String("game", game),
			)
			continue
		}

		// Check if this is the first time the player has played this game today
		// and if this is the first time the player has played any game today
		if ps.LastPlayed.Before(todayTime) {
			newUniquePlayersForGame++
		}
		lastDatePlayed := getLastDatePlayed(guildID, memberID)
		if lastDatePlayed.Before(todayTime) {
			newUniquePlayersForAllGames++
		}

		ps.LastPlayed = todayTime
		ps.NumberOfTimesPlayed++

		err := writePlayerStats(ps)
		if err != nil {
			slog.Error("failed to update player stats",
				slog.Any("guild_id", guildID),
				slog.Any("member_id", memberID),
				slog.String("game", game),
				slog.Any("error", err))
			return
		}

		slog.Debug("player stats updated",
			slog.Any("guild_id", guildID),
			slog.Any("member_id", memberID),
			slog.String("game", game),
			slog.Time("last_played", ps.LastPlayed),
			slog.Int("number_of_times_played", ps.NumberOfTimesPlayed),
		)
	}

	gs := getGameStats(guildID, game, todayTime)
	gs.UniquePlayers += newUniquePlayersForGame
	gs.TotalPlayers += len(memberIDs)
	gs.GamesPlayed++

	err := writeGameStats(gs)
	if err != nil {
		slog.Error("failed to update server stats",
			slog.Any("guild_id", guildID),
			slog.String("game", game),
			slog.Time("day", gs.Day),
			slog.Any("error", err))
		return
	}

	slog.Debug("server stats for game updated",
		slog.Any("guild_id", gs.GuildID),
		slog.String("game", gs.Game),
		slog.Time("day", gs.Day),
		slog.Int("games_played", gs.GamesPlayed),
		slog.Int("new_unique_players_for_game", newUniquePlayersForGame),
		slog.Int("unique_players", gs.UniquePlayers),
		slog.Int("new_total_players_for_game", len(memberIDs)),
		slog.Int("total_players", gs.TotalPlayers),
	)

	// Update unique players for all games
	gsAll := getGameStats(guildID, "all", todayTime)
	gsAll.UniquePlayers += newUniquePlayersForAllGames
	gsAll.TotalPlayers += len(memberIDs)
	gsAll.GamesPlayed++

	if err := writeGameStats(gsAll); err != nil {
		slog.Error("failed to update server stats for all games",
			slog.Any("guild_id", guildID),
			slog.String("game", "all"),
			slog.Time("day", gsAll.Day),
			slog.Any("error", err))
		return
	}

	slog.Debug("server stats for all games updated",
		slog.Any("guild_id", gsAll.GuildID),
		slog.String("game", gsAll.Game),
		slog.Time("day", gsAll.Day),
		slog.Int("games_played", gsAll.GamesPlayed),
		slog.Int("new_unique_players_for_all_games", newUniquePlayersForAllGames),
		slog.Int("unique_players", gsAll.UniquePlayers),
		slog.Int("new_total_players_for_all_games", len(memberIDs)),
		slog.Int("total_players", gsAll.TotalPlayers),
	)
}

// GetGamesPlayed retrieves the aggregated games played statistics from the game_stats table
func GetGamesPlayed(guildID string, game string, startDate time.Time, endDate time.Time) (*GamesPlayed, error) {
	// Default to all games
	if game == "" {
		game = "all"
	}

	// Stage 1: Match documents for the specific guild, game, and date range
	pipeline := []bson.D{
		{{Key: "$match", Value: bson.D{
			{Key: "guild_id", Value: guildID},
			{Key: "game", Value: game},
			{Key: "day", Value: bson.D{
				{Key: "$gte", Value: startDate},
				{Key: "$lte", Value: endDate},
			}},
		}}},
		// Stage 2: Group all documents and sum the statistics
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total_unique_players", Value: bson.D{{Key: "$sum", Value: "$unique_players"}}},
			{Key: "total_players", Value: bson.D{{Key: "$sum", Value: "$total_players"}}},
			{Key: "total_games_played", Value: bson.D{{Key: "$sum", Value: "$games_played"}}},
		}}},
	}

	docs, err := db.Aggregate(GameStatsCollection, pipeline)
	if err != nil {
		return nil, err
	}

	if len(docs) == 0 {
		return &GamesPlayed{}, nil
	}

	result := docs[0]
	gamesPlayed := &GamesPlayed{
		TotalPlayers:       getInt(result["total_players"]),
		TotalUniquePlayers: getInt(result["total_unique_players"]),
		TotalGamesPlayed:   getInt(result["total_games_played"]),
	}
	gamesPlayed.UniquePlayers, _ = GetUniquePlayers(guildID, game, startDate, endDate)
	gamesPlayed.NumberOfDays = endDate.Sub(startDate).Hours() / 24.0
	gamesPlayed.TotalPlayersPerDay = float64(gamesPlayed.TotalPlayers) / gamesPlayed.NumberOfDays
	gamesPlayed.UniquePlayersPerDay = float64(gamesPlayed.TotalUniquePlayers) / gamesPlayed.NumberOfDays
	gamesPlayed.AverageGamesPerDay = float64(gamesPlayed.TotalGamesPlayed) / gamesPlayed.NumberOfDays
	gamesPlayed.AveragePlayersPerGame = float64(gamesPlayed.TotalPlayers) / float64(gamesPlayed.TotalGamesPlayed)
	gamesPlayed.AverageGamesPerPlayer = float64(gamesPlayed.TotalGamesPlayed) / float64(gamesPlayed.UniquePlayers)

	return gamesPlayed, nil
}
