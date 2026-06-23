package disgobot

import (
	"context"
	"goblin2/database"
	"goblin2/plugin"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/handler"
)

var (
	goblin *Bot
)

// Bot represents the main structure of the Goblin Discord Bot. It can hold any necessary state or configuration for the bot's operation.
type Bot struct {
	cfg     *Config
	version string
	client  *bot.Client
	router  *handler.Mux
	plugins []plugin.Plugin
	mutex   sync.RWMutex
}

// NewBot creates a new Goblin Bot instance.
func NewBot(cfg *Config, version string) *Bot {
	goblin = &Bot{
		cfg:     cfg,
		version: version,
		plugins: make([]plugin.Plugin, 0, 10),
	}
	return goblin
}

// Start starts the Goblin Bot.
func (b *Bot) Start(mongoDB *database.MongoDB) error {
	slog.Info("starting bot", slog.String("disgo version", disgo.Version))
	if b.cfg.Token == "" {
		return ErrTokenRequired
	}
	if mongoDB == nil {
		return ErrMongoDBRequired
	}
	db = mongoDB

	client, err := disgo.New(b.cfg.Token,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuilds,
				//gateway.IntentGuildMembers,
				gateway.IntentDirectMessages,
			),
		),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagGuilds)),
		bot.WithEventListenerFunc(func(e *events.Ready) {
			slog.Info("bot is connected as " + e.User.Username)
		}),
		bot.WithEventListeners(b.getEventListeners()),
		bot.WithEventListeners(b.getPluginEventListeners()...),
	)
	if err != nil {
		slog.Error("error while building disgo", slog.Any("err", err))
		return ErrClientNotCreated
	}
	b.client = client

	for _, p := range b.plugins {
		p.Initialize(db, b.client)
	}

	if err := b.connectToGateway(); err != nil {
		return err
	}

	if err := b.syncCommands(); err != nil {
		return err
	}

	return nil
}

// IsStopping returns true if the bot is shutting down.
func (b *Bot) IsStopping() bool {
	status := b.getStatus()
	shuttingDown := status != plugin.Running

	return shuttingDown
}

// GetPlugins returns the plugins that are registered to the bot.
func (b *Bot) GetPlugins() []plugin.Plugin {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return append([]plugin.Plugin(nil), b.plugins...)
}

// RegisterPlugin registers a plugin to the bot.
func (b *Bot) RegisterPlugin(p plugin.Plugin) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if p == nil {
		slog.Error("bot cannot register nil plugin")
		panic("bot cannot register nil plugin")
	}

	b.plugins = append(b.plugins, p)
}

// Stop shuts down the bot.
func (b *Bot) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	b.client.Close(ctx)
	slog.Info("bot stopped")
}

// connectToGateway connects to the Discord gateway.
func (b *Bot) connectToGateway() error {
	// Connect to the gateway
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := b.client.OpenGateway(ctx); err != nil {
		return err
	}
	return nil
}

func (b *Bot) getEventListeners() *handler.Mux {
	h := handler.New()
	b.router = h

	// Basic help commands; it consolidates the help from all plugins
	h.SlashCommand("/help", helpHandler(b))
	h.SlashCommand("/admin-help", adminHelpHandler(b))
	h.SlashCommand("/version", versionHandler(b))

	// Server commands; intended for the bot owner, not bot admins
	h.SlashCommand("/server/shutdown", serverShutdownHandler)
	h.SlashCommand("/server/status", serverStatusHandler)
	h.SlashCommand("/server/owner/add", serverOwnerAddHandler)
	h.SlashCommand("/server/owner/remove", serverOwnerRemoveHandler)
	h.SlashCommand("/server/owner/list", serverOwnerListHandler)
	h.SlashCommand("/server/admin/add", serverAdminAddHandler)
	h.SlashCommand("/server/admin/remove", serverAdminRemoveHandler)
	h.SlashCommand("/server/admin/list", serverAdminListHandler)
	h.SlashCommand("/server/log", serverLogHandler)

	for _, p := range b.plugins {
		for path, fn := range p.GetSlashHandlers() {
			h.SlashCommand(path, fn)
		}
		for path, fn := range p.GetComponentHandlers() {
			h.Component(path, fn)
		}
	}

	return h
}

// getPluginEventListeners returns the gateway event listeners registered by plugins.
func (b *Bot) getPluginEventListeners() []bot.EventListener {
	listeners := make([]bot.EventListener, 0)

	for _, p := range b.plugins {
		listeners = append(listeners, p.GetEventListeners()...)
	}

	return listeners
}

// syncCommands syncs the commands with the Discord API.
func (b *Bot) syncCommands() error {
	allCommands := make([]discord.ApplicationCommandCreate, 0)
	allCommands = append(allCommands, helpCommands...)
	allCommands = append(allCommands, serverCommands...)

	for _, p := range b.plugins {
		allCommands = append(allCommands, p.GetSlashCommands()...)
	}

	if err := handler.SyncCommands(b.client, allCommands, b.cfg.DevGuilds); err != nil {
		return err
	}
	return nil
}

// getStatus returns the status of the bot.
func (b *Bot) getStatus() plugin.Status {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	botStatus := plugin.Running
	for _, p := range b.plugins {
		switch p.Status() {
		case plugin.Stopped:
			botStatus = plugin.Stopped
		case plugin.Stopping:
			return plugin.Stopping
		default:
			// NO-OP
		}
	}
	return botStatus
}
