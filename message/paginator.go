package message

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
)

// Paginator represents a paginator for creating paginated Discord messages.
type Paginator struct {
	id       string
	config   *Config
	messages map[string]*message
	mutex    sync.Mutex
	manager  *paginatorManager
}

// NewPaginator creates a new Paginator with the given options.
func NewPaginator(opts ...ConfigOpt) *Paginator {
	cfg := GetDefaultConfig()
	cfg.Apply(opts)

	p := &Paginator{
		id:       fmt.Sprintf("%s-%d", cfg.CustomIDPrefix, time.Now().UnixNano()),
		config:   cfg,
		messages: make(map[string]*message),
		mutex:    sync.Mutex{},
		manager:  manager,
	}

	p.manager.addPaginator(p)

	slog.Debug("created new paginator",
		slog.String("paginator", p.id),
		slog.Int("itemsPerPage", p.config.ItemsPerPage),
		slog.Duration("idleWait", p.config.IdleWait),
	)

	return p
}

// CreateInteractionResponse creates and sends a paginated message as an interaction response.
func (p *Paginator) CreateInteractionResponse(e *handler.CommandEvent, title string, embedFields []discord.EmbedField, ephemeral ...bool) error {
	m := newMessage(p, title, embedFields)
	m.id = fmt.Sprintf("%s-%d", e.Channel().ID(), time.Now().UnixNano())
	m.token = e.Token()
	m.ephemeral = len(ephemeral) > 0 && ephemeral[0]
	m.channelID = e.Channel().ID()

	p.mutex.Lock()
	p.messages[m.id] = m
	p.mutex.Unlock()

	embed := m.makeEmbed()
	component := m.makeComponent(false)
	m.registerComponentHandlers()

	var flags discord.MessageFlags
	if m.ephemeral {
		flags = discord.MessageFlagEphemeral
	}

	err := e.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed},
		Components: []discord.LayoutComponent{component},
		Flags:      flags,
	})
	if err != nil {
		slog.Error("error sending paginated interaction response",
			slog.String("paginator", p.id),
			slog.String("message", m.id),
			slog.Any("error", err),
		)

		m.deregisterComponentHandlers()

		p.mutex.Lock()
		delete(p.messages, m.id)
		p.mutex.Unlock()

		return err
	}

	slog.Debug("created paginated interaction response",
		slog.String("paginator", p.id),
		slog.String("message", m.id),
	)

	return nil
}

// CreateMessage creates and sends a paginated message to a channel.
func (p *Paginator) CreateMessage(channelID snowflake.ID, title string, embedFields []discord.EmbedField) error {
	if p.config.DiscordConfig.Client == nil {
		return ErrClientMissing
	}

	m := newMessage(p, title, embedFields)
	m.id = fmt.Sprintf("%s-%d", channelID, time.Now().UnixNano())
	m.channelID = channelID

	p.mutex.Lock()
	p.messages[m.id] = m
	p.mutex.Unlock()

	embed := m.makeEmbed()
	component := m.makeComponent(false)
	m.registerComponentHandlers()

	msg, err := p.config.DiscordConfig.Client.Rest.CreateMessage(channelID, discord.MessageCreate{
		Embeds:     []discord.Embed{embed},
		Components: []discord.LayoutComponent{component},
	})
	if err != nil {
		slog.Error("error sending paginated message",
			slog.String("paginator", p.id),
			slog.String("message", m.id),
			slog.String("channel", m.channelID.String()),
			slog.Any("error", err),
		)

		m.deregisterComponentHandlers()

		p.mutex.Lock()
		delete(p.messages, m.id)
		p.mutex.Unlock()

		return err
	}

	m.messageID = msg.ID

	slog.Debug("created paginated message",
		slog.String("paginator", p.id),
		slog.String("message", m.id),
		slog.String("channel", m.channelID.String()),
	)

	return nil
}

// Close closes the paginator and disables all active paginated messages.
func (p *Paginator) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, m := range p.messages {
		if err := m.disable(); err != nil {
			slog.Error("error disabling paginated message on close",
				slog.String("paginator", p.id),
				slog.String("message", m.id),
				slog.Any("error", err),
			)
		}

		m.deregisterComponentHandlers()
		delete(p.messages, m.id)
	}

	manager.removePaginator(p)
}

// cleanup removes expired paginated messages.
func (p *Paginator) cleanup() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, m := range p.messages {
		if m.hasExpired() {
			if err := m.disable(); err != nil {
				slog.Error("error disabling expired paginated message",
					slog.String("paginator", p.id),
					slog.String("message", m.id),
					slog.Any("error", err),
				)
			}

			m.deregisterComponentHandlers()
			delete(p.messages, m.id)
		}
	}
}
