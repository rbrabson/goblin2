package stats

import (
	"goblin2/database"
	"goblin2/plugin"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "stats"
)

// Plugin is the plugin for the stats system used by the bot
type Plugin struct {
	status plugin.Status
	name   string
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin creates a new instance of the stats plugin
func NewPlugin(_ string) (*Plugin, error) {
	p := &Plugin{
		status: plugin.Running,
		name:   pluginName,
	}
	return p, nil
}

// Initialize initializes the stats plugin
func (p Plugin) Initialize(mongoDB *database.MongoDB, _ *bot.Client) {
	db = mongoDB
}

// GetHelp returns the help text for the stats plugin
func (p *Plugin) GetHelp() map[string]string {
	return nil
}

// GetName returns the name of the stats plugin
func (p *Plugin) GetName() string {
	return p.name
}

// GetAdminHelp returns the admin help text for the stats plugin
func (p *Plugin) GetAdminHelp() map[string]string {
	return nil
}

// Stop stops the stats plugin
func (p *Plugin) Stop() {
	p.status = plugin.Stopped
}

// Status returns the status of the stats plugin
func (p *Plugin) Status() plugin.Status {
	return p.status
}

// GetSlashHandlers returns the slash command handlers for the stats plugin
func (p *Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/stats-admin/retention": statsAdmin,
		"/stats-admin/played":    statsAdmin,
		"/stats/played":          stats,
		"/stats/active":          stats,
	}
}

// GetSlashCommands returns the slash commands for the stats plugin
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
