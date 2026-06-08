package blackjack

import (
	"goblin2/database"
	"goblin2/plugin"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "blackjack"
)

// Plugin is the plugin implementation for the blackjack package.
type Plugin struct {
	status plugin.Status
	name   string
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin creates a new blackjack plugin.
func NewPlugin(cfgPath string) (*Plugin, error) {
	if err := LoadConfig(cfgPath); err != nil {
		return nil, err
	}
	if err := LoadCards(cfgPath); err != nil {
		return nil, err
	}

	return &Plugin{
		status: plugin.Running,
		name:   pluginName,
	}, nil
}

// Initialize initializes the blackjack plugin.
func (p *Plugin) Initialize(mongoDB *database.MongoDB, _ *bot.Client) {
	db = mongoDB
}

// GetHelp returns blackjack help.
func (p *Plugin) GetHelp() map[string]string {
	return map[string]string{
		"/blackjack play":  "Starts a new blackjack game.",
		"/blackjack stats": "Shows your blackjack statistics.",
	}
}

// GetName returns the plugin name.
func (p *Plugin) GetName() string {
	return p.name
}

// GetAdminHelp returns blackjack admin help.
func (p *Plugin) GetAdminHelp() map[string]string {
	return map[string]string{
		"/blackjack-admin config info":          "Shows the blackjack configuration.",
		"/blackjack-admin config bet":           "Sets the blackjack bet amount.",
		"/blackjack-admin config payout":        "Sets the blackjack payout percentage.",
		"/blackjack-admin config single-player": "Enables or disables single-player blackjack mode.",
	}
}

// Stop stops the blackjack plugin.
func (p *Plugin) Stop() {
	p.status = plugin.Stopped
}

// Status returns the plugin status.
func (p *Plugin) Status() plugin.Status {
	return p.status
}

// GetSlashHandlers returns slash command handlers for blackjack.
func (p *Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/blackjack/play":                       playBlackjackHandler,
		"/blackjack/stats":                      blackjackStatsHandler,
		"/blackjack-admin/config/info":          configInfoHandler,
		"/blackjack-admin/config/bet":           configBetAmountHandler,
		"/blackjack-admin/config/payout":        configPayoutPercentHandler,
		"/blackjack-admin/config/single-player": configSinglePlayerHandler,
	}
}

// GetComponentHandlers returns component handlers for blackjack.
func (p *Plugin) GetComponentHandlers() map[string]handler.ComponentHandler {
	return map[string]handler.ComponentHandler{
		"/blackjack/join":        blackjackJoinButtonHandler,
		"/blackjack/hit":         blackjackHitButtonHandler,
		"/blackjack/stand":       blackjackStandButtonHandler,
		"/blackjack/double-down": blackjackDoubleDownButtonHandler,
		"/blackjack/split":       blackjackSplitButtonHandler,
		"/blackjack/surrender":   blackjackSurrenderButtonHandler,
	}
}

// GetSlashCommands returns blackjack slash commands.
func (p *Plugin) GetSlashCommands() []discord.ApplicationCommandCreate {
	commands := make([]discord.ApplicationCommandCreate, 0, len(adminCommands)+len(memberCommands))
	commands = append(commands, adminCommands...)
	commands = append(commands, memberCommands...)
	return commands
}
