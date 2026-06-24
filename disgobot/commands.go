package disgobot

import (
	"fmt"
	"goblin2/guild"
	"goblin2/internal/discordid"
	"goblin2/internal/log"
	"goblin2/internal/message"
	"goblin2/plugin"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var helpCommands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "help",
		Description: "Provides a description of commands for this server.",
	},
	discord.SlashCommandCreate{
		Name:        "admin-help",
		Description: "Provides a description of admin commands for this server.",
	},
	discord.SlashCommandCreate{
		Name:        "version",
		Description: "Returns the version of the bot running on the server.",
	},
}

// commands is a list of slash commands supported by the bot.
var serverCommands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "server",
		Description: "Commands used to interact with the server itself.",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "shutdown",
				Description: "Prepares the server to be shutdown.",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "status",
				Description: "Returns the status of the server.",
			},
			discord.ApplicationCommandOptionSubCommandGroup{
				Name:        "owner",
				Description: "Manages the server owners.",
				Options: []discord.ApplicationCommandOptionSubCommand{
					{
						Name:        "add",
						Description: "Adds an owner for this server.",
						Options: []discord.ApplicationCommandOption{
							discord.ApplicationCommandOptionUser{
								Name:        "user",
								Description: "The member to add as an owner.",
								Required:    true,
							},
						},
					},
					{
						Name:        "remove",
						Description: "Removes an owner for this server.",
						Options: []discord.ApplicationCommandOption{
							discord.ApplicationCommandOptionUser{
								Name:        "user",
								Description: "The member to remove as an owner.",
								Required:    true,
							},
						},
					},
					{
						Name:        "list",
						Description: "Lists the owners for this server.",
					},
				},
			},
			discord.ApplicationCommandOptionSubCommandGroup{
				Name:        "admin",
				Description: "Manages the server admins.",
				Options: []discord.ApplicationCommandOptionSubCommand{
					{
						Name:        "add",
						Description: "Adds an admin for this server.",
						Options: []discord.ApplicationCommandOption{
							discord.ApplicationCommandOptionUser{
								Name:        "user",
								Description: "The member to add as an admin.",
								Required:    true,
							},
						},
					},
					{
						Name:        "remove",
						Description: "Removes an admin for this server.",
						Options: []discord.ApplicationCommandOption{
							discord.ApplicationCommandOptionUser{
								Name:        "user",
								Description: "The member to remove as an admin.",
								Required:    true,
							},
						},
					},
					{
						Name:        "list",
						Description: "Lists the admins for this server.",
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "log",
				Description: "Sets the logging level for the server.",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "level",
						Description: "The logging level to set for the server.",
						Required:    true,
						Choices: []discord.ApplicationCommandOptionChoiceString{
							{
								Name:  "debug",
								Value: "debug",
							},
							{
								Name:  "info",
								Value: "info",
							},
							{
								Name:  "warn",
								Value: "warn",
							},
							{
								Name:  "error",
								Value: "error",
							},
						},
					},
				},
			},
		},
	},
}

// commandHelp is a struct that holds the name and help string of a command.
type commandHelp struct {
	Name string
	Help string
}

// pluginHelp is a struct that holds the name and help commands of a plugin.
type pluginHelp struct {
	PluginName string
	Commands   []commandHelp
}

// helpHandler is a slash command handler that provides help information for commands.
func helpHandler(b *Bot) handler.SlashCommandHandler {
	helpMessages := buildHelpMessages(b, func(p plugin.Plugin) map[string]string {
		return p.GetHelp()
	})
	return func(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
		if IsShuttingDown(e) {
			return ErrUnableToProcessCommand
		}
		return sendHelpMessages(b, e, "Commands", helpMessages)
	}
}

// adminHelpHandler is a slash command handler that provides help information for admin commands.
func adminHelpHandler(b *Bot) handler.SlashCommandHandler {
	helpMessages := buildHelpMessages(b, func(p plugin.Plugin) map[string]string {
		return p.GetAdminHelp()
	})
	return func(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
		if !IsAdmin(e) || IsShuttingDown(e) {
			return ErrUnableToProcessCommand
		}
		return sendHelpMessages(b, e, "Admin Commands", helpMessages)
	}
}

// buildHelpMessages constructs structured help messages for the paginator.
func buildHelpMessages(b *Bot, helpSelector func(plugin.Plugin) map[string]string) [][]string {
	caser := cases.Title(language.AmericanEnglish)
	pluginHelps := collectPluginHelp(b, helpSelector)

	helpMessages := make([][]string, 0, len(pluginHelps))
	for _, ph := range pluginHelps {
		if len(ph.Commands) == 0 {
			continue
		}

		msg := make([]string, 0, len(ph.Commands)+1)
		msg = append(msg, caser.String(ph.PluginName))

		for _, ch := range ph.Commands {
			msg = append(msg, fmt.Sprintf("- `%s`: %s", ch.Name, ch.Help))
		}

		helpMessages = append(helpMessages, msg)
	}

	return helpMessages
}

// collectPluginHelp collects help information for all plugins using the provided help selector function.
func collectPluginHelp(b *Bot, helpSelector func(plugin.Plugin) map[string]string) []pluginHelp {
	msgs := make([]pluginHelp, 0, len(b.plugins))

	for _, p := range b.plugins {
		helpMap := helpSelector(p)
		if len(helpMap) == 0 {
			continue
		}

		commands := make([]commandHelp, 0, len(helpMap))
		for commandName, commandText := range helpMap {
			commands = append(commands, commandHelp{
				Name: commandName,
				Help: commandText,
			})
		}

		sort.Slice(commands, func(i, j int) bool {
			return commands[i].Name < commands[j].Name
		})

		msgs = append(msgs, pluginHelp{
			PluginName: p.GetName(),
			Commands:   commands,
		})
	}

	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].PluginName < msgs[j].PluginName
	})

	return msgs
}

// versionHandler handles the version command.
func versionHandler(b *Bot) handler.SlashCommandHandler {
	return func(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
		if IsShuttingDown(e) {
			return ErrUnableToProcessCommand
		}

		member := e.Member()
		slog.Info("version command handler",
			slog.String("user", member.User.Username),
			slog.Any("user_id", member.User.ID),
			slog.String("name", member.EffectiveName()),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Version: %s", b.version),
		})
	}
}

// sendHelpMessages sends help messages in a paginated format.
func sendHelpMessages(b *Bot, e *handler.CommandEvent, title string, helpMessages [][]string) error {
	inline := false
	embedFields := make([]discord.EmbedField, 0, len(helpMessages))

	for _, msg := range helpMessages {
		if len(msg) == 0 {
			continue
		}

		name := msg[0]

		var value string
		if len(msg) > 1 {
			value = strings.Join(msg[1:], "\n")
			value = strings.ReplaceAll(value, "\n\n", "\n")
		}
		if value == "" {
			value = "\u200b"
		}

		embedFields = append(embedFields, discord.EmbedField{
			Name:   name,
			Value:  value,
			Inline: &inline,
		})
	}

	if len(embedFields) == 0 {
		embedFields = append(embedFields, discord.EmbedField{
			Name:   "No commands",
			Value:  "No help is available.",
			Inline: &inline,
		})
	}

	paginator := message.NewPaginator(
		message.WithDiscordConfig(message.DiscordConfig{
			Client: b.client,
			AddComponentHandler: func(key string, h handler.ComponentHandler) {
				if !strings.HasPrefix(key, "/") {
					key = "/" + key
				}
				b.router.Component(key, h)
			},
			RemoveComponentHandler: func(key string) {
			},
		}),
		message.WithItemsPerPage(5),
		message.WithIdleWait(5*time.Minute),
	)

	if err := paginator.CreateInteractionResponse(e, title, embedFields, true); err != nil {
		slog.Error("unable to send help commands",
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// ------------ Server Commands ------------ //

// serverShutdownHandler handles the server shutdown command.
func serverShutdownHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isOwner(e) || IsShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	for _, p := range goblin.plugins {
		p.Stop()
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: "Shutting down all bot services.",
	})
}

// serverStatusHandler handles the server status command.
func serverStatusHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isOwner(e) {
		return ErrUnableToProcessCommand
	}

	plugins := goblin.GetPlugins()
	inline := true
	pluginStatus := make([]discord.EmbedField, 0, len(plugins))

	botStatus := plugin.Running
	for _, p := range plugins {
		slog.Debug("plugin status",
			slog.String("plugin", p.GetName()),
			slog.String("status", p.Status().String()),
		)
		switch p.Status() {
		case plugin.Stopping:
			botStatus = plugin.Stopping
			break
		case plugin.Stopped:
			if botStatus == plugin.Running {
				botStatus = plugin.Stopped
			}
		default:
			// NO-OP
		}

		pluginStatus = append(pluginStatus, discord.EmbedField{
			Name:   cases.Title(language.AmericanEnglish).String(p.GetName()),
			Value:  p.Status().String(),
			Inline: &inline,
		})
	}

	if len(pluginStatus) == 0 {
		pluginStatus = append(pluginStatus, discord.EmbedField{
			Name:   "No plugins",
			Value:  "No plugins are registered.",
			Inline: &inline,
		})
	}

	embeds := []discord.Embed{
		{
			Title:       "Server Status",
			Description: botStatus.String(),
		},
		{
			Title:  "Plugin Status",
			Fields: pluginStatus,
		},
	}

	if err := e.CreateMessage(discord.MessageCreate{
		Embeds: embeds,
		Flags:  discord.MessageFlagEphemeral,
	}); err != nil {
		slog.Error("failed to send server status",
			slog.Any("error", err),
		)
		return err
	}

	slog.Debug("sent server status",
		slog.Any("embeds", embeds),
	)
	return nil
}

// serverOwnerAddHandler handles the server owner add command.
func serverOwnerAddHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isOwner(e) || IsShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	user := fmt.Sprint(data.Options["user"])
	userID, err := getUserID(e, user)
	if err != nil {
		return err
	}

	server := GetServer()
	if err := server.AddOwner(userID); err != nil {
		slog.Error("failed to add owner",
			slog.Any("userID", e.Member().User.ID),
			slog.String("userName", e.Member().EffectiveName()),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to add %s as a server owner", e.Member().EffectiveName()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Added %s as a server owner", user),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverOwnerRemoveHandler handles the server owner remove command.
func serverOwnerRemoveHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isOwner(e) || IsShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	user := fmt.Sprint(data.Options["user"])
	userID, err := getUserID(e, user)
	if err != nil {
		return err
	}

	server := GetServer()

	if err := server.RemoveOwner(userID); err != nil {
		slog.Error("failed to remove owner",
			slog.Any("userID", e.Member().User.ID),
			slog.String("userName", e.Member().EffectiveName()),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to remove %s as a server owner", e.Member().EffectiveName()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Removed %s as a server owner", user),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverOwnerListHandler handles the server owner list command.
func serverOwnerListHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isOwner(e) || IsShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	server := GetServer()
	owners := server.ListOwners()
	if len(owners) == 0 {
		return e.CreateMessage(discord.MessageCreate{
			Content: "There are no owners for this server",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	ownerNames := make([]string, 0, len(owners))
	guildID := discordid.NewSnowflakeID(e.Member().GuildID)
	for _, ownerID := range owners {
		owner, err := guild.GetMemberByID(guildID, ownerID)
		if err != nil {
			ownerNames = append(ownerNames, ownerID.String())
		} else {
			ownerNames = append(ownerNames, owner.Name)
		}
	}

	err := e.CreateMessage(discord.MessageCreate{
		Content: "Owners: " + strings.Join(ownerNames, ","),
		Flags:   discord.MessageFlagEphemeral,
	})
	if err != nil {
		slog.Error("failed to send response",
			slog.Any("error", err),
		)
	}
	return nil
}

// serverAdminAddHandler handles the server admin add command.
func serverAdminAddHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isOwner(e) || IsShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	user := fmt.Sprint(data.Options["user"])
	userID, err := getUserID(e, user)
	if err != nil {
		return err
	}

	server := GetServer()
	if err := server.AddAdmin(userID); err != nil {
		slog.Error("failed to add admin",
			slog.Any("userID", e.Member().User.ID),
			slog.String("userName", e.Member().EffectiveName()),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to add %s as a server admin", e.Member().EffectiveName()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Added %s as a server admin", user),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverAdminRemoveHandler handles the server admin remove command.
func serverAdminRemoveHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isOwner(e) || IsShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	user := fmt.Sprint(data.Options["user"])
	userID, err := getUserID(e, user)
	if err != nil {
		return err
	}

	server := GetServer()

	if err := server.RemoveAdmin(userID); err != nil {
		slog.Error("failed to remove admin",
			slog.Any("userID", e.Member().User.ID),
			slog.String("userName", e.Member().EffectiveName()),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to remove %s as a server admin", e.Member().EffectiveName()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Removed %s as a server admin", user),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverAdminListHandler handles the server admin list command.
func serverAdminListHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isOwner(e) || IsShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	server := GetServer()
	admins := server.ListAdmins()
	if len(admins) == 0 {
		return e.CreateMessage(discord.MessageCreate{
			Content: "There are no admins for this server",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	adminNames := make([]string, 0, len(admins))
	guildID := discordid.NewSnowflakeID(e.Member().GuildID)
	for _, adminID := range admins {
		admin, err := guild.GetMemberByID(guildID, adminID)
		if err != nil {
			adminNames = append(adminNames, adminID.String())
		} else {
			adminNames = append(adminNames, admin.Name)
		}
	}

	err := e.CreateMessage(discord.MessageCreate{
		Content: "Admins: " + strings.Join(adminNames, ","),
		Flags:   discord.MessageFlagEphemeral,
	})
	if err != nil {
		slog.Error("failed to send response",
			slog.Any("error", err),
		)
	}
	return nil
}

// serverLogHandler handles the server log command.
func serverLogHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if !isOwner(e) || IsShuttingDown(e) {
		return ErrUnableToProcessCommand
	}

	level := data.Options["level"]
	slog.Info("set log level", slog.Any("level", level))
	log.SetLevel(slog.LevelDebug)
	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Log level set to %s", level),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// isOwner checks if the member has permissions to manage the server.
func isOwner(e *handler.CommandEvent) bool {
	server := GetServer()
	if !server.CanManageOwners(discordid.NewSnowflakeID(e.Member().User.ID)) {
		slog.Warn("user does not have permission to manage server",
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

// IsAdmin checks if the member has permissions to manage the server. Owners are treated as admins (and when no owners
// are configured, any member may manage the server). If the member lacks permission, it sends an ephemeral message to
// the user and returns false. Otherwise, it returns true.
func IsAdmin(e *handler.CommandEvent) bool {
	server := GetServer()
	if !server.CanManage(discordid.NewSnowflakeID(e.Member().User.ID)) {
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

// IsShuttingDown checks if the bot is shutting down. If it is, it sends an ephemeral message to the user and returns
// true. Otherwise, it returns false.
func IsShuttingDown(e *handler.CommandEvent) bool {
	if goblin.IsStopping() {
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

// getUserID parses a user ID from a string.
func getUserID(e *handler.CommandEvent, id string) (discordid.SnowflakeID, error) {
	userID, err := discordid.SnowflakeIDFromString(id)
	if err != nil {
		slog.Error("failed to parse user ID",
			slog.String("userID", id),
			slog.Any("error", err),
		)
		err = e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to add %s as a server owner", e.Member().EffectiveName()),
			Flags:   discord.MessageFlagEphemeral,
		})
		if err != nil {
			slog.Error("error ssending parse error message,",
				slog.Any("error", err),
			)
		}
	}
	return userID, nil
}
