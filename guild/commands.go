package guild

import (
	"fmt"
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var adminCommands = discord.SlashCommandCreate{
	Name:        "guild-admin",
	Description: "Commands used to configure the bot for a given server.",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionSubCommandGroup{
			Name:        "role",
			Description: "Manages the admin roles for the bot for this server.",
			Options: []discord.ApplicationCommandOptionSubCommand{
				{
					Name:        "list",
					Description: "Returns the list of admin roles for the server.",
				},
				{
					Name:        "add",
					Description: "Adds an admin role for this server.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "name",
							Description: "The name of the role to add.",
							Required:    true,
						},
					},
				},
				{
					Name:        "remove",
					Description: "Removes an admin role for this server.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "name",
							Description: "The name of the role to remove.",
							Required:    true,
						},
					},
				},
			},
		},
	},
}

// guildAdminRoleListHandler returns the list of admin roles for the server.
func guildAdminRoleListHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	slog.Warn("TBD: GuildAdminRoleListHandler")
	return e.CreateMessage(discord.MessageCreate{
		Content: "stub: guild-admin role list",
		Flags:   discord.MessageFlagEphemeral,
	})
}

// guildAdminRoleAddHandler adds an admin role for the server.
func guildAdminRoleAddHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	slog.Warn("TBD: GuildAdminRoleAddHandler")
	name := fmt.Sprint(data.Options["name"])

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("stub: guild-admin role add %s", name),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// guildAdminRoleRemoveHandler removes an admin role for the server.
func guildAdminRoleRemoveHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	slog.Warn("TBD: GuildAdminRoleRemoveHandler")
	name := fmt.Sprint(data.Options["name"])

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("stub: guild-admin role remove %s", name),
		Flags:   discord.MessageFlagEphemeral,
	})
}
