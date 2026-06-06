package payday

import (
	"goblin2/database"
	"goblin2/plugin"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "payday"
)

// Plugin is the plugin for the payday system used by the bot to manage paydays for guilds.
type Plugin struct {
	status plugin.Status
	name   string
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin creates a new bank plugin.
func NewPlugin(cfgPath string) (*Plugin, error) {
	var err error
	if err = LoadConfig(cfgPath); err != nil {
		return nil, err
	}
	p := &Plugin{
		status: plugin.Running,
		name:   pluginName,
	}
	return p, nil
}

// Initialize initializes the payday plugin.
func (p Plugin) Initialize(mongoDB *database.MongoDB, _ *bot.Client) {
	db = mongoDB
}

// GetHelp returns the help text for the payday plugin.
func (p Plugin) GetHelp() map[string]string {
	return map[string]string{
		"/payday-stats": "View your payday statistics.",
		"/payday":       "Deposits your daily check into your bank account.",
	}
}

// GetName returns the name of the payday plugin.
func (p Plugin) GetName() string {
	return p.name
}

// GetAdminHelp returns the admin help message for the payday plugin.
func (p Plugin) GetAdminHelp() map[string]string {
	return nil
}

// Stop stops the payday plugin.
func (p Plugin) Stop() {
	p.status = plugin.Stopped
}

// Status returns the status of the payday plugin.
func (p Plugin) Status() plugin.Status {
	return p.status
}

// GetSlashHandlers returns the slash command handlers for the payday plugin.
func (p Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/payday":       payday,
		"/payday-stats": showStats,
	}
}

// GetSlashCommands returns the slash commands for the payday plugin.
func (p Plugin) GetSlashCommands() []discord.ApplicationCommandCreate {
	return memberCommands
}

// GetComponentHandlers returns the component handlers for the bank plugin.
func (p *Plugin) GetComponentHandlers() map[string]handler.ComponentHandler {
	return nil
}
