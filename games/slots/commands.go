package slots

import (
	"fmt"
	"goblin2/bank"
	"goblin2/disgobot"
	"goblin2/internal/discordid"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	rslots "github.com/rbrabson/slots"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	memberCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "slots",
			Description: "Interacts with the slot machine.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "play",
					Description: "Play the slot machine.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionInt{
							Name:        "bet",
							Description: "The amount to bet on the slot machine.",
							Required:    true,
							Choices: []discord.ApplicationCommandOptionChoiceInt{
								{
									Name:  "500",
									Value: 500,
								},
								{
									Name:  "300",
									Value: 300,
								},
								{
									Name:  "100",
									Value: 100,
								},
							},
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "paytable",
					Description: "Get the pay table for the slot machine.",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "stats",
					Description: "Shows a user's stats.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionUser{
							Name:        "user",
							Description: "The member or member ID.",
							Required:    false,
						},
					},
				},
			},
		},
	}
)

// playSlots handles the `/slots play` command.
func playSlots(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	bet := data.Int("bet")
	guildID := discordid.NewSnowflakeID(member.GuildID)
	memberID := discordid.NewSnowflakeID(member.User.ID)

	slog.Debug("`/slots play` command",
		slog.Any("guildID", guildID),
		slog.Any("memberID", memberID),
		slog.Int("bet", bet),
	)

	config := GetConfig()
	slotsMember := GetMember(guildID, memberID)
	if slotsMember.IsInCooldown(config) {
		remaining := slotsMember.GetCooldownRemaining(config)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("You are on cooldown. Please wait %d seconds before playing again.", int(remaining.Seconds())+1),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	account := bank.GetAccount(guildID, memberID)
	if err := account.Withdraw(bet); err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("You are unable to play slots, Error: %s", err.Error()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	sm := GetSlotMachine()
	spinResult := sm.Spin(bet)

	slotsMember.AddResults(spinResult)

	if spinResult.Payout > 0 {
		if err := account.Deposit(spinResult.Payout); err != nil {
			slog.Error("error depositing slots winnings to account",
				slog.Any("guildID", guildID),
				slog.Any("memberID", memberID),
				slog.Int("payout", spinResult.Payout),
				slog.Any("error", err),
			)
		}
	}

	symbols := sm.symbols
	spinMsg := symbols["Blank"].Emoji
	for i, symbol := range spinResult.TopLine {
		if i != 0 {
			spinMsg += " | "
		}
		spinMsg += sm.symbols[symbol].Emoji
	}
	spinMsg += "\n" + symbols["Right Arrow"].Emoji
	for i, symbol := range spinResult.Payline {
		if i != 0 {
			spinMsg += " | "
		}
		spinMsg += sm.symbols[symbol].Emoji
	}
	spinMsg += "\n" + symbols["Blank"].Emoji
	for i, symbol := range spinResult.BottomLine {
		if i != 0 {
			spinMsg += " | "
		}
		spinMsg += symbols[symbol].Emoji
	}
	spinMsg += "\n"

	var embedColor int
	var resultTitle string
	var resultDescription string

	if spinResult.Payout > 0 {
		embedColor = 0x00ff00
		resultTitle = "🎉 " + spinResult.Message
		resultDescription = p.Sprintf("You won **%d** coins!", spinResult.Payout)
	} else {
		embedColor = 0xff0000
		resultTitle = "💸 No Win"
		resultDescription = "Better luck next time!"
	}

	inline := false
	embed := discord.Embed{
		Type:        discord.EmbedTypeRich,
		Title:       "🎰 Slot Machine 🎰",
		Description: p.Sprintf("<@%s> bet **%d** coins", member.User.ID, spinResult.Bet),
		Color:       embedColor,
		Fields: []discord.EmbedField{
			{
				Value:  spinMsg,
				Inline: &inline,
			},
			{
				Name:   resultTitle,
				Value:  resultDescription,
				Inline: &inline,
			},
		},
		Timestamp: new(time.Now()),
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
}

// showStats handles the `/slots stats` command.
func showStats(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	memberID := member.User.ID
	if option, ok := data.Options["user"]; ok {
		parsed, err := discordid.SnowflakeIDFromString(fmt.Sprint(option))
		if err != nil {
			slog.Warn("unable to parse slots stats user option",
				slog.Any("guildID", member.GuildID),
				slog.Any("memberID", member.User.ID),
				slog.Any("option", option),
				slog.Any("error", err),
			)
			return e.CreateMessage(discord.MessageCreate{
				Content: "The user to get stats for was not found. Please try again.",
				Flags:   discord.MessageFlagEphemeral,
			})
		}
		memberID = parsed.ID()
	}

	guildID := member.GuildID

	slog.Debug("`/slots stats` command",
		slog.Any("guildID", guildID),
		slog.Any("memberID", memberID),
	)

	slotsMember := GetMember(discordid.NewSnowflakeID(guildID), discordid.NewSnowflakeID(memberID))

	inline := true
	embed := discord.Embed{
		Type:        discord.EmbedTypeRich,
		Title:       "Slot Machine Stats",
		Description: p.Sprintf("Here are the stats for <@%s>:", memberID),
		Color:       0x5865F2,
		Fields: []discord.EmbedField{
			{
				Name:   "Total Wins",
				Value:  p.Sprintf("%d", slotsMember.TotalWins),
				Inline: &inline,
			},
			{
				Name:   "Total Losses",
				Value:  p.Sprintf("%d", slotsMember.TotalLosses),
				Inline: &inline,
			},
			{
				Name:   "Winning Percentage",
				Value:  p.Sprintf("%.1f%%", (float64(slotsMember.TotalWins)/float64(slotsMember.TotalWins+slotsMember.TotalLosses))*100),
				Inline: &inline,
			},
			{
				Name:   "Total Bet",
				Value:  p.Sprintf("%d", slotsMember.TotalBet),
				Inline: &inline,
			},
			{
				Name:   "Total Winnings",
				Value:  p.Sprintf("%d", slotsMember.TotalWinnings),
				Inline: &inline,
			},
			{
				Name:   "Returns",
				Value:  p.Sprintf("%.1f%%", (float64(slotsMember.TotalWinnings)/float64(slotsMember.TotalBet))*100),
				Inline: &inline,
			},
			{
				Name:   "Current Winning Streak",
				Value:  p.Sprintf("%d", slotsMember.CurrentWinStreak),
				Inline: &inline,
			},
			{
				Name:   "Longest Winning Streak",
				Value:  p.Sprintf("%d", slotsMember.LongestWinStreak),
				Inline: &inline,
			},
			{
				Name:   "Max Win",
				Value:  p.Sprintf("%d", slotsMember.MaxWin),
				Inline: &inline,
			},
			{
				Name:   "Current Losing Streak",
				Value:  p.Sprintf("%d", slotsMember.CurrentLosingStreak),
				Inline: &inline,
			},
			{
				Name:   "Longest Losing Streak",
				Value:  p.Sprintf("%d", slotsMember.LongestLosingStreak),
				Inline: &inline,
			},
		},
		Timestamp: new(time.Now()),
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
		Flags:  discord.MessageFlagEphemeral,
	})
}

// payTable handles the `/slots paytable` command.
func payTable(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	guildID := member.GuildID
	payTable := GetPayoutTable()

	slog.Debug("`/slots paytable` command",
		slog.Any("guildID", guildID),
	)

	embeds := make([]discord.Embed, 0, 1)
	if payTable != nil {
		inline := false
		embed := discord.Embed{
			Type:        discord.EmbedTypeRich,
			Title:       "Slot Machine Pay Table",
			Description: "Here are the possible winning combinations and their payouts.",
			Color:       0x00ff00,
			Fields:      make([]discord.EmbedField, 0, len(payTable)),
		}

		sm := GetSlotMachine()
		twoConsecutiveTroops := false
		for _, payout := range payTable {
			payoutStr := strconv.FormatFloat(payout.Payout, 'f', -1, 64)
			betPayouts := p.Sprintf("Payout %s:%d\n", payoutStr, payout.Bet)

			name := getPayoutDisplayMessage(payout.Win, sm.symbols)
			if name == "two consecutive troops" {
				if twoConsecutiveTroops {
					continue
				}
				twoConsecutiveTroops = true
			}

			embed.Fields = append(embed.Fields, discord.EmbedField{
				Name:   name,
				Value:  betPayouts,
				Inline: &inline,
			})
		}

		embeds = append(embeds, embed)
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: "Pay table:",
		Embeds:  embeds,
		Flags:   discord.MessageFlagEphemeral,
	})
}

// getPayoutDisplayMessage creates a formatted string for displaying payout information.
func getPayoutDisplayMessage(spin []string, symbolTable SymbolTable) string {
	if len(spin) == 1 {
		return spin[0]
	}

	symbols := make([]string, 0, len(spin))
	for _, symbol := range spin {
		if lookup, ok := symbolTable[symbol]; ok {
			symbols = append(symbols, lookup.Emoji)
			continue
		}

		switch symbol {
		case rslots.Any:
			symbols = append(symbols, "any symbol")
		case rslots.AnySeven:
			symbols = append(symbols, "hero")
		case rslots.AnyBar:
			symbols = append(symbols, "basic troop")
		case rslots.AnyRed:
			aq := symbolTable["red 7"]
			archer := symbolTable["1 bar"]
			symbols = append(symbols, fmt.Sprintf("%s/%s", aq.Emoji, archer.Emoji))
		case rslots.AnyWhite:
			gw := symbolTable["white 7"]
			wizard := symbolTable["2 bar"]
			symbols = append(symbols, fmt.Sprintf("%s/%s", gw.Emoji, wizard.Emoji))
		case rslots.AnyBlue:
			bk := symbolTable["blue 7"]
			barbarian := symbolTable["3 bar"]
			symbols = append(symbols, fmt.Sprintf("%s/%s", bk.Emoji, barbarian.Emoji))
		case rslots.MatchingNonBlank:
			symbols = append(symbols, "any troop")
		default:
			spell := symbolTable["blank"]
			symbols = append(symbols, spell.Emoji)
		}
	}
	display := strings.Join(symbols, " | ")
	switch display {
	case "any troop | any troop | any symbol", "any symbol | any troop | any troop":
		display = "two consecutive troops"
	}
	return display
}
