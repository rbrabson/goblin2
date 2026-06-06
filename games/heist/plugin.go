package heist

import (
	"goblin2/database"
	"goblin2/plugin"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "heist"
)

// Plugin is the plugin implementation for the heist package.
type Plugin struct {
	status plugin.Status
	name   string
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin creates a new heist plugin.
func NewPlugin(cfgPath string) (*Plugin, error) {
	if err := LoadConfig(cfgPath); err != nil {
		return nil, err
	}
	if err := LoadTheme(cfgPath); err != nil {
		return nil, err
	}
	if err := LoadTargets(cfgPath); err != nil {
		return nil, err
	}

	p := &Plugin{
		status: plugin.Running,
		name:   pluginName,
	}
	return p, nil
}

// Initialize initializes the heist plugin.
func (p *Plugin) Initialize(mongoDB *database.MongoDB, _ *bot.Client) {
	db = mongoDB
}

// GetHelp returns the help text for the heist plugin.
func (p *Plugin) GetHelp() map[string]string {
	return map[string]string{
		"/heist bail":    "Bail a player out of jail.",
		"/heist stats":   "Shows your heist statistics.",
		"/heist start":   "Plans a new heist.",
		"/heist targets": "Gets the list of available heist targets.",
	}
}

// GetName returns the name of the heist plugin.
func (p *Plugin) GetName() string {
	return p.name
}

// GetAdminHelp returns the admin help text for the heist plugin.
func (p *Plugin) GetAdminHelp() map[string]string {
	return map[string]string{
		"/heist-admin boost":                       "Enables or disables boosts for the heist game.",
		"/heist-admin clear":                       "Clears the criminal settings for a member.",
		"/heist-admin config info":                 "Returns the configuration information for the server.",
		"/heist-admin config bail":                 "Sets the base cost of bail.",
		"/heist-admin config boost":                "Sets the boost percentage.",
		"/heist-admin config cost":                 "Sets the cost to plan or join a heist.",
		"/heist-admin config death":                "Sets how long players remain dead.",
		"/heist-admin config patrol":               "Sets the time the authorities prevent a new heist.",
		"/heist-admin config sentence":             "Sets the base apprehension time when caught.",
		"/heist-admin config boost-vault-recovery": "Sets the boosted vault recovery percentage.",
		"/heist-admin config wait":                 "Sets how long players can gather others for a heist.",
		"/heist-admin reset":                       "Resets a heist that is hung.",
		"/heist-admin vault-reset":                 "Resets the vaults to their maximum value.",
	}
}

// Stop stops the heist plugin.
func (p *Plugin) Stop() {
	p.status = plugin.Stopped
}

// Status returns the status of the heist plugin.
func (p *Plugin) Status() plugin.Status {
	return p.status
}

// GetSlashHandlers returns the slash command handlers for the heist plugin.
func (p *Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/heist/bail":    bailoutPlayer,
		"/heist/stats":   playerStats,
		"/heist/start":   startHeist,
		"/heist/targets": listTargets,

		"/heist-admin/boost":                       enableBoost,
		"/heist-admin/clear":                       clearMember,
		"/heist-admin/config/info":                 configInfo,
		"/heist-admin/config/bail":                 configBail,
		"/heist-admin/config/boost":                configBoost,
		"/heist-admin/config/cost":                 configCost,
		"/heist-admin/config/death":                configDeath,
		"/heist-admin/config/patrol":               configPatrol,
		"/heist-admin/config/sentence":             configSentence,
		"/heist-admin/config/boost-vault-recovery": configBoostVaultRecovery,
		"/heist-admin/config/wait":                 configWait,
		"/heist-admin/reset":                       resetHeist,
		"/heist-admin/vault-reset":                 resetVaults,
	}
}

func (p *Plugin) GetComponentHandlers() map[string]handler.ComponentHandler {
	return map[string]handler.ComponentHandler{
		"/heist/join": joinHeist,
	}
}

// GetSlashCommands returns the slash commands for the heist plugin.
func (p *Plugin) GetSlashCommands() []discord.ApplicationCommandCreate {
	commands := make([]discord.ApplicationCommandCreate, 0, len(adminCommands)+len(memberCommands))
	commands = append(commands, adminCommands...)
	commands = append(commands, memberCommands...)
	return commands
}
