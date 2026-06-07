package main

import (
	"goblin2/bank"
	"goblin2/database"
	"goblin2/disgobot"
	"goblin2/games/blackjack"
	"goblin2/games/heist"
	"goblin2/games/race"
	"goblin2/games/slots"
	"goblin2/guild"
	"goblin2/internal/config"
	"goblin2/internal/log"
	"goblin2/leaderboard"
	"goblin2/payday"
	"goblin2/plugin"
	"goblin2/shop"
	"goblin2/stats"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/joho/godotenv"
)

var (
	Version  string
	Revision string
	BotName  string
)

func init() {

}

// getPlugins returns a list of plugins to be registered with the bot.
func getPlugins(configPath string) []plugin.Plugin {
	plugins := make([]plugin.Plugin, 0, 10)

	var err error
	var p plugin.Plugin

	p, err = guild.NewPlugin(configPath)
	if err != nil {
		slog.Error("failed to create guild plugin", "error", err)
		os.Exit(-1)
	}
	plugins = append(plugins, p)

	p, err = bank.NewPlugin(configPath)
	if err != nil {
		slog.Error("failed to create bank plugin", "error", err)
		os.Exit(-1)
	}
	plugins = append(plugins, p)

	p, err = leaderboard.NewPlugin(configPath)
	if err != nil {
		slog.Error("failed to create leaderboard plugin", "error", err)
		os.Exit(-1)
	}
	plugins = append(plugins, p)

	p, err = stats.NewPlugin(configPath)
	if err != nil {
		slog.Error("failed to create stats plugin", "error", err)
		os.Exit(-1)
	}
	plugins = append(plugins, p)

	p, err = payday.NewPlugin(configPath)
	if err != nil {
		slog.Error("failed to create payday plugin", "error", err)
		os.Exit(-1)
	}
	plugins = append(plugins, p)

	p, err = shop.NewPlugin(configPath)
	if err != nil {
		slog.Error("failed to create shop plugin", "error", err)
		os.Exit(-1)
	}
	plugins = append(plugins, p)

	p, err = heist.NewPlugin(configPath)
	if err != nil {
		slog.Error("failed to create heist plugin", "error", err)
		os.Exit(-1)
	}
	plugins = append(plugins, p)

	p, err = race.NewPlugin(configPath)
	if err != nil {
		slog.Error("failed to create race plugin", "error", err)
		os.Exit(-1)
	}
	plugins = append(plugins, p)

	p, err = blackjack.NewPlugin(configPath)
	if err != nil {
		slog.Error("failed to create blackjack plugin", "error", err)
		os.Exit(-1)
	}
	plugins = append(plugins, p)

	p, err = slots.NewPlugin(configPath)
	if err != nil {
		slog.Error("failed to create slots plugin", "error", err)
		os.Exit(-1)
	}
	plugins = append(plugins, p)

	return plugins
}

func main() {
	// Initialize the logger
	if err := godotenv.Load(".env"); err != nil {
		slog.Info("failed to load .env file", "error", err)
	}
	configPath := os.Getenv("GOBLIN_CONFIG_PATH")

	logPath := filepath.Join(configPath, "log/config.yaml")
	var logConfig log.Config
	if err := config.LoadConfig(logPath, &logConfig); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(-1)
	}
	log.Initialize(logConfig)

	version := Version
	if Revision != "" {
		version += "-" + Revision
	}
	if version == "-" {
		version = "development"
	}
	botName := BotName
	if botName == "" {
		botName = "Goblin-Dev"
	}

	slog.Info("starting goblin",
		slog.String("Version", version),
		slog.String("BotName", botName),
	)

	db, err := database.New(configPath)
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

	botCfg, err := disgobot.LoadConfig(configPath)
	if err != nil {
		slog.Error("failed to load bot config", "error", err)
		os.Exit(-1)
	}
	bot := disgobot.NewBot(botCfg, version)

	for _, p := range getPlugins(configPath) {
		bot.RegisterPlugin(p)
	}

	if err := bot.Start(db); err != nil {
		slog.Error("failed to start the bot", "error", err)
		os.Exit(-1)
	}
	defer bot.Stop()

	// Wait for the user to cancel the program
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	slog.Info("press Ctrl+C to exit")
	<-sc

	slog.Info("stopping goblin")
}
