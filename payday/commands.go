package payday

import (
	"goblin2/bank"
	"goblin2/discordid"
	"goblin2/internal/format"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	memberCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "payday",
			Description: "Deposits your daily check into your bank account.",
		},
		discord.SlashCommandCreate{
			Name:        "payday-stats",
			Description: "View your payday statistics.",
		},
	}
)

// payday handles the `/payday` command.
func payday(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	p := message.NewPrinter(language.AmericanEnglish)

	payday := GetPayday(discordid.NewSnowflakeID(member.GuildID))
	paydayAccount := payday.GetAccount(discordid.NewSnowflakeID(member.User.ID))

	if paydayAccount.getNextPayday().After(time.Now()) {
		remainingTime := time.Until(paydayAccount.NextPayday)
		return e.CreateMessage(discord.MessageCreate{
			Content: p.Sprintf("You can't get another payday yet. You need to wait %s.", format.Duration(remainingTime)),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	paydayAmount := paydayAccount.getPayAmount()

	account := bank.GetAccount(discordid.NewSnowflakeID(member.GuildID), discordid.NewSnowflakeID(member.User.ID))
	if err := account.Deposit(paydayAmount); err != nil {
		slog.Error("error depositing data in the account",
			slog.Any("guildID", member.GuildID),
			slog.Any("memberID", member.User.ID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Unable to deposit your payday into your bank account.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	paydayAccount.TotalAmountPaid += paydayAmount
	paydayAccount.setNextPayday(payday.PaydayFrequency)

	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("You deposited your check of %d into your bank account. You now have %d credits.", paydayAmount, account.CurrentBalance),
		Flags:   discord.MessageFlagEphemeral,
	})
}

// showStats handles the `/payday-stats` command.
func showStats(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	p := message.NewPrinter(language.AmericanEnglish)

	payday := GetPayday(discordid.NewSnowflakeID(member.GuildID))
	paydayAccount := payday.GetAccount(discordid.NewSnowflakeID(member.User.ID))
	currentStreak := paydayAccount.CurrentStreak
	maxStreak := paydayAccount.MaxStreak
	pay := paydayAccount.getPayAmount()
	nextPayday := paydayAccount.getNextPayday().Format(time.DateTime)

	inline := true

	embeds := []discord.Embed{
		{
			Type:        discord.EmbedTypeRich,
			Title:       "Payday Stats",
			Description: "Here are your payday statistics.",
			Color:       0x00ff00,
			Fields: []discord.EmbedField{
				{
					Name:   "Payday Amount",
					Value:  p.Sprintf("%d", pay),
					Inline: &inline,
				},
				{
					Name:   "Current Streak",
					Value:  p.Sprintf("%d", currentStreak),
					Inline: &inline,
				},
				{
					Name:   "Max Streak",
					Value:  p.Sprintf("%d", maxStreak),
					Inline: &inline,
				},
				{
					Name:   "Next Payday",
					Value:  nextPayday,
					Inline: &inline,
				},
			},
		},
	}

	slog.Debug("`/payday-stats` command",
		slog.Any("guildID", member.GuildID),
		slog.Any("memberID", member.User.ID),
	)

	return e.CreateMessage(discord.MessageCreate{
		Embeds: embeds,
		Flags:  discord.MessageFlagEphemeral,
	})
}
