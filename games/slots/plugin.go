package slots

import (
	"goblin2/database"
	"goblin2/plugin"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "slots"
)

var (
	db *database.MongoDB
)

// Plugin is the plugin for the slots system used by the bot
type Plugin struct {
	status plugin.Status
	name   string
}

// Ensure the plugin implements the Plugin interface
var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin creates a new slots plugin.
func NewPlugin(cfgPath string) (*Plugin, error) {
	if err := LoadConfig(cfgPath); err != nil {
		return nil, err
	}
	if err := LoadSymbols(cfgPath); err != nil {
		return nil, err
	}
	if err := LoadLookupTable(cfgPath); err != nil {
		return nil, err
	}
	if err := LoadPayoutTable(cfgPath); err != nil {
		return nil, err
	}

	return &Plugin{
		status: plugin.Running,
		name:   pluginName,
	}, nil
}

// Stop stops the slots game. This is called when the bot is shutting down.
func (p *Plugin) Stop() {
	p.status = plugin.Stopped
}

// Status returns the status of the slots game.
func (p *Plugin) Status() plugin.Status {
	return p.status
}

// Initialize saves the database to be used by the slots system.
func (p *Plugin) Initialize(mongoDB *database.MongoDB, _ *bot.Client) {
	db = mongoDB
}

// SetDB sets the database to be used by the slots system. This is used for testing.
func SetDB(mongoDB *database.MongoDB) {
	db = mongoDB
}

// GetName returns the name of the slots system plugin
func (p *Plugin) GetName() string {
	return p.name
}

// GetHelp returns the member help for the slots system
func (p *Plugin) GetHelp() map[string]string {
	return map[string]string{
		"/slots play":     "Play the slot machine.",
		"/slots paytable": "Get the pay table for the slot machine.",
		"/slots stats":    "Shows a user's stats.",
	}
}

// GetAdminHelp returns the admin help for the slots system
func (p *Plugin) GetAdminHelp() map[string]string {
	return nil
}

// GetSlashHandlers returns the slash command handlers for the slots plugin.
func (p *Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/slots/play":     playSlotsHandler,
		"/slots/paytable": payTableHandler,
		"/slots/stats":    showStatsHandler,
	}
}

// GetEventListeners returns the gateway event listeners for the slots plugin.
func (p *Plugin) GetEventListeners() []bot.EventListener {
	return nil
}

// GetComponentHandlers returns the component handlers for the slots plugin.
func (p *Plugin) GetComponentHandlers() map[string]handler.ComponentHandler {
	return nil
}

// GetSlashCommands returns the slash commands for the slots plugin.
func (p *Plugin) GetSlashCommands() []discord.ApplicationCommandCreate {
	return memberCommands
}
