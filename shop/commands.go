package shop

import (
	"fmt"
	"goblin2/disgobot"
	"goblin2/internal/discordid"
	"goblin2/internal/message"
	"log/slog"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var (
	adminCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "shop-admin",
			Description: "Commands used to configure the shop for this server.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "add-role",
					Description: "Adds a Discord role to the shop.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "name",
							Description: "The Discord role name.",
							Required:    true,
						},
						discord.ApplicationCommandOptionString{
							Name:        "description",
							Description: "The shop item description.",
							Required:    true,
						},
						discord.ApplicationCommandOptionInt{
							Name:        "price",
							Description: "The price of the role.",
							Required:    true,
						},
						discord.ApplicationCommandOptionString{
							Name:        "duration",
							Description: "How long the role lasts. Leave empty for permanent.",
							Required:    false,
						},
						discord.ApplicationCommandOptionBool{
							Name:        "auto-renewable",
							Description: "Whether the purchase can auto-renew.",
							Required:    false,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "remove-role",
					Description: "Removes a Discord role from the shop.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "name",
							Description: "The Discord role name.",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "channel",
					Description: "Sets the shop channel.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "id",
							Description: "The channel ID.",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "mod-channel",
					Description: "Sets the shop moderation notification channel.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "id",
							Description: "The channel ID.",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "publish",
					Description: "Publishes or repairs the shop message in the configured shop channel.",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "info",
					Description: "Gets the shop configuration.",
				},
			},
		},
	}

	memberCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "shop",
			Description: "Commands used to interact with the shop.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "purchases",
					Description: "Lists your shop purchases.",
				},
			},
		},
	}
)

// addRoleHandler handles the /shop add-role command.
func addRoleHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !disgobot.IsAdmin(e) || disgobot.IsShuttingDown(e) {
		return disgobot.ErrUnableToProcessCommand
	}

	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	roleName := strings.TrimSpace(stringValue(data, "name"))
	description := strings.TrimSpace(stringValue(data, "description"))
	price := intValue(data, "price")
	duration := strings.TrimSpace(stringValue(data, "duration"))
	autoRenewable := boolValue(data, "auto-renewable")

	if roleName == "" {
		return e.CreateMessage(discord.MessageCreate{
			Content: "Role name is required.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}
	if price < 0 {
		return e.CreateMessage(discord.MessageCreate{
			Content: "Price must be zero or greater.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	if err := roleExistsChecks(discordid.NewSnowflakeID(member.GuildID), roleName); err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	role := NewRole(discordid.NewSnowflakeID(member.GuildID), roleName, description, price, duration, autoRenewable)
	if role == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "Unable to add role to the shop.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	s := GetShop(member.GuildID.String())
	if err := role.AddToShop(s); err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	s = GetShop(member.GuildID.String())
	if err := s.Publish(); err != nil {
		slog.Error("unable to publish shop",
			slog.Any("guildID", member.GuildID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Role was added, but I could not publish the updated shop.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Added role `%s` to the shop for %d credits.", roleName, price),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// removeRoleHandler handles the /shop remove-role command.
func removeRoleHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !disgobot.IsAdmin(e) || disgobot.IsShuttingDown(e) {
		return disgobot.ErrUnableToProcessCommand
	}

	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	roleName := strings.TrimSpace(stringValue(data, "name"))
	role := GetRole(discordid.NewSnowflakeID(member.GuildID), roleName)
	if role == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Role `%s` is not in the shop.", roleName),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	s := GetShop(member.GuildID.String())
	if err := role.RemoveFromShop(s); err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	s = GetShop(member.GuildID.String())
	if err := s.Publish(); err != nil {
		slog.Error("unable to publish shop",
			slog.Any("guildID", member.GuildID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Role was removed, but I could not publish the updated shop.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Removed role `%s` from the shop.", roleName),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// setChannelHandler handles the /shop set-channel command.
func setChannelHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !disgobot.IsAdmin(e) || disgobot.IsShuttingDown(e) {
		return disgobot.ErrUnableToProcessCommand
	}

	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	channelID := strings.TrimSpace(stringValue(data, "id"))
	config := GetConfig(discordid.NewSnowflakeID(member.GuildID))
	config.SetChannel(channelID)

	s := GetShop(member.GuildID.String())
	if err := s.Publish(); err != nil {
		slog.Error("unable to publish shop",
			slog.Any("guildID", member.GuildID),
			slog.String("channelID", channelID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Shop channel was set, but I could not publish the shop.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Shop channel set to %s.", channelID),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// publishShopHandler handles the /shop-admin publish command.
func publishShopHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !disgobot.IsAdmin(e) || disgobot.IsShuttingDown(e) {
		return disgobot.ErrUnableToProcessCommand
	}

	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	config := GetConfig(discordid.NewSnowflakeID(member.GuildID))
	if config.ChannelID == "" {
		return e.CreateMessage(discord.MessageCreate{
			Content: "Shop channel is not set. Use `/shop-admin channel` first.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	s := GetShop(member.GuildID.String())
	if err := s.Publish(); err != nil {
		slog.Error("unable to publish shop",
			slog.Any("guildID", member.GuildID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "I could not publish the shop.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: "Shop published.",
		Flags:   discord.MessageFlagEphemeral,
	})
}

// setModChannelHandler handles the /shop set-mod-channel command.
func setModChannelHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !disgobot.IsAdmin(e) || disgobot.IsShuttingDown(e) {
		return disgobot.ErrUnableToProcessCommand
	}

	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	channelID := strings.TrimSpace(stringValue(data, "id"))
	config := GetConfig(discordid.NewSnowflakeID(member.GuildID))
	config.SetModChannel(channelID)

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Shop mod channel set to %s.", channelID),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// getShopInfoHandler handles the /shop info command.
func getShopInfoHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !disgobot.IsAdmin(e) || disgobot.IsShuttingDown(e) {
		return disgobot.ErrUnableToProcessCommand
	}

	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	config := GetConfig(discordid.NewSnowflakeID(member.GuildID))

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf(
			"**Shop Channel**: %s\n**Mod Channel**: %s\n**Message ID**: %s\n**Notification ID**: %s",
			config.ChannelID,
			config.ModChannelID,
			config.MessageID,
			config.NotificationID,
		),
		Flags: discord.MessageFlagEphemeral,
	})
}

// purchasesHandler handles the /shop purchases command.
func purchasesHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if disgobot.IsShuttingDown(e) {
		return disgobot.ErrUnableToProcessCommand
	}

	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	purchases := GetAllPurchases(member.GuildID.String(), member.User.ID.String())
	if len(purchases) == 0 {
		return e.CreateMessage(discord.MessageCreate{
			Content: "You have not purchased anything from the shop.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	fields := make([]discord.EmbedField, 0, len(purchases))
	for _, purchase := range purchases {
		fields = append(fields, discord.EmbedField{
			Name:  fmt.Sprintf("%s `%s`", purchase.Item.Type, purchase.Item.Name),
			Value: formatPurchase(purchase),
		})
	}

	p := message.NewPaginator(
		message.WithDiscordConfig(message.DiscordConfig{
			Client: client,
		}),
	)
	return p.CreateInteractionResponse(e, "Your Shop Purchases", fields, true)
}

// buyRoleButtonHandler handles the purchase of a role from the shop via a button component.
func buyRoleButtonHandler(e *handler.ComponentEvent) error {
	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This button can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	roleName := strings.TrimSpace(e.Vars["roleName"])
	if roleName == "" {
		return e.CreateMessage(discord.MessageCreate{
			Content: "Invalid shop role button.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	if err := rolePurchaseChecks(discordid.NewSnowflakeID(member.GuildID), discordid.NewSnowflakeID(member.User.ID), roleName); err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	role := GetRole(discordid.NewSnowflakeID(member.GuildID), roleName)
	if role == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Role `%s` was not found in the shop.", roleName),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	purchase, err := role.Purchase(discordid.NewSnowflakeID(member.User.ID), false)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	guildRole, err := getExistingGuildRole(discordid.NewSnowflakeID(member.GuildID), roleName)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Purchased `%s`, but I could not find the Discord role to assign it.", roleName),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	if err := client.Rest.AddMemberRole(member.GuildID, member.User.ID, guildRole.ID); err != nil {
		slog.Error("unable to assign purchased role",
			slog.Any("guildID", member.GuildID),
			slog.Any("memberID", member.User.ID),
			slog.String("roleName", roleName),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Purchased `%s`, but I could not assign the Discord role. Please contact an admin.", roleName),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Purchased role `%s` for %d credits.", purchase.Item.Name, purchase.Item.Price),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// formatPurchase formats a purchase into a human-readable string.
func formatPurchase(purchase *Purchase) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("**Status**: %s", purchase.Status))
	parts = append(parts, fmt.Sprintf("**Price**: %d", purchase.Item.Price))
	parts = append(parts, fmt.Sprintf("**Purchased**: %s", purchase.PurchasedOn.Format("2006-01-02 15:04:05 MST")))

	if !purchase.ExpiresOn.IsZero() {
		parts = append(parts, fmt.Sprintf("**Expires**: %s", purchase.ExpiresOn.Format("2006-01-02 15:04:05 MST")))
	}
	if purchase.AutoRenew {
		parts = append(parts, "**Auto-renew**: yes")
	}
	if purchase.IsExpired {
		parts = append(parts, "**Expired**: yes")
	}

	return strings.Join(parts, "\n")
}

// stringValue returns the string value of the given option name from the given slash command interaction data or an empty string if the option is not present.
func stringValue(data discord.SlashCommandInteractionData, name string) string {
	value, ok := data.Options[name]
	if !ok {
		return ""
	}

	return fmt.Sprint(value)
}

// intValue returns the integer value of the given option name from the given slash command interaction data or 0 if the option is not present or cannot be parsed.
func intValue(data discord.SlashCommandInteractionData, name string) int {
	value, ok := data.Options[name]
	if !ok {
		return 0
	}

	parsed, err := strconv.Atoi(fmt.Sprint(value))
	if err != nil {
		slog.Warn("unable to parse int option",
			slog.String("name", name),
			slog.Any("value", value),
			slog.Any("error", err),
		)
		return 0
	}

	return parsed
}

// boolValue returns the boolean value of the given option name from the given slash command interaction data or false if the option is not present or cannot be parsed.
func boolValue(data discord.SlashCommandInteractionData, name string) bool {
	value, ok := data.Options[name]
	if !ok {
		return false
	}

	parsed, err := strconv.ParseBool(fmt.Sprint(value))
	if err != nil {
		slog.Warn("unable to parse bool option",
			slog.String("name", name),
			slog.Any("value", value),
			slog.Any("error", err),
		)
		return false
	}

	return parsed
}
