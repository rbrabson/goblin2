package main

import (
	"goblin2/database"
	"goblin2/games/slots"
	"goblin2/internal/discordid"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		slog.Info("failed to load .env file", "error", err)
	}
	configPath := os.Getenv("GOBLIN_CONFIG_PATH")

	guildID := os.Getenv("DISCORD_GUILD_ID")
	if guildID == "" {
		slog.Error("DISCORD_GUILD_ID environment variable not set")
		os.Exit(1)
	}

	dbName := os.Getenv("GOBLIN_MONGODB_DATABASE")
	dbURL := os.Getenv("GOBLIN_MONGODB_URL")

	db, err := database.New(dbName, dbURL)
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(-1)
	}
	defer func(db *database.MongoDB) {
		err := db.Close()
		if err != nil {
			slog.Error("failed to close the database",
				slog.Any("error", err),
			)
		}
	}(db)

	if _, err := slots.NewPlugin(configPath); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(-1)
	}

	slots.SetDB(db)

	id, err := discordid.SnowflakeIDFromString(guildID)
	if err != nil {
		slog.Error("failed to parse guild ID",
			slog.String("guildID", guildID),
			slog.Any("error", err),
		)
		os.Exit(-1)
	}

	averages, err := slots.GetPayoutAverages(id)
	if err != nil {
		slog.Error("error getting payout averages",
			slog.String("guildID", guildID),
			slog.Any("error", err),
		)
		return
	}

	p := message.NewPrinter(language.AmericanEnglish)

	totalGames := averages.TotalWins + averages.TotalLosses
	p.Printf("Total wins %d (%.2f%%)\n", averages.TotalWins, float64(averages.TotalWins)/float64(totalGames)*100)
	p.Printf("Total losses %d (%.2f%%)\n", averages.TotalLosses, float64(averages.TotalLosses)/float64(totalGames)*100)
	p.Printf("Average wins: %.0f (%.2f%%)\n", averages.AverageTotalWins, averages.AverageWinPercentage)
	p.Printf("Average losses: %.0f (%.2f%%)\n", averages.AverageTotalLosses, averages.AverageLossPercentage)
	p.Printf("Average return: %.2f%% (%d won from %d bet)\n", averages.AverageReturns, averages.TotalWon, averages.TotalBet)
}
