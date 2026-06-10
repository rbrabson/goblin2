package heist

import (
	"goblin2/database"
	"goblin2/plugin"
	"sync"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "heist"
)

var (
	currentPlugin *Plugin
)

// Plugin is the plugin implementation for the heist package.
type Plugin struct {
	status plugin.Status
	name   string
	mutex  sync.RWMutex
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
	currentPlugin = p
	return p, nil
}

// Initialize initializes the heist plugin.
func (p *Plugin) Initialize(mongoDB *database.MongoDB, _ *bot.Client) {
	db = mongoDB
	go vaultUpdater()
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

// Stop stops the heist plugin after any active heists complete.
func (p *Plugin) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if activeHeistCount() == 0 {
		p.status = plugin.Stopped
		return
	}

	p.status = plugin.Stopping
}

// Status returns the status of the heist plugin.
func (p *Plugin) Status() plugin.Status {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.status
}

func (p *Plugin) stopIfIdle() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.status == plugin.Stopping && activeHeistCount() == 0 {
		p.status = plugin.Stopped
	}
}

// GetSlashHandlers returns the slash command handlers for the heist plugin.
func (p *Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/heist/bail":    bailoutPlayerHandler,
		"/heist/stats":   playerStatsHandler,
		"/heist/start":   startHeistHandler,
		"/heist/targets": listTargetsHandler,

		"/heist-admin/boost":                       enableBoostHandler,
		"/heist-admin/clear":                       clearMemberHandler,
		"/heist-admin/config/info":                 configInfoHandler,
		"/heist-admin/config/bail":                 configBailHandler,
		"/heist-admin/config/boost":                configBoostHandler,
		"/heist-admin/config/cost":                 configCostHandler,
		"/heist-admin/config/death":                configDeathHandler,
		"/heist-admin/config/patrol":               configPatrolHandler,
		"/heist-admin/config/sentence":             configSentenceHandler,
		"/heist-admin/config/boost-vault-recovery": configBoostVaultRecoveryHandler,
		"/heist-admin/config/wait":                 configWaitHandler,
		"/heist-admin/reset":                       resetHeistHandler,
		"/heist-admin/vault-reset":                 resetVaultsHandler,
	}
}

func (p *Plugin) GetComponentHandlers() map[string]handler.ComponentHandler {
	return map[string]handler.ComponentHandler{
		"/heist/join": joinHeistButtonHandler,
	}
}

// GetSlashCommands returns the slash commands for the heist plugin.
func (p *Plugin) GetSlashCommands() []discord.ApplicationCommandCreate {
	commands := make([]discord.ApplicationCommandCreate, 0, len(adminCommands)+len(memberCommands))
	commands = append(commands, adminCommands...)
	commands = append(commands, memberCommands...)
	return commands
}
