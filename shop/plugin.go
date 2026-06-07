package shop

import (
	"goblin2/database"
	"goblin2/plugin"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pluginName = "shop"
)

var (
	client *bot.Client
)

// Plugin is the plugin implementation for the shop package.
type Plugin struct {
	status plugin.Status
	name   string
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin creates a new shop plugin.
func NewPlugin(_ string) (*Plugin, error) {
	p := &Plugin{
		status: plugin.Running,
		name:   pluginName,
	}
	return p, nil
}

// Initialize initializes the shop plugin.
func (p *Plugin) Initialize(mongoDB *database.MongoDB, botClient *bot.Client) {
	db = mongoDB
	client = botClient

	go checkForExpiredPurchases()
}

// GetHelp returns the help text for the shop plugin.
func (p *Plugin) GetHelp() map[string]string {
	return map[string]string{
		"/shop purchases": "Lists the items you have purchased.",
	}
}

// GetName returns the name of the shop plugin.
func (p *Plugin) GetName() string {
	return p.name
}

// GetAdminHelp returns the admin help text for the shop plugin.
func (p *Plugin) GetAdminHelp() map[string]string {
	return map[string]string{
		"/shop-admin add":             "Adds an item to the shop that may be purchased by a member.",
		"/shop-admin ban":             "Bans a member from the shop.",
		"/shop-admin channel":         "Sets the channel to which to publish the shop items.",
		"/shop-admin delete":          "Removes a purchasable item from the shop.",
		"/shop-admin list-bans":       "Lists the users banned from the shop.",
		"/shop-admin mod-channel":     "Sets the channel to which to publish notices.",
		"/shop-admin notification-id": "Sets the member to which to notify.",
		"/shop-admin publish":         "Publishes the shop items in the shop channel.",
		"/shop-admin unban":           "Removes the ban of a member from the shop.",
	}
}

// Stop stops the shop plugin.
func (p *Plugin) Stop() {
	p.status = plugin.Stopped
}

// Status returns the status of the shop plugin.
func (p *Plugin) Status() plugin.Status {
	return p.status
}

// GetSlashHandlers returns the slash command handlers for the shop plugin.
func (p *Plugin) GetSlashHandlers() map[string]handler.SlashCommandHandler {
	return map[string]handler.SlashCommandHandler{
		"/shop/purchases":         purchasesHandler,
		"/shop-admin/add-role":    addRoleHandler,
		"/shop-admin/remove-role": removeRoleHandler,
		"/shop-admin/channel":     setChannelHandler,
		"/shop-admin/mod-channel": setModChannelHandler,
		"/shop-admin/info":        getShopInfoHandler,
	}
}

// GetSlashCommands returns the slash commands for the shop plugin.
func (p *Plugin) GetSlashCommands() []discord.ApplicationCommandCreate {
	commands := make([]discord.ApplicationCommandCreate, 0, len(adminCommands)+len(memberCommands))
	commands = append(commands, adminCommands...)
	commands = append(commands, memberCommands...)
	return commands
}

// GetComponentHandlers returns the component handlers for the shop plugin.
func (p *Plugin) GetComponentHandlers() map[string]handler.ComponentHandler {
	return map[string]handler.ComponentHandler{
		shopBuyRoleComponentRoute: buyRoleButtonHandler,
	}
}
