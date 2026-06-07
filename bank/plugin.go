package bank

import (
	"goblin2/database"
	"goblin2/plugin"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "bank"
)

// Plugin is the plugin implementation for the bank package.
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

// Initialize initializes the bank plugin.
func (p *Plugin) Initialize(mongoDB *database.MongoDB, _ *bot.Client) {
	db = mongoDB
}

// GetName returns the name of the bank plugin.
func (p *Plugin) GetName() string {
	return p.name
}

// GetHelp returns the help text for the bank plugin.
func (p *Plugin) GetHelp() map[string]string {
	return map[string]string{
		"/bank account": "Shows a member's bank account balance.",
	}
}

// GetAdminHelp returns the admin help text for the bank plugin.
func (p *Plugin) GetAdminHelp() map[string]string {
	return map[string]string{
		"/bank-admin account":  "Sets a member's bank account balance.",
		"/bank-admin add":      "Adds credits to a member's bank account.",
		"/bank-admin balance":  "Sets the default bank balance.",
		"/bank-admin name":     "Sets the bank name.",
		"/bank-admin currency": "Sets the bank currency.",
		"/bank-admin info":     "Shows bank configuration information.",
	}
}

// GetSlashHandlers returns the slash command handlers for the bank plugin.
func (p *Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/bank/account":        accountHandler,
		"/bank-admin/account":  setAccountBalanceHandler,
		"/bank-admin/add":      addAccountBalanceHandler,
		"/bank-admin/balance":  setDefaultBalanceHandler,
		"/bank-admin/name":     setBankNameHandler,
		"/bank-admin/currency": setBankCurrencyHandler,
		"/bank-admin/info":     getBankInfoHandler,
	}
}

// GetSlashCommands returns the slash commands for the bank plugin.
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

// Stop stops the bank plugin.
func (p *Plugin) Stop() {
	p.status = plugin.Stopped
}

// Status returns the status of the bank plugin.
func (p *Plugin) Status() plugin.Status {
	return p.status
}
