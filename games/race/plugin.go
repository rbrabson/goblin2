package race

import (
	"goblin2/database"
	"goblin2/plugin"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "race"
)

// Plugin is the plugin implementation for the race package.
type Plugin struct {
	status plugin.Status
	name   string
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin creates a new race plugin.
func NewPlugin(cfgPath string) (*Plugin, error) {
	if err := LoadConfig(cfgPath); err != nil {
		return nil, err
	}
	if err := LoadAvatars(cfgPath); err != nil {
		return nil, err
	}

	return &Plugin{
		status: plugin.Running,
		name:   pluginName,
	}, nil
}

// Initialize initializes the race plugin.
func (p *Plugin) Initialize(mongoDB *database.MongoDB, _ *bot.Client) {
	db = mongoDB
}

// GetHelp returns the help text for the race plugin.
func (p *Plugin) GetHelp() map[string]string {
	return map[string]string{
		"/race start": "Starts a new race.",
		"/race stats": "Shows your race statistics.",
	}
}

// GetName returns the plugin name.
func (p *Plugin) GetName() string {
	return p.name
}

// GetAdminHelp returns the admin help text for the race plugin.
func (p *Plugin) GetAdminHelp() map[string]string {
	return map[string]string{
		"/race-admin reset": "Resets a hung race.",
	}
}

// Stop stops the race plugin.
func (p *Plugin) Stop() {
	p.status = plugin.Stopped
}

// Status returns the plugin status.
func (p *Plugin) Status() plugin.Status {
	return p.status
}

// GetSlashHandlers returns slash command handlers for the race plugin.
func (p *Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/race/start":       startRace,
		"/race/stats":       raceStats,
		"/race-admin/reset": resetRace,
	}
}

// GetComponentHandlers returns component handlers for the race plugin.
func (p *Plugin) GetComponentHandlers() map[string]handler.ComponentHandler {
	return map[string]handler.ComponentHandler{
		"/race/join": joinRace,
		"/race/bet":  betOnRace,
	}
}

// GetSlashCommands returns the slash commands for the race plugin.
func (p *Plugin) GetSlashCommands() []discord.ApplicationCommandCreate {
	commands := make([]discord.ApplicationCommandCreate, 0, len(adminCommands)+len(memberCommands))
	commands = append(commands, adminCommands...)
	commands = append(commands, memberCommands...)
	return commands
}
