package guild

import (
	"fmt"
	"goblin2/internal/discordid"
	"goblin2/plugin"
	"log/slog"
	"slices"
	"strings"

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
func guildAdminRoleListHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !IsAdmin(e) || isShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	guild := GetGuild(discordid.NewSnowflakeID(e.Member().GuildID))
	adminRoleNames := guild.GetAdminRoles()
	slices.Sort(adminRoleNames)
	adminRoles := strings.Join(adminRoleNames, "\n")

	return e.CreateMessage(discord.MessageCreate{
		Content: "**Admin Roles**:\n" + adminRoles,
		Flags:   discord.MessageFlagEphemeral,
	})
}

// guildAdminRoleAddHandler adds an admin role for the server.
func guildAdminRoleAddHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !IsAdmin(e) || isShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	guild := GetGuild(discordid.NewSnowflakeID(e.Member().GuildID))
	role := data.Options["name"].String()
	err := guild.AddAdminRole(role)
	if err != nil {
		slog.Error("failed to add the admin role",
			slog.Any("guildID", guild.GuildID),
			slog.Any("role", role),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Unable to add the admin role `%s`: %s", role, err.Error()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	slog.Info("admin role added to guild",
		slog.Any("guildID", guild.GuildID),
		slog.Any("role", role),
	)
	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Admin role `%s` has been added", role),
	})
}

// guildAdminRoleRemoveHandler removes an admin role for the server.
func guildAdminRoleRemoveHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !IsAdmin(e) || isShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	guild := GetGuild(discordid.NewSnowflakeID(e.Member().GuildID))
	role := data.Options["name"].String()
	err := guild.RemoveAdminRole(role)
	if err != nil {
		slog.Error("failed to remove the admin role",
			slog.Any("guildID", guild.GuildID),
			slog.Any("role", role),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Unable to remove the admin role `%s`: %s", role, err.Error()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	slog.Info("admin role removed from guild",
		slog.Any("guildID", guild.GuildID),
		slog.Any("role", role),
	)
	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Admin role `%s` has been removed", role),
	})
}

// IsAdmin checks if the member has permissions to manage the server. If not, it sends an ephemeral message to the user
// and returns false. Otherwise, it returns true.
func IsAdmin(e *handler.CommandEvent) bool {
	guild := GetGuild(discordid.NewSnowflakeID(e.Member().GuildID))
	member := GetMember(guild.GuildID, &e.Member().Member)
	isAdmin, _ := member.IsAdmin(e.Client(), guild)

	if !isAdmin {
		slog.Warn("user is not a guild admin",
			slog.String("user", e.Member().User.Username),
			slog.Any("user_id", e.Member().User.ID),
			slog.String("name", e.Member().EffectiveName()),
		)
		err := e.CreateMessage(discord.MessageCreate{
			Content: "You do not have permission to manage this server",
			Flags:   discord.MessageFlagEphemeral,
		})
		if err != nil {
			slog.Error("error sending permission error message",
				slog.Any("error", err),
			)
		}
		return false
	}

	return true
}

// isShuttingDown checks if the plugin is shutting down. If it is, it sends an ephemeral message to the user and returns
// true. Otherwise, it returns false.
func isShuttingDown(e *handler.CommandEvent) bool {
	if guildPlugin.status != plugin.Running {
		err := e.CreateMessage(discord.MessageCreate{
			Content: "The bot is shutting down and cannot process this command.",
			Flags:   discord.MessageFlagEphemeral,
		})
		if err != nil {
			slog.Error("error sending shutdown message",
				slog.Any("error", err),
			)
		}
		return true
	}
	return false
}
