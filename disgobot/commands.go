package disgobot

import (
	"fmt"
	"goblin2/internal/log"
	"goblin2/message"
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
	return staticHelpHandler(b, "Commands", helpMessages)
}

// adminHelpHandler is a slash command handler that provides help information for admin commands.
func adminHelpHandler(b *Bot) handler.SlashCommandHandler {
	helpMessages := buildHelpMessages(b, func(p plugin.Plugin) map[string]string {
		return p.GetAdminHelp()
	})
	return staticHelpHandler(b, "Admin Commands", helpMessages)
}

// staticHelpHandler is a helper function that returns a slash command handler that sends a static help message.
func staticHelpHandler(b *Bot, title string, helpMessages [][]string) handler.SlashCommandHandler {
	return func(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
		return sendHelpMessages(b, e, title, helpMessages)
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

// serverShutdownHandler handles the server shutdown command.
func serverShutdownHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	return e.CreateMessage(discord.MessageCreate{
		Content: "stub: server shutdown",
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverStatusHandler handles the server status command.
func serverStatusHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	return e.CreateMessage(discord.MessageCreate{
		Content: "stub: server status",
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverOwnerAddHandler handles the server owner add command.
func serverOwnerAddHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	user := fmt.Sprint(data.Options["user"])
	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("stub: server owner add %s", user),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverOwnerRemoveHandler handles the server owner remove command.
func serverOwnerRemoveHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	user := fmt.Sprint(data.Options["user"])
	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("stub: server owner remove %s", user),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverOwnerListHandler handles the server owner list command.
func serverOwnerListHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	return e.CreateMessage(discord.MessageCreate{
		Content: "stub: server owner list",
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverAdminAddHandler handles the server admin add command.
func serverAdminAddHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	user := fmt.Sprint(data.Options["user"])
	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("stub: server admin add %s", user),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverAdminRemoveHandler handles the server admin remove command.
func serverAdminRemoveHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	user := fmt.Sprint(data.Options["user"])
	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("stub: server admin remove %s", user),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverAdminListHandler handles the server admin list command.
func serverAdminListHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	return e.CreateMessage(discord.MessageCreate{
		Content: "stub: server admin list",
		Flags:   discord.MessageFlagEphemeral,
	})
}

// serverLogHandler handles the server log command.
func serverLogHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	member := e.Member()
	slog.Info("log command handler",
		slog.String("user", member.User.Username),
		slog.Any("user_id", member.User.ID),
		slog.String("name", member.EffectiveName()),
	)

	level := data.Options["level"]
	slog.Info("set log level", slog.Any("level", level))
	log.SetLevel(slog.LevelDebug)
	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Log level set to %s", level),
		Flags:   discord.MessageFlagEphemeral,
	})
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
