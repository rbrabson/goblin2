package leaderboard

import (
	"goblin2/database"
	"goblin2/plugin"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "leaderboard"
)

// Plugin is the plugin for the leaderboard
type Plugin struct {
	status plugin.Status
	name   string
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin creates a new leaderboard plugin.
func NewPlugin(_ string) (*Plugin, error) {
	p := &Plugin{
		status: plugin.Running,
		name:   pluginName,
	}
	return p, nil
}

// Initialize initializes the leaderboard plugin.
func (p *Plugin) Initialize(mongoDB *database.MongoDB, client *bot.Client) {
	db = mongoDB
	go sendAllMonthlyLeaderboards(client)
}

// GetHelp returns the help message for the leaderboard plugin.
func (p *Plugin) GetHelp() map[string]string {
	return map[string]string{
		"/lb current":  "Gets the current economy leaderboard.",
		"/lb lifetime": "Gets the lifetime economy leaderboard.",
		"/lb monthly":  "Gets the monthly economy leaderboard.",
		"/lb rank":     "Gets the member rank for the leaderboards.",
	}
}

// GetName returns the name of the leaderboard plugin.
func (p *Plugin) GetName() string {
	return p.name
}

// GetAdminHelp returns the admin help message for the leaderboard plugin.
func (p *Plugin) GetAdminHelp() map[string]string {
	return map[string]string{
		"/lb-admin channel": "Sets the channel ID where the monthly leaderboard is published.",
		"/lb-admin info":    "Gets information about the leaderboard configuration.",
	}
}

// Stop stops the leaderboard plugin.
func (p *Plugin) Stop() {
	p.status = plugin.Stopped
}

// Status returns the current status of the leaderboard plugin.
func (p *Plugin) Status() plugin.Status {
	return p.status
}

// GetSlashHandlers returns the slash command handlers for the leaderboard plugin.
func (p *Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/lb/current":       currentLeaderboard,
		"/lb/monthly":       monthlyLeaderboard,
		"/lb/lifetime":      lifetimeLeaderboard,
		"/lb/rank":          rank,
		"/lb-admin/channel": leaderboardAdmin,
		"/lb-admin/info":    leaderboardAdmin,
	}
}

// GetSlashCommands returns the slash commands for the leaderboard plugin.
func (p *Plugin) GetSlashCommands() []discord.ApplicationCommandCreate {
	commands := make([]discord.ApplicationCommandCreate, 0, len(adminCommands)+len(memberCommands))
	commands = append(commands, adminCommands...)
	commands = append(commands, memberCommands...)
	return commands
}

// GetComponentHandlers returns the component handlers for the bank plugin.
func (p *Plugin) GetComponentHandlers() map[string]handler.ComponentHandler {
	return nil
}
