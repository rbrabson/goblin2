package bank

import (
	"fmt"
	"goblin2/internal/discordid"
	"log/slog"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	adminCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "bank-admin",
			Description: "Commands used to interact with the economy for this server.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "account",
					Description: "Sets the amount of credits for a given member.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionUser{
							Name:        "user",
							Description: "The member or member ID.",
							Required:    true,
						},
						discord.ApplicationCommandOptionInt{
							Name:        "amount",
							Description: "The amount to set the account to.",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "add",
					Description: "Adds credits to a given member's bank account.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionUser{
							Name:        "user",
							Description: "The member or member ID.",
							Required:    true,
						},
						discord.ApplicationCommandOptionInt{
							Name:        "amount",
							Description: "The amount to add to the account.",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "balance",
					Description: "Sets the default balance for the bank for the server.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionInt{
							Name:        "value",
							Description: "The default balance for the bank for the server.",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "name",
					Description: "Sets the name of the bank for the server.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "value",
							Description: "The name of the bank for the server.",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "currency",
					Description: "Sets the currency for the server.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "value",
							Description: "The currency to set for the server.",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "info",
					Description: "Gets information about the banking system configuration.",
				},
			},
		},
	}

	memberCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "bank",
			Description: "Commands used to interact with the economy for this server.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "account",
					Description: "Bank account balance for the member.",
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

// accountHandler gets information about a member's bank account.
func accountHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	p := message.NewPrinter(language.AmericanEnglish)

	member := e.Member()
	guildID := discordid.NewSnowflakeID(member.GuildID)
	memberID := discordid.NewSnowflakeID(member.User.ID)

	if optionMemberID, ok := userIDValue(data, "user"); ok {
		memberID = optionMemberID
	}

	account := GetAccount(guildID, memberID)
	if account == nil {
		slog.Error("account not found",
			slog.Any("guild_id", guildID),
			slog.Any("member_id", memberID))
		return e.CreateMessage(discord.MessageCreate{
			Content: p.Sprintf("An account for member %s does not exist.", memberID),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	content := p.Sprintf(
		"**Current Balance**: %d\n**Monthly Balance**: %d\n**Lifetime Balance**: %d\n**Created**: %s\n",
		account.CurrentBalance,
		account.MonthlyBalance,
		account.LifetimeBalance,
		account.CreatedAt.Format("2006-01-02 15:04:05 MST"),
	)

	return e.CreateMessage(discord.MessageCreate{
		Content: content,
		Flags:   discord.MessageFlagEphemeral,
	})
}

// setAccountBalanceHandler sets the balance of the account for the member of the guild to the specified amount.
func setAccountBalanceHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	slog.Warn("TBD: bank-admin account handler",
		slog.Any("options", data.Options),
	)

	return e.CreateMessage(discord.MessageCreate{
		Content: "stub: bank-admin account",
		Flags:   discord.MessageFlagEphemeral,
	})
}

// addAccountBalanceHandler adds the amount to the balance of the account for the member of the guild.
func addAccountBalanceHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	slog.Warn("TBD: bank-admin add handler",
		slog.Any("options", data.Options),
	)

	return e.CreateMessage(discord.MessageCreate{
		Content: "stub: bank-admin add",
		Flags:   discord.MessageFlagEphemeral,
	})
}

// setDefaultBalanceHandler sets the default balance for the bank for the guild.
func setDefaultBalanceHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	p := message.NewPrinter(language.AmericanEnglish)

	balance := intValue(data, "value")
	b := GetBank(discordid.NewSnowflakeID(e.Member().GuildID))
	b.SetDefaultBalance(balance)

	slog.Debug("/bank-admin balance",
		slog.Any("guildID", e.Member().GuildID),
		slog.Int("balance", balance),
	)

	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Bank default balance was set to %d", b.DefaultBalance),
	})
}

// setBankNameHandler sets the name of the bank for the guild.
func setBankNameHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	p := message.NewPrinter(language.AmericanEnglish)

	name := strings.TrimSpace(stringValue(data, "value"))
	b := GetBank(discordid.NewSnowflakeID(e.Member().GuildID))
	b.SetName(name)

	slog.Debug("/bank-admin name",
		slog.Any("guildID", e.Member().GuildID),
		slog.String("name", name),
	)

	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Bank name was set to %s", b.Name),
	})
}

// setBankCurrencyHandler sets the name of the currency used by the bank for the guild.
func setBankCurrencyHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	p := message.NewPrinter(language.AmericanEnglish)

	currency := strings.TrimSpace(stringValue(data, "value"))
	b := GetBank(discordid.NewSnowflakeID(e.Member().GuildID))
	b.SetCurrency(currency)

	slog.Debug("/bank-admin currency",
		slog.Any("guildID", e.Member().GuildID),
		slog.String("currency", currency),
	)

	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Bank currency was set to %s", b.Currency),
	})
}

// getBankInfoHandler gets information about the bank for the guild.
func getBankInfoHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	p := message.NewPrinter(language.AmericanEnglish)

	b := GetBank(discordid.NewSnowflakeID(e.Member().GuildID))

	content := p.Sprintf("**Bank Name**: %s\n**Currency**: %s\n**Default Balance**: %d\n",
		b.Name,
		b.Currency,
		b.DefaultBalance,
	)

	return e.CreateMessage(discord.MessageCreate{
		Content: content,
		Flags:   discord.MessageFlagEphemeral,
	})
}

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

// intValue returns the int value of the option with the given name.
func intValue(data discord.SlashCommandInteractionData, name string) int {
	value, ok := data.Options[name]
	if !ok {
		return 0
	}

	var result int
	if _, err := fmt.Sscan(fmt.Sprint(value), &result); err != nil {
		slog.Warn("unable to parse int option",
			slog.String("name", name),
			slog.Any("value", value),
			slog.Any("error", err),
		)
		return 0
	}

	return result
}
