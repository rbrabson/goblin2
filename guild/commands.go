package guild

import (
	"fmt"
	"goblin2/internal/discordid"
	"goblin2/plugin"
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
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
						discord.ApplicationCommandOptionRole{
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
						discord.ApplicationCommandOptionRole{
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

func guildAdminRoleListHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isAdmin(e) || isShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	guild := GetGuild(discordid.NewSnowflakeID(e.Member().GuildID))
	adminRoleNames := guild.GetAdminRoles()
	adminRoles := strings.Join(adminRoleNames, "\n")

	return e.CreateMessage(discord.MessageCreate{
		Content: "**Admin Roles**:\n" + adminRoles,
		Flags:   discord.MessageFlagEphemeral,
	})
}

// guildAdminRoleAddHandler adds an admin role for the server.
func guildAdminRoleAddHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isAdmin(e) || isShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	guild := GetGuild(discordid.NewSnowflakeID(e.Member().GuildID))
	role := data.Role("role")
	if role.ID == snowflake.ID(0) {
		slog.Error("invalid role",
			slog.Any("guildID", guild.GuildID),
			slog.Any("role", role.Name),
			slog.Any("roleID", role.ID),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Please provide a valid role",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	roleID := discordid.NewSnowflakeID(role.ID)
	err := guild.AddAdminRole(roleID)
	if err != nil {
		slog.Error("failed to add the admin role",
			slog.Any("guildID", guild.GuildID),
			slog.Any("role", role.Name),
			slog.Any("roleID", role.ID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Unable to add the admin role `%s`: %s", role.Name, err.Error()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	slog.Info("admin role added to guild",
		slog.Any("guildID", guild.GuildID),
		slog.Any("role", role.Name),
		slog.Any("roleID", role.ID),
	)
	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Admin role `%s` has been added", role.Name),
	})
}

// guildAdminRoleRemoveHandler removes an admin role for the server.
func guildAdminRoleRemoveHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isAdmin(e) || isShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	guild := GetGuild(discordid.NewSnowflakeID(e.Member().GuildID))
	role := data.Role("role")
	if role.ID == snowflake.ID(0) {
		slog.Error("invalid role",
			slog.Any("guildID", guild.GuildID),
			slog.Any("role", role.Name),
			slog.Any("roleID", role.ID),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Please provide a valid role",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	roleID := discordid.NewSnowflakeID(role.ID)
	err := guild.RemoveAdminRole(roleID)
	if err != nil {
		slog.Error("failed to remove the admin role",
			slog.Any("guildID", guild.GuildID),
			slog.Any("role", role.Name),
			slog.Any("roleID", role.ID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Unable to remove the admin role `%s`: %s", role.Name, err.Error()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	slog.Info("admin role removed from guild",
		slog.Any("guildID", guild.GuildID),
		slog.Any("role", role.Name),
		slog.Any("roleID", role.ID),
	)
	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Admin role `%s` has been removed", role.Name),
	})
}

// isAdmin checks if the member has permissions to manage the server. If not, it sends an ephemeral message to the user
// and returns false. Otherwise, it returns true.
func isAdmin(e *handler.CommandEvent) bool {
	guildID := discordid.NewSnowflakeID(e.Member().GuildID)
	memberID := discordid.NewSnowflakeID(e.Member().User.ID)

	guild := GetGuild(guildID)
	member, err := GetMemberByID(guildID, memberID)
	if err != nil {
		member = GetMember(guildID, &e.Member().Member)
	}

	isAdmin, _ := member.IsAdmin(e.Client(), guild)

	if !isAdmin {
		slog.Warn("user is not a guild admin",
			slog.String("user", e.Member().User.Username),
			slog.Any("user_id", e.Member().User.ID),
			slog.Any("guild_id", guildID),
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
