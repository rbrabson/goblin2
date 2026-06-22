package guild

import (
	"goblin2/database"
	"goblin2/plugin"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "guild"
)

var guildPlugin *Plugin

// Plugin is the plugin implementation for the guild package.
type Plugin struct {
	status plugin.Status
	name   string
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin creates a new guild plugin.
func NewPlugin(_ string) (*Plugin, error) {
	p := &Plugin{
		status: plugin.Running,
		name:   pluginName,
	}
	guildPlugin = p
	return p, nil
}

// Initialize initializes the guild plugin.
func (p *Plugin) Initialize(mongoDB *database.MongoDB, client *bot.Client) {
	db = mongoDB
	startRoleSync(client)
}

// GetName returns the name of the guild plugin.
func (p *Plugin) GetName() string {
	return p.name
}

// GetHelp returns the help text for the guild plugin.
func (p *Plugin) GetHelp() map[string]string {
	return nil
}

// GetAdminHelp returns the admin help text for the guild plugin.
func (p *Plugin) GetAdminHelp() map[string]string {
	return map[string]string{
		"/guild-admin role": "Manages the admin roles for the bot for this server.",
	}
}

// GetSlashHandlers returns the slash command handlers for the guild plugin.
func (p *Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/guild-admin/role/list":   guildAdminRoleListHandler,
		"/guild-admin/role/add":    guildAdminRoleAddHandler,
		"/guild-admin/role/remove": guildAdminRoleRemoveHandler,
	}
}

func (p *Plugin) GetSlashCommands() []discord.ApplicationCommandCreate {
	return []discord.ApplicationCommandCreate{
		adminCommands,
	}
}

// GetEventListeners returns the gateway event listeners for the guild plugin.
func (p *Plugin) GetEventListeners() []bot.EventListener {
	return []bot.EventListener{
		bot.NewListenerFunc(guildReadyListener),
		bot.NewListenerFunc(guildJoinListener),
		bot.NewListenerFunc(guildRoleCreateListener),
		bot.NewListenerFunc(guildRoleUpdateListener),
		bot.NewListenerFunc(guildRoleDeleteListener),
	}
}

// GetComponentHandlers returns the component handlers for the bank plugin.
func (p *Plugin) GetComponentHandlers() map[string]handler.ComponentHandler {
	return nil
}

// Stop stops the guild plugin.
func (p *Plugin) Stop() {
	p.status = plugin.Stopped
}

// Status returns the status of the guild plugin.
func (p *Plugin) Status() plugin.Status {
	return p.status
}
