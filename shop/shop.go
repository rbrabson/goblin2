package shop

import (
	"cmp"
	"fmt"
	"goblin2/internal/discordid"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

const (
	shopBuyRoleComponentRoute = "/shop/buy-role/{roleName}"
	shopBuyRoleComponentPath  = "/shop/buy-role"
)

func shopBuyRoleComponentID(roleName string) string {
	return shopBuyRoleComponentPath + "/" + roleName
}

// Shop is the shop for a guild. The shop contains all items available for purchase.
type Shop struct {
	GuildID string  // Guild (server) for the shop
	Items   []*Item // All items available in the shop
}

// GetShop returns the shop for the guild.
func GetShop(guildID string) *Shop {
	var err error

	shop := &Shop{
		GuildID: guildID,
	}

	shop.Items, err = readShopItems(guildID)
	if err != nil {
		slog.Error("unable to read shop items from the database", "guildID", guildID, "error", err)
		shop.Items = make([]*Item, 0)
	}

	cacheShopItems(shop.Items)

	for i, item := range shop.Items {
		shop.Items[i] = copyItem(item)
	}

	shopItemCmp := func(a, b *Item) int {
		return cmp.Or(
			cmp.Compare(a.Type, b.Type),
			cmp.Compare(a.Name, b.Name),
		)
	}
	slices.SortFunc(shop.Items, shopItemCmp)

	return shop
}

// GetShopItem finds an item in the shop. If the item does not exist, then nil is returned.
func (s *Shop) GetShopItem(name string, itemType string) *Item {
	for _, item := range s.Items {
		if item.Name == name && item.Type == itemType {
			return copyItem(item)
		}
	}

	return nil
}

// Publish publishes the shop to its configured channel.
func (s *Shop) Publish() error {
	if client == nil {
		return fmt.Errorf("discord client is nil")
	}
	if client.Rest == nil {
		return fmt.Errorf("discord REST client is nil")
	}

	guildID, err := discordid.SnowflakeIDFromString(s.GuildID)
	if err != nil {
		return fmt.Errorf("invalid shop guild ID %q: %w", s.GuildID, err)
	}

	config := GetConfig(guildID)
	if config.ChannelID == "" {
		return nil
	}

	channelID, err := strconv.ParseUint(config.ChannelID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid shop channel ID %q: %w", config.ChannelID, err)
	}

	message := s.messageCreate()

	if config.MessageID != "" {
		messageID, err := strconv.ParseUint(config.MessageID, 10, 64)
		if err == nil {
			_, err = client.Rest.UpdateMessage(
				snowflake.ID(channelID),
				snowflake.ID(messageID),
				discord.MessageUpdate{
					Embeds:     &message.Embeds,
					Components: &message.Components,
				},
			)
			if err == nil {
				return nil
			}

			slog.Warn("unable to update existing shop message, creating a new one",
				slog.Any("guildID", guildID),
				slog.String("channelID", config.ChannelID),
				slog.String("messageID", config.MessageID),
				slog.Any("error", err),
			)

			config.SetMessageID("")
		}
	}

	created, err := client.Rest.CreateMessage(snowflake.ID(channelID), message)
	if err != nil {
		return fmt.Errorf("unable to publish shop message: %w", err)
	}

	config.SetMessageID(created.ID.String())

	return nil
}

func (s *Shop) messageCreate() discord.MessageCreate {
	fields := make([]discord.EmbedField, 0, len(s.Items))
	buttons := make([]discord.InteractiveComponent, 0, len(s.Items))

	for _, item := range s.Items {
		fields = append(fields, discord.EmbedField{
			Name:  fmt.Sprintf("%s %s", shopItemDisplayType(item.Type), item.Name),
			Value: formatPublishedShopItem(item),
		})

		if item.Type != roleItemType {
			continue
		}

		buttons = append(buttons, discord.ButtonComponent{
			Label:    fmt.Sprintf("Role: %s", item.Name),
			Style:    discord.ButtonStylePrimary,
			CustomID: shopBuyRoleComponentID(item.Name),
		})
	}

	components := make([]discord.LayoutComponent, 0, 1)
	if len(buttons) > 0 {
		components = append(components, discord.ActionRowComponent{
			Components: buttons,
		})
	}

	return discord.MessageCreate{
		Embeds: []discord.Embed{
			{
				Type:   discord.EmbedTypeRich,
				Title:  "Shop Items",
				Fields: fields,
			},
		},
		Components: components,
	}
}

func formatPublishedShopItem(item *Item) string {
	var parts []string

	if item.Description != "" {
		parts = append(parts, fmt.Sprintf("Description: %s", item.Description))
	}

	parts = append(parts, fmt.Sprintf("Cost: %s", strconv.Itoa(item.Price)))

	if item.Duration != "" {
		parts = append(parts, fmt.Sprintf("Duration: %s", item.Duration))
	}

	return strings.Join(parts, "\n")
}

func shopItemDisplayType(itemType string) string {
	if itemType == "" {
		return ""
	}

	return strings.ToUpper(itemType[:1]) + itemType[1:]
}
