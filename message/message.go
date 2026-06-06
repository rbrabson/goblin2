package message

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
)

// message represents a single paginated message.
type message struct {
	id          string
	title       string
	embedFields []discord.EmbedField
	expiry      time.Time
	currentPage int
	channelID   snowflake.ID
	paginator   *Paginator
	token       string // interaction token for editing via webhook
	messageID   snowflake.ID
	ephemeral   bool
}

// newMessage creates a new message for the paginator.
func newMessage(p *Paginator, title string, embedFields []discord.EmbedField) *message {
	return &message{
		paginator:   p,
		title:       title,
		embedFields: embedFields,
		expiry:      time.Now().Add(p.config.IdleWait),
	}
}

// editMessage edits the current paginated message in response to a button press.
func (m *message) editMessage(e *handler.ComponentEvent) error {
	embed := m.makeEmbed()
	component := m.makeComponent(false)

	// Acknowledge the button press with a deferred update.
	if err := e.DeferUpdateMessage(); err != nil {
		slog.Error("error deferring paginated message update",
			slog.String("paginator", m.id),
			slog.String("channel", m.channelID.String()),
			slog.Any("error", err),
		)
	}

	var err error
	if m.token != "" {
		_, err = e.Client().Rest.UpdateInteractionResponse(
			e.ApplicationID(),
			m.token,
			discord.MessageUpdate{
				Embeds:     &[]discord.Embed{embed},
				Components: &[]discord.LayoutComponent{component},
			},
		)
	} else {
		_, err = e.Client().Rest.UpdateMessage(
			m.channelID,
			m.messageID,
			discord.MessageUpdate{
				Embeds:     &[]discord.Embed{embed},
				Components: &[]discord.LayoutComponent{component},
			},
		)
	}
	if err != nil {
		slog.Error("error editing paginated message",
			slog.String("paginator", m.id),
			slog.String("channel", m.channelID.String()),
			slog.Any("error", err),
		)
		return err
	}

	slog.Debug("edited paginated message",
		slog.String("paginator", m.id),
		slog.String("channel", m.channelID.String()),
	)
	return nil
}

// disable disables the message by disabling all buttons.
func (m *message) disable() error {
	embed := m.makeEmbed()
	component := m.makeComponent(true)

	client := m.paginator.config.DiscordConfig.Client
	if client == nil {
		slog.Error("cannot disable paginated message without discord client",
			slog.String("paginator", m.id),
			slog.String("channel", m.channelID.String()),
		)
		return nil
	}

	var err error
	if m.token != "" {
		_, err = client.Rest.UpdateInteractionResponse(
			client.ApplicationID,
			m.token,
			discord.MessageUpdate{
				Embeds:     &[]discord.Embed{embed},
				Components: &[]discord.LayoutComponent{component},
			},
		)
	} else {
		_, err = client.Rest.UpdateMessage(
			m.channelID,
			m.messageID,
			discord.MessageUpdate{
				Embeds:     &[]discord.Embed{embed},
				Components: &[]discord.LayoutComponent{component},
			},
		)
	}
	if err != nil {
		slog.Error("error disabling paginated message",
			slog.String("paginator", m.id),
			slog.String("channel", m.channelID.String()),
			slog.Any("error", err),
		)
		return err
	}

	slog.Debug("disabled paginated message",
		slog.String("paginator", m.id),
		slog.String("channel", m.channelID.String()),
	)
	return nil
}

// pageCount returns the number of pages.
func (m *message) pageCount() int {
	itemsPerPage := m.paginator.config.ItemsPerPage
	if itemsPerPage <= 0 {
		return 1
	}

	pageCount := (len(m.embedFields) + itemsPerPage - 1) / itemsPerPage
	if pageCount == 0 {
		return 1
	}

	return pageCount
}

// makeEmbed creates the embed for the current page.
func (m *message) makeEmbed() discord.Embed {
	itemsPerPage := m.paginator.config.ItemsPerPage
	if itemsPerPage <= 0 {
		itemsPerPage = len(m.embedFields)
		if itemsPerPage == 0 {
			itemsPerPage = 1
		}
	}

	start := m.currentPage * itemsPerPage
	if start > len(m.embedFields) {
		start = len(m.embedFields)
	}

	end := min(start+itemsPerPage, len(m.embedFields))

	return discord.Embed{
		Color:  m.paginator.config.EmbedColor,
		Title:  m.title,
		Fields: m.embedFields[start:end],
		Footer: &discord.EmbedFooter{
			Text: fmt.Sprintf("Page %d of %d", m.currentPage+1, m.pageCount()),
		},
	}
}

// makeComponent creates the action row with navigation buttons.
func (m *message) makeComponent(disabled bool) discord.LayoutComponent {
	cfg := m.paginator.config.ButtonsConfig
	buttons := make([]discord.InteractiveComponent, 0, 5)

	atFirst := m.currentPage == 0
	atLast := m.currentPage == m.pageCount()-1

	if cfg.First != nil {
		buttons = append(buttons, discord.ButtonComponent{
			Label:    cfg.First.Label,
			Style:    cfg.First.Style,
			Disabled: disabled || atFirst,
			Emoji:    cfg.First.Emoji,
			CustomID: m.customButtonID("first"),
		})
	}
	if cfg.Back != nil {
		buttons = append(buttons, discord.ButtonComponent{
			Label:    cfg.Back.Label,
			Style:    cfg.Back.Style,
			Disabled: disabled || atFirst,
			Emoji:    cfg.Back.Emoji,
			CustomID: m.customButtonID("back"),
		})
	}
	if cfg.Stop != nil {
		buttons = append(buttons, discord.ButtonComponent{
			Label:    cfg.Stop.Label,
			Style:    cfg.Stop.Style,
			Disabled: disabled,
			Emoji:    cfg.Stop.Emoji,
			CustomID: m.customButtonID("stop"),
		})
	}
	if cfg.Next != nil {
		buttons = append(buttons, discord.ButtonComponent{
			Label:    cfg.Next.Label,
			Style:    cfg.Next.Style,
			Disabled: disabled || atLast,
			Emoji:    cfg.Next.Emoji,
			CustomID: m.customButtonID("next"),
		})
	}
	if cfg.Last != nil {
		buttons = append(buttons, discord.ButtonComponent{
			Label:    cfg.Last.Label,
			Style:    cfg.Last.Style,
			Disabled: disabled || atLast,
			Emoji:    cfg.Last.Emoji,
			CustomID: m.customButtonID("last"),
		})
	}

	return discord.ActionRowComponent{Components: buttons}
}

// registerComponentHandlers registers button handlers with the disgo router.
func (m *message) registerComponentHandlers() {
	cfg := m.paginator.config
	addHandler := cfg.DiscordConfig.AddComponentHandler
	if addHandler == nil {
		return
	}

	if cfg.ButtonsConfig.First != nil {
		addHandler(m.customButtonID("first"), pageResponse)
	}
	if cfg.ButtonsConfig.Back != nil {
		addHandler(m.customButtonID("back"), pageResponse)
	}
	if cfg.ButtonsConfig.Stop != nil {
		addHandler(m.customButtonID("stop"), pageResponse)
	}
	if cfg.ButtonsConfig.Next != nil {
		addHandler(m.customButtonID("next"), pageResponse)
	}
	if cfg.ButtonsConfig.Last != nil {
		addHandler(m.customButtonID("last"), pageResponse)
	}
}

// deregisterComponentHandlers removes button handlers from the disgo router.
func (m *message) deregisterComponentHandlers() {
	cfg := m.paginator.config
	removeHandler := cfg.DiscordConfig.RemoveComponentHandler
	if removeHandler == nil {
		return
	}

	if cfg.ButtonsConfig.First != nil {
		removeHandler(m.customButtonID("first"))
	}
	if cfg.ButtonsConfig.Back != nil {
		removeHandler(m.customButtonID("back"))
	}
	if cfg.ButtonsConfig.Stop != nil {
		removeHandler(m.customButtonID("stop"))
	}
	if cfg.ButtonsConfig.Next != nil {
		removeHandler(m.customButtonID("next"))
	}
	if cfg.ButtonsConfig.Last != nil {
		removeHandler(m.customButtonID("last"))
	}
}

// hasExpired returns true if the message has passed its idle timeout.
func (m *message) hasExpired() bool {
	return !m.expiry.IsZero() && m.expiry.Before(time.Now())
}

// pageResponse handles a button press on a paginated message.
func pageResponse(e *handler.ComponentEvent) error {
	ids := strings.Split(strings.TrimPrefix(e.Data.CustomID(), "/"), "/")
	if len(ids) != 3 {
		return nil
	}
	paginatorID, messageID, action := ids[0], ids[1], ids[2]

	manager.mutex.Lock()
	paginator, ok := manager.paginators[paginatorID]
	manager.mutex.Unlock()
	if !ok {
		slog.Error("paginator not found", slog.String("paginator", paginatorID))
		return nil
	}

	paginator.mutex.Lock()
	defer paginator.mutex.Unlock()

	m, ok := paginator.messages[messageID]
	if !ok {
		return nil
	}

	switch action {
	case "first":
		m.currentPage = 0
	case "back":
		if m.currentPage > 0 {
			m.currentPage--
		}
	case "next":
		if m.currentPage < m.pageCount()-1 {
			m.currentPage++
		}
	case "last":
		m.currentPage = m.pageCount() - 1
	case "stop":
		m.deregisterComponentHandlers()
		delete(paginator.messages, m.id)
		return m.disable()
	}

	m.expiry = time.Now().Add(m.paginator.config.IdleWait)
	return m.editMessage(e)
}

// customButtonID returns the unique custom ID for a navigation button.
func (m *message) customButtonID(action string) string {
	return fmt.Sprintf("/%s/%s/%s", m.paginator.id, m.id, action)
}
