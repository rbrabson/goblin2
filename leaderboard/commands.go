package leaderboard

import (
	"fmt"
	"goblin2/bank"
	"goblin2/disgobot"
	"goblin2/guild"
	"goblin2/internal/discordid"
	"log/slog"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type Type string

const (
	CurrentLeaderboard  Type = "Current Leaderboard"
	MonthlyLeaderboard  Type = "Monthly Leaderboard"
	LifetimeLeaderboard Type = "Lifetime Leaderboard"
)

var (
	adminCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "lb-admin",
			Description: "Commands used to interact with the leaderboard for this server.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "channel",
					Description: "Sets the channel ID where the monthly leaderboard is published.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "id",
							Description: "The channel ID.",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "info",
					Description: "Gets information about the leaderboard configuration.",
				},
			},
		},
	}

	memberCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "lb",
			Description: "Commands used to retrieve leaderboards on this server.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "current",
					Description: "Gets the current economy leaderboard.",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "monthly",
					Description: "Gets the monthly economy leaderboard.",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "lifetime",
					Description: "Gets the lifetime economy leaderboard.",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "rank",
					Description: "Gets the member rank for the leaderboards.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionUser{
							Name:        "user",
							Description: "The member to return the leaderboard.",
							Required:    false,
						},
					},
				},
			},
		},
	}
)

// currentLeaderboardHandler returns the top-ranked accounts for the current balance.
func currentLeaderboardHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	lb := getLeaderboard(discordid.NewSnowflakeID(member.GuildID))
	leaderboard := lb.getCurrentLeaderboard()
	return sendLeaderboard(e, CurrentLeaderboard, leaderboard)
}

// monthlyLeaderboardHandler returns the top-ranked accounts for the current month.
func monthlyLeaderboardHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	lb := getLeaderboard(discordid.NewSnowflakeID(member.GuildID))
	leaderboard := lb.getMonthlyLeaderboard()
	return sendLeaderboard(e, MonthlyLeaderboard, leaderboard)
}

// lifetimeLeaderboardHandler returns the top-ranked accounts for the lifetime of the server.
func lifetimeLeaderboardHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	lb := getLeaderboard(discordid.NewSnowflakeID(member.GuildID))
	leaderboard := lb.getLifetimeLeaderboard()
	return sendLeaderboard(e, LifetimeLeaderboard, leaderboard)
}

// setLeaderboardChannelHandler sets the server channel to which the monthly leaderboard is published.
func setLeaderboardChannelHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	lb := getLeaderboard(discordid.NewSnowflakeID(member.GuildID))
	channelID := stringValue(data, "id")
	lb.setChannel(channelID)

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Channel ID for the monthly leaderboard set to %s.", channelID),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// getLeaderboardInfoHandler returns the leaderboard configuration for the server.
func getLeaderboardInfoHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	lb := getLeaderboard(discordid.NewSnowflakeID(member.GuildID))

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Channel ID for the monthly leaderboard is `%s`.", lb.ChannelID),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// sendLeaderboard is a utility function that sends an economy leaderboard to Discord.
func sendLeaderboard(e *handler.CommandEvent, title Type, accounts []*bank.Account) error {
	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	guildMember := guild.GetMember(discordid.NewSnowflakeID(member.GuildID), &member.Member)
	if guildMember != nil {
		_ = guildMember.Update(&member.Member)
	}

	p := message.NewPrinter(language.AmericanEnglish)
	embeds := formatAccounts(e.Client(), discordid.NewSnowflakeID(member.GuildID), p, string(title), accounts)

	return e.CreateMessage(discord.MessageCreate{
		Embeds: embeds,
		Flags:  discord.MessageFlagEphemeral,
	})
}

// rankHandler returns the rankHandler of the member in the leaderboard.
func rankHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	p := message.NewPrinter(language.AmericanEnglish)

	memberID := discordid.NewSnowflakeID(member.User.ID)
	if optionMemberID, ok := userIDValue(data, "user"); ok {
		memberID = optionMemberID
	}

	account := bank.GetAccount(discordid.NewSnowflakeID(member.GuildID), memberID)
	if account == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: p.Sprintf("An account with the ID of %s does not exist.", memberID),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	currentRank, err := account.GetCurrentRanking()
	if err != nil {
		slog.Error("failed to get current rank",
			slog.Any("guildID", member.GuildID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Unable to get the current rank.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	monthlyRank, err := account.GetMonthlyRanking()
	if err != nil {
		slog.Error("failed to get monthly rank",
			slog.Any("guildID", member.GuildID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Unable to get the monthly rank.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	lifetimeRank, err := account.GetLifetimeRanking()
	if err != nil {
		slog.Error("failed to get lifetime rank",
			slog.Any("guildID", member.GuildID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Unable to get the lifetime rank.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	content := p.Sprintf("**Current Rank**: %d\n**Monthly Rank**: %d\n**Lifetime Rank**: %d\n", currentRank, monthlyRank, lifetimeRank)
	return e.CreateMessage(discord.MessageCreate{
		Content: content,
		Flags:   discord.MessageFlagEphemeral,
	})
}

// formatAccounts formats the leaderboard to be sent to a Discord server.
func formatAccounts(_ *bot.Client, guildID discordid.SnowflakeID, p *message.Printer, title string, accounts []*bank.Account) []discord.Embed {
	var tableBuffer strings.Builder

	table := tablewriter.NewTable(&tableBuffer,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Borders: tw.BorderNone,
			Symbols: tw.NewSymbols(tw.StyleASCII),
			Settings: tw.Settings{
				Separators: tw.Separators{BetweenRows: tw.Off, BetweenColumns: tw.Off},
				Lines:      tw.Lines{ShowHeaderLine: tw.Off},
			},
		})),
		tablewriter.WithConfig(tablewriter.Config{
			Row: tw.CellConfig{
				Padding:    tw.CellPadding{Global: tw.Padding{Left: "", Right: "", Top: "", Bottom: ""}},
				Formatting: tw.CellFormatting{AutoWrap: tw.WrapNone},
				Alignment:  tw.CellAlignment{Global: tw.AlignLeft},
			},
		}),
	)
	defer func(table *tablewriter.Table) {
		if err := table.Close(); err != nil {
			slog.Error("failed to close the table", "error", err)
		}
	}(table)

	header := []string{
		p.Sprintf("%-3s %-25s %-15s", "#", "NAME", "BALANCE"),
	}
	if err := table.Append(header); err != nil {
		slog.Error("failed to append header to the table", "error", err)
	}

	for i, account := range accounts {
		memberName := account.MemberID.String()
		guildMember, _ := guild.GetMemberByID(guildID, account.MemberID)
		if guildMember != nil {
			memberName = guildMember.Name
		}

		var balance int
		switch title {
		case string(CurrentLeaderboard):
			balance = account.CurrentBalance
		case string(MonthlyLeaderboard):
			balance = account.MonthlyBalance
		case string(LifetimeLeaderboard):
			balance = account.LifetimeBalance
		default:
			balance = account.MonthlyBalance
		}

		data := []string{
			p.Sprintf("%-3d %-25s %-15s", i+1, memberName, p.Sprintf("%d", balance)),
		}
		if err := table.Append(data); err != nil {
			slog.Error("failed to append data to the table", "error", err)
		}
	}
	if err := table.Render(); err != nil {
		slog.Error("failed to render the table", "error", err)
	}

	return []discord.Embed{
		{
			Type:  discord.EmbedTypeRich,
			Title: title,
			Fields: []discord.EmbedField{
				{
					Value: p.Sprintf("```\n%s```\n", tableBuffer.String()),
				},
			},
		},
	}
}

// userIDValue returns the SnowflakeID value of the option with the given name.
func userIDValue(data discord.SlashCommandInteractionData, name string) (discordid.SnowflakeID, bool) {
	value, ok := data.Options[name]
	if !ok {
		return 0, false
	}

	parsed, err := strconv.ParseUint(fmt.Sprint(value), 10, 64)
	if err != nil {
		slog.Warn("unable to parse user option",
			slog.String("name", name),
			slog.Any("value", value),
			slog.Any("error", err),
		)
		return 0, false
	}

	return discordid.SnowflakeID(parsed), true
}

// stringValue returns the string value of the option with the given name.
func stringValue(data discord.SlashCommandInteractionData, name string) string {
	value, ok := data.Options[name]
	if !ok {
		return ""
	}

	return fmt.Sprint(value)
}
