package guild

import (
	"fmt"
	"goblin2/internal/discordid"
	"goblin2/plugin"
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/bot"
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

// roleFromOption returns the role with the given name from the given guild.
func roleFromOption(data discord.SlashCommandInteractionData, optionName string, guildID discordid.SnowflakeID, client *bot.Client) (discord.Role, error) {
	option, ok := data.Options[optionName]
	if !ok {
		slog.Error("invalid role option",
			slog.String("optionName", optionName),
		)
		return discord.Role{}, ErrRoleNotFound
	}

	roleIDText := strings.TrimSpace(option.String())
	roleIDText = strings.TrimPrefix(roleIDText, "<@&")
	roleIDText = strings.TrimSuffix(roleIDText, ">")

	roleID, err := snowflake.Parse(roleIDText)
	if err != nil {
		slog.Error("invalid role option",
			slog.Any("guildID", guildID),
			slog.Any("option", option),
			slog.String("roleIDText", roleIDText),
			slog.Any("error", err),
		)
		return discord.Role{}, fmt.Errorf("invalid role option %q: %w", option.String(), err)
	}

	roles, err := client.Rest.GetRoles(guildID.ID())
	if err != nil {
		return discord.Role{}, fmt.Errorf("unable to get roles for guild %s: %w", guildID, err)
	}

	for _, role := range roles {
		if role.ID == roleID {
			return role, nil
		}
	}

	slog.Error("role not found",
		slog.Any("guildID", guildID),
		slog.Any("option", option),
		slog.String("roleIDText", roleIDText),
		slog.Any("error", err),
	)
	return discord.Role{}, ErrRoleNotFound
}

// guildAdminRoleAddHandler adds an admin role for the server.
func guildAdminRoleAddHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isAdmin(e) || isShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	guild := GetGuild(discordid.NewSnowflakeID(e.Member().GuildID))
	role, err := roleFromOption(data, "role", guild.GuildID, e.Client())
	if err != nil {
		slog.Error("invalid role",
			slog.Any("guildID", guild.GuildID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Please provide a valid role",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	roleID := discordid.NewSnowflakeID(role.ID)
	err = guild.AddAdminRole(roleID)
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
	role, err := roleFromOption(data, "role", guild.GuildID, e.Client())
	if err != nil {
		slog.Error("invalid role",
			slog.Any("guildID", guild.GuildID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Please provide a valid role",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	roleID := discordid.NewSnowflakeID(role.ID)
	err = guild.RemoveAdminRole(roleID)
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
	guild := GetGuild(guildID)

	discordGuild, err := e.Client().Rest.GetGuild(guildID.ID(), false)
	if err != nil {
		slog.Error("unable to get guild for admin check",
			slog.Any("guildID", guildID),
			slog.Any("userID", e.Member().User.ID),
			slog.Any("error", err),
		)
		return sendPermissionDeniedMessage(e)
	}

	if discordGuild.OwnerID == e.Member().User.ID {
		return true
	}

	memberRoleIDs := make(map[discordid.SnowflakeID]struct{}, len(e.Member().RoleIDs))
	for _, roleID := range e.Member().RoleIDs {
		memberRoleIDs[discordid.NewSnowflakeID(roleID)] = struct{}{}
	}

	for _, adminRoleID := range guild.AdminRoles {
		if _, ok := memberRoleIDs[adminRoleID]; ok {
			return true
		}
	}

	defaultAdminRoleNameSet := make(map[string]struct{}, len(defaultAdminRoleNames))
	for _, roleName := range defaultAdminRoleNames {
		defaultAdminRoleNameSet[roleName] = struct{}{}
	}

	guildRoles, err := e.Client().Rest.GetRoles(guildID.ID())
	if err != nil {
		slog.Error("unable to get guild roles for admin check",
			slog.Any("guildID", guildID),
			slog.Any("userID", e.Member().User.ID),
			slog.Any("error", err),
		)
		return sendPermissionDeniedMessage(e)
	}

	for _, role := range guildRoles {
		roleID := discordid.NewSnowflakeID(role.ID)
		if _, memberHasRole := memberRoleIDs[roleID]; !memberHasRole {
			continue
		}

		if role.Permissions.Has(discord.PermissionAdministrator) {
			return true
		}

		if _, isDefaultAdminRole := defaultAdminRoleNameSet[role.Name]; isDefaultAdminRole {
			return true
		}
	}

	slog.Warn("user is not a guild admin",
		slog.String("user", e.Member().User.Username),
		slog.Any("user_id", e.Member().User.ID),
		slog.Any("guild_id", guildID),
		slog.String("name", e.Member().EffectiveName()),
		slog.Any("member_role_ids", e.Member().RoleIDs),
		slog.Any("configured_admin_role_ids", guild.AdminRoles),
	)

	return sendPermissionDeniedMessage(e)
}

func sendPermissionDeniedMessage(e *handler.CommandEvent) bool {
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
