package stats

import (
	"fmt"
	"goblin2/disgobot"
	"goblin2/guild"
	"goblin2/internal/discordid"
	"log/slog"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	All       = "all"
	Blackjack = "blackjack"
	Heist     = "heist"
	Race      = "race"
	Slots     = "slots"
)

var (
	adminCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "stats-admin",
			Description: "Commands used to interact with the stats system.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "retention",
					Description: "View player retention.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "game",
							Description: "The game for which to determine the retention.",
							Required:    true,
							Choices: []discord.ApplicationCommandOptionChoiceString{
								{Name: "All", Value: All},
								{Name: "Blackjack", Value: Blackjack},
								{Name: "Heist", Value: Heist},
								{Name: "Race", Value: Race},
								{Name: "Slots", Value: Slots},
							},
						},
						discord.ApplicationCommandOptionString{
							Name:        "after",
							Description: "The time period to get the number of games.",
							Required:    true,
							Choices: []discord.ApplicationCommandOptionChoiceString{
								{Name: "One Week", Value: OneWeek},
								{Name: "Three Months", Value: ThreeMonths},
								{Name: "Six Months", Value: SixMonths},
								{Name: "Nine Months", Value: NineMonths},
								{Name: "Twelve Months", Value: TwelveMonths},
							},
						},
						discord.ApplicationCommandOptionString{
							Name:        "since",
							Description: "The time period to check the player retention.",
							Required:    false,
							Choices: []discord.ApplicationCommandOptionChoiceString{
								{Name: "Last Week", Value: LastWeek},
								{Name: "Last Month", Value: LastMonth},
								{Name: "Three Months Ago", Value: ThreeMonthsAgo},
								{Name: "Six Months Ago", Value: SixMonthsAgo},
								{Name: "Nine Months Ago", Value: NineMonthsAgo},
								{Name: "Twelve Months Ago", Value: TwelveMonthsAgo},
							},
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "played",
					Description: "View the number of games played.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "game",
							Description: "The game for which to get the number of games played.",
							Required:    true,
							Choices: []discord.ApplicationCommandOptionChoiceString{
								{Name: "All", Value: All},
								{Name: "Blackjack", Value: Blackjack},
								{Name: "Heist", Value: Heist},
								{Name: "Race", Value: Race},
								{Name: "Slots", Value: Slots},
							},
						},
						discord.ApplicationCommandOptionString{
							Name:        "since",
							Description: "The time period to check the number of games played.",
							Required:    false,
							Choices: []discord.ApplicationCommandOptionChoiceString{
								{Name: "Yesterday", Value: OneDayAgo},
								{Name: "Last Week", Value: LastWeek},
								{Name: "Last Month", Value: LastMonth},
								{Name: "Three Months Ago", Value: ThreeMonthsAgo},
								{Name: "Six Months Ago", Value: SixMonthsAgo},
								{Name: "Nine Months Ago", Value: NineMonthsAgo},
								{Name: "Twelve Months Ago", Value: TwelveMonthsAgo},
							},
						},
					},
				},
			},
		},
	}

	memberCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "stats",
			Description: "Commands used to interact with the stats system.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "played",
					Description: "View games played by a player.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "game",
							Description: "The game for which to determine the number of games played.",
							Required:    true,
							Choices: []discord.ApplicationCommandOptionChoiceString{
								{Name: "All", Value: All},
								{Name: "Blackjack", Value: Blackjack},
								{Name: "Heist", Value: Heist},
								{Name: "Race", Value: Race},
								{Name: "Slots", Value: Slots},
							},
						},
						discord.ApplicationCommandOptionUser{
							Name:        "user",
							Description: "The member or member ID.",
							Required:    false,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "active",
					Description: "View the players who play the most games.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "game",
							Description: "The game for which to determine the number of games played.",
							Required:    true,
							Choices: []discord.ApplicationCommandOptionChoiceString{
								{Name: "All", Value: All},
								{Name: "Blackjack", Value: Blackjack},
								{Name: "Heist", Value: Heist},
								{Name: "Race", Value: Race},
								{Name: "Slots", Value: Slots},
							},
						},
					},
				},
			},
		},
	}
)

// statsAdmin handles the /stats-admin command.
func statsAdmin(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	g := guild.GetGuild(discordid.NewSnowflakeID(member.GuildID))
	guildMember := guild.GetMember(discordid.NewSnowflakeID(member.GuildID), &member.Member)
	if guildMember == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "Unable to resolve your server membership.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	isAdmin, err := guildMember.IsAdmin(e.Client(), g)
	if err != nil {
		slog.Error("failed to check admin permissions",
			slog.Any("guildID", member.GuildID),
			slog.Any("memberID", member.User.ID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Unable to verify your permissions.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}
	if !isAdmin {
		return e.CreateMessage(discord.MessageCreate{
			Content: "You do not have permission to use this command.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	switch slashSubCommandName(data) {
	case "retention":
		return playerRetention(data, e)
	case "played":
		return gamesPlayed(data, e)
	default:
		return e.CreateMessage(discord.MessageCreate{
			Content: "Invalid command.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}
}

// playerRetention handles the /stats-admin retention command.
func playerRetention(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	p := message.NewPrinter(language.AmericanEnglish)
	titleCaser := cases.Title(language.AmericanEnglish)

	game := stringValue(data, "game")
	after := stringValue(data, "after")
	since := stringValue(data, "since")

	guildID := getGuildID(e)

	slog.Debug("player retention command received",
		slog.String("guild_id", guildID),
		slog.String("game", game),
		slog.String("after", after),
		slog.String("since", since),
	)

	firstGameDate := getFirstGameDate(guildID, game)
	duration := getDuration(after, firstGameDate)
	timeAfter := getTime(since, firstGameDate)

	retention, err := GetPlayerRetention(guildID, game, timeAfter, duration)
	if err != nil {
		slog.Error("failed to get player retention", slog.Any("error", err))
		return e.CreateMessage(discord.MessageCreate{
			Content: "Failed to get player retention: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	checkDuration := today().AddDate(0, 0, -1).Sub(timeAfter)

	title := titleCaser.String("Player Retention")
	if game != "" && game != "all" {
		title = titleCaser.String("Player Retention for " + game)
	}

	fields := []discord.EmbedField{
		{
			Name:  "After",
			Value: timeToString(after),
		},
	}
	if since != "" {
		fields = append(fields, discord.EmbedField{
			Name:  "Since",
			Value: p.Sprintf("%s Ago", fmtDuration(checkDuration)),
		})
	}
	fields = append(fields,
		discord.EmbedField{
			Name:  "Total Players",
			Value: p.Sprintf("%d", retention.ActivePlayers+retention.InactivePlayers),
		},
		discord.EmbedField{
			Name:  "Active Players",
			Value: p.Sprintf("%d (%.0f%%)", retention.ActivePlayers, retention.ActivePercentage),
		},
		discord.EmbedField{
			Name:  "Inactive Players",
			Value: p.Sprintf("%d (%.0f%%)", retention.InactivePlayers, retention.InactivePercentage),
		},
	)

	return e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{
			{
				Type:   discord.EmbedTypeRich,
				Title:  title,
				Fields: fields,
			},
		},
	})
}

// gamesPlayed handles the /stats-admin played command.
func gamesPlayed(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	p := message.NewPrinter(language.AmericanEnglish)
	titleCaser := cases.Title(language.AmericanEnglish)
	today := today()

	game := stringValue(data, "game")
	since := stringValue(data, "since")

	guildID := getGuildID(e)

	slog.Debug("games played command received",
		slog.String("guild_id", guildID),
		slog.String("game", game),
		slog.String("since", since),
	)

	firstGameDate := getFirstServerGameDate(guildID, game)
	startTime := getTime(since, firstGameDate)
	endTime := today.AddDate(0, 0, -1)

	if game == "" {
		game = "all"
	}

	gamesPlayed, err := GetGamesPlayed(guildID, game, startTime, endTime)
	if err != nil {
		slog.Error("failed to get games played", slog.Any("error", err))
		return e.CreateMessage(discord.MessageCreate{
			Content: "Failed to get games played: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	title := titleCaser.String("Games Played")
	if game != "all" {
		title = titleCaser.String("Games Played for " + game)
	}

	fields := []discord.EmbedField{
		{
			Name:  "Since",
			Value: p.Sprintf("%s Ago", fmtDuration(endTime.Sub(startTime))),
		},
		{
			Name:  "Unique Players",
			Value: p.Sprintf("%d", gamesPlayed.UniquePlayers),
		},
		{
			Name:  "Total Games Played",
			Value: p.Sprintf("%d", gamesPlayed.TotalGamesPlayed),
		},
		{
			Name:  "Total Players in Games",
			Value: p.Sprintf("%d", gamesPlayed.TotalPlayers),
		},
	}
	if startTime.Before(today) {
		fields = append(fields, discord.EmbedField{
			Name:  "Average Games Per Day",
			Value: p.Sprintf("%.0f", math.Round(gamesPlayed.AverageGamesPerDay)),
		})
	}
	fields = append(fields,
		discord.EmbedField{
			Name:  "Average Players Per Game",
			Value: p.Sprintf("%.2f", gamesPlayed.AveragePlayersPerGame),
		},
		discord.EmbedField{
			Name:  "Average Games Per Player",
			Value: p.Sprintf("%.2f", gamesPlayed.AverageGamesPerPlayer),
		},
	)

	return e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{
			{
				Type:   discord.EmbedTypeRich,
				Title:  title,
				Fields: fields,
			},
		},
	})
}

// stats handles the /stats command.
func stats(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if disgobot.IsShuttingDown(e) {
		return disgobot.ErrUnableToProcessCommand
	}

	switch slashSubCommandName(data) {
	case "played":
		return playerGames(data, e)
	case "active":
		return activePlayers(data, e)
	default:
		return e.CreateMessage(discord.MessageCreate{
			Content: "Invalid command.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}
}

// playerGames handles the /stats played command.
func playerGames(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	p := message.NewPrinter(language.AmericanEnglish)
	titleCaser := cases.Title(language.AmericanEnglish)
	today := today()

	memberID := member.User.ID
	game := stringValue(data, "game")
	if optionMemberID, ok := userIDValue(data, "user"); ok {
		memberID = optionMemberID.ID()
	}

	guildID := getGuildID(e)
	guildMember, _ := guild.GetMemberByID(discordid.NewSnowflakeID(member.GuildID), discordid.NewSnowflakeID(memberID))
	if guildMember == nil {
		guildMember = guild.GetMember(discordid.NewSnowflakeID(member.GuildID), &member.Member)
	}

	ps, _ := getAggregatePlayerStats(guildID, memberID.String(), game)
	if ps == nil {
		memberName := memberID.String()
		if guildMember != nil {
			memberName = guildMember.Name
		}

		var content string
		if game == "" || game == "all" {
			content = p.Sprintf("No player stats found for %s", memberName)
		} else {
			content = p.Sprintf("No player stats found for %s in the %s game.", memberName, game)
		}
		return e.CreateMessage(discord.MessageCreate{
			Content: content,
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	memberName := memberID.String()
	if guildMember != nil {
		memberName = guildMember.Name
	}

	firstPlayedDate := fmtDuration(today.Sub(ps.FirstPlayed))
	if firstPlayedDate != "Today" {
		firstPlayedDate += " Ago"
	}
	lastPlayedDate := fmtDuration(today.Sub(ps.LastPlayed))
	if lastPlayedDate != "Today" {
		lastPlayedDate += " Ago"
	}

	title := "Games Played"
	if game != "" && game != "all" {
		title = "Games Played For " + game
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{
			{
				Type:  discord.EmbedTypeRich,
				Title: titleCaser.String(title),
				Fields: []discord.EmbedField{
					{
						Name:  "Member",
						Value: p.Sprintf("%s", memberName),
					},
					{
						Name:  "First Played",
						Value: firstPlayedDate,
					},
					{
						Name:  "Last Played",
						Value: lastPlayedDate,
					},
					{
						Name:  "Games Played",
						Value: p.Sprintf("%d", ps.NumberOfTimesPlayed),
					},
				},
			},
		},
		Flags: discord.MessageFlagEphemeral,
	})
}

// activePlayers handles the /stats active command.
func activePlayers(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	titleCaser := cases.Title(language.AmericanEnglish)
	p := message.NewPrinter(language.AmericanEnglish)

	guildID := getGuildID(e)
	game := stringValue(data, "game")

	playerStats := getPlayerStatsForMostActiveMembers(guildID, game)

	guildMember := guild.GetMember(discordid.NewSnowflakeID(member.GuildID), &member.Member)
	if guildMember != nil {
		_ = guildMember.Update(&member.Member)
	}

	var title string
	if game == "" || game == "all" {
		title = titleCaser.String("Most Active Players")
	} else {
		title = titleCaser.String(p.Sprintf("Most Active Players for %ss", game))
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds: formatPlayerStats(title, playerStats),
		Flags:  discord.MessageFlagEphemeral,
	})
}

// formatPlayerStats formats the player stats to be sent to a Discord server.
func formatPlayerStats(title string, playerStats []*PlayerStats) []discord.Embed {
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
			Header: tw.CellConfig{
				Padding:    tw.CellPadding{Global: tw.Padding{Left: "", Right: "", Top: "", Bottom: ""}},
				Formatting: tw.CellFormatting{AutoWrap: tw.WrapNone},
				Alignment:  tw.CellAlignment{Global: tw.AlignLeft},
			},
		}),
	)
	defer func(table *tablewriter.Table) {
		if err := table.Close(); err != nil {
			slog.Error("failed to close the table", slog.Any("error", err))
		}
	}(table)

	table.Header([]string{"#", "Name", "Games"})

	p := message.NewPrinter(language.AmericanEnglish)
	for i, ps := range playerStats {
		memberName := ps.MemberID.String()
		member, _ := guild.GetMemberByID(ps.GuildID, ps.MemberID)
		if member != nil {
			memberName = member.Name
		}

		data := []string{strconv.Itoa(i + 1), memberName, p.Sprintf("%d", ps.NumberOfTimesPlayed)}
		if err := table.Append(data); err != nil {
			slog.Error("failed to append data to the table", slog.Any("error", err))
		}
	}
	if err := table.Render(); err != nil {
		slog.Error("failed to render the table", slog.Any("error", err))
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

// getGuildID returns the guild ID from the interaction.
func getGuildID(e *handler.CommandEvent) string {
	guildID := os.Getenv("DISCORD_STATS_GUILDID")
	if guildID != "" {
		return guildID
	}

	member := e.Member()
	if member == nil {
		return ""
	}

	return member.GuildID.String()
}

func slashSubCommandName(data discord.SlashCommandInteractionData) string {
	if data.SubCommandName == nil {
		return ""
	}

	return *data.SubCommandName
}

// userIDValue returns the user ID from the interaction.
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

// stringValue returns the string value from the interaction.
func stringValue(data discord.SlashCommandInteractionData, name string) string {
	value, ok := data.Options[name]
	if !ok {
		return ""
	}

	return strings.TrimSpace(fmt.Sprint(value))
}
