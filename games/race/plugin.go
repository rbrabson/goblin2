package race

import (
	"goblin2/database"
	"goblin2/plugin"
	"sync"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "race"
)

var (
	currentPlugin *Plugin
)

// Plugin is the plugin implementation for the race package.
type Plugin struct {
	status plugin.Status
	name   string
	mutex  sync.RWMutex
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

	p := &Plugin{
		status: plugin.Running,
		name:   pluginName,
	}
	currentPlugin = p

	return p, nil
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

// Stop stops the race plugin after any active races complete.
func (p *Plugin) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if activeRaceCount() == 0 {
		p.status = plugin.Stopped
		return
	}

	p.status = plugin.Stopping
}

// Status returns the plugin status.
func (p *Plugin) Status() plugin.Status {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.status
}

// stopIfIdle stops the race plugin if it is in the stopping state and there are no active races.
func (p *Plugin) stopIfIdle() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.status == plugin.Stopping && activeRaceCount() == 0 {
		p.status = plugin.Stopped
	}
}

// GetSlashHandlers returns slash command handlers for the race plugin.
func (p *Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/race/start":       startRaceHandler,
		"/race/stats":       raceStatsHandler,
		"/race-admin/reset": resetRaceHandler,
	}
}

// GetEventListeners returns the gateway event listeners for the race plugin.
func (p *Plugin) GetEventListeners() []bot.EventListener {
	return nil
}

// GetComponentHandlers returns component handlers for the race plugin.
func (p *Plugin) GetComponentHandlers() map[string]handler.ComponentHandler {
	return map[string]handler.ComponentHandler{
		"/race/join": joinRaceButtonHandler,
		"/race/bet":  betOnRaceButtonHandler,
	}
}

// GetSlashCommands returns the slash commands for the race plugin.
func (p *Plugin) GetSlashCommands() []discord.ApplicationCommandCreate {
	commands := make([]discord.ApplicationCommandCreate, 0, len(adminCommands)+len(memberCommands))
	commands = append(commands, adminCommands...)
	commands = append(commands, memberCommands...)
	return commands
}
