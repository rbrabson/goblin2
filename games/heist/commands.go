package heist

import (
	"fmt"
	"goblin2/bank"
	"goblin2/guild"
	"goblin2/internal/discordid"
	"goblin2/internal/format"
	"log/slog"
	"math"
	"strings"
	"time"

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
	MaxWinningsPerPage = 30
)

var (
	adminCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "heist-admin",
			Description: "Heist admin commands.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "boost",
					Description: "Enables or disables boosts for the heist game.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionBool{
							Name:        "enable",
							Description: "The status of the boost.",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "clear",
					Description: "Clears the criminal settings for the user.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionUser{
							Name:        "user",
							Description: "The member to clear.",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommandGroup{
					Name:        "config",
					Description: "Configures the Heist bot.",
					Options: []discord.ApplicationCommandOptionSubCommand{
						{
							Name:        "info",
							Description: "Returns the configuration information for the server.",
						},
						{
							Name:        "bail",
							Description: "Sets the base cost of bail.",
							Options: []discord.ApplicationCommandOption{
								discord.ApplicationCommandOptionInt{
									Name:        "amount",
									Description: "The base cost of bail.",
									Required:    true,
								},
							},
						},
						{
							Name:        "boost",
							Description: "Sets the boost percentage.",
							Options: []discord.ApplicationCommandOption{
								discord.ApplicationCommandOptionFloat{
									Name:        "percentage",
									Description: "The percentage to increase payouts when boosts are enabled.",
									Required:    true,
								},
							},
						},
						{
							Name:        "cost",
							Description: "Sets the cost to plan or join a heist.",
							Options: []discord.ApplicationCommandOption{
								discord.ApplicationCommandOptionInt{
									Name:        "amount",
									Description: "The cost to plan or join a heist.",
									Required:    true,
								},
							},
						},
						{
							Name:        "death",
							Description: "Sets how long players remain dead.",
							Options: []discord.ApplicationCommandOption{
								discord.ApplicationCommandOptionInt{
									Name:        "time",
									Description: "The time the player remains dead, in seconds.",
									Required:    true,
								},
							},
						},
						{
							Name:        "patrol",
							Description: "Sets the time the authorities will prevent a new heist.",
							Options: []discord.ApplicationCommandOption{
								discord.ApplicationCommandOptionInt{
									Name:        "time",
									Description: "The time the authorities will patrol, in seconds.",
									Required:    true,
								},
							},
						},
						{
							Name:        "sentence",
							Description: "Sets the base apprehension time when caught.",
							Options: []discord.ApplicationCommandOption{
								discord.ApplicationCommandOptionInt{
									Name:        "time",
									Description: "The base time, in seconds.",
									Required:    true,
								},
							},
						},
						{
							Name:        "boost-vault-recovery",
							Description: "Sets the percentage to multiply the vault recovery percent by when boosts are enabled.",
							Options: []discord.ApplicationCommandOption{
								discord.ApplicationCommandOptionInt{
									Name:        "percent",
									Description: "The percentage to multiply the vault recovery percent by when boosts are enabled.",
									Required:    true,
								},
							},
						},
						{
							Name:        "wait",
							Description: "Sets how long players can gather others for a heist.",
							Options: []discord.ApplicationCommandOption{
								discord.ApplicationCommandOptionInt{
									Name:        "time",
									Description: "The time to wait for players to join the heist, in seconds.",
									Required:    true,
								},
							},
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "reset",
					Description: "Resets a new heist that is hung.",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "vault-reset",
					Description: "Resets the vaults to their maximum value.",
				},
			},
		},
	}

	memberCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "heist",
			Description: "Heist game commands.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "bail",
					Description: "Bail a player out of jail.",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionUser{
							Name:        "user",
							Description: "The member to bail out of jail. Defaults to you.",
							Required:    false,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "stats",
					Description: "Shows a user's stats.",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "start",
					Description: "Plans a new heist.",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "targets",
					Description: "Gets the list of available heist targets.",
				},
			},
		},
	}
)

// startHeist plans a new heist.
func startHeist(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	member := e.Member()
	if member == nil {
		return serverOnly(e)
	}

	guildMember := resolvedGuildMember(member)

	heist, err := createHeist(e, guildMember)
	if err != nil {
		slog.Warn("unable to create the heist", slog.Any("error", err))
		return e.CreateMessage(discord.MessageCreate{
			Content: firstToUpper(err.Error()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	slog.Info("heist created",
		slog.Any("guildID", heist.GuildID),
		slog.String("organizer", guildMember.Name),
	)

	if err := e.CreateMessage(discord.MessageCreate{
		Content: "Planning a " + heist.config.Theme.Heist + "...",
	}); err != nil {
		heist.Cancel()
		return err
	}

	heist.interaction = e

	if err := heistMessage(heist); err != nil {
		slog.Error("failed to send heist message", slog.Any("error", err))
		heist.Cancel()
		return err
	}

	go runHeist(e, heist)

	return nil
}

func runHeist(e *handler.CommandEvent, heist *Heist) {
	defer heist.End()

	waitForMembersToJoin(heist)

	if err := heistMessage(heist); err != nil {
		slog.Error("failed to update heist message", slog.Any("error", err))
	}

	res, err := heist.Start()
	if err != nil {
		slog.Info("failed to start the heist",
			slog.Any("guildID", heist.GuildID),
			slog.Any("error", err),
		)

		if msgErr := heistMessage(heist); msgErr != nil {
			slog.Error("failed to update heist message", slog.Any("error", msgErr))
		}

		if _, sendErr := e.Client().Rest.CreateMessage(e.Channel().ID(), discord.MessageCreate{
			Content: firstToUpper(err.Error()),
		}); sendErr != nil {
			slog.Error("failed to send heist failure message", slog.Any("error", sendErr))
		}
		return
	}

	if err := heistMessage(heist); err != nil {
		slog.Error("failed to update heist message", slog.Any("error", err))
	}

	if err := sendHeistResults(e, heist, res); err != nil {
		slog.Error("failed to send heist results", slog.Any("error", err))
		return
	}

	if res.Target != nil {
		res.Target.Vault -= res.TotalStolen
		if res.Target.Vault < 0 {
			res.Target.Vault = 0
		}
		writeTarget(res.Target)
	}

	heist.State = Completed

	if err := heistMessage(heist); err != nil {
		slog.Error("failed to update heist message", slog.Any("error", err))
	}

	slog.Info("heist completed",
		slog.Any("guildID", heist.GuildID),
		slog.Int("stolenAmount", res.TotalStolen),
	)
}

// createHeist creates a new heist and sets the organizer. It also withdraws the cost of planning the heist
// from the organizer's account.
func createHeist(e *handler.CommandEvent, guildMember *guild.Member) (*Heist, error) {
	member := e.Member()

	heist, err := NewHeist(discordid.NewSnowflakeID(member.GuildID), discordid.NewSnowflakeID(member.User.ID))
	if err != nil {
		slog.Warn("unable to create the heist", slog.Any("error", err))
		return nil, err
	}

	account := bank.GetAccount(heist.GuildID, heist.Organizer.MemberID)
	if err := account.Withdraw(heist.config.HeistCost); err != nil {
		slog.Error("failed to withdraw heist cost", slog.Any("error", err))
		heist.Cancel()
		return nil, ErrNotEnoughCredits{CreditsNeeded: heist.config.HeistCost}
	}

	heist.Organizer.SetGuildMember(guildMember)
	heist.interaction = e

	return heist, nil
}

// waitForMembersToJoin waits until the planning stage for the heist expires.
func waitForMembersToJoin(heist *Heist) {
	startTime := heist.StartTime.Add(heist.config.WaitTime)

	slog.Debug("wait for heist to start",
		slog.Any("guildID", heist.GuildID),
		slog.Time("currentTime", time.Now()),
		slog.Duration("configWaitTime", heist.config.WaitTime),
		slog.Time("startTime", startTime),
	)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		if err := heistMessage(heist); err != nil {
			slog.Error("failed to update heist message", slog.Any("error", err))
		}
		if time.Now().After(startTime) {
			break
		}
		<-ticker.C
	}

	slog.Debug("wait for the heist to start is over", slog.Any("guildID", heist.GuildID))
}

// sendHeistResults runs the heist and sends the results to the channel.
func sendHeistResults(e *handler.CommandEvent, heist *Heist, res *Result) error {
	if err := heistMessage(heist); err != nil {
		slog.Error("failed to update heist message", slog.Any("error", err))
	}

	p := message.NewPrinter(language.AmericanEnglish)
	if _, err := e.Client().Rest.CreateMessage(e.Channel().ID(), discord.MessageCreate{
		Content: p.Sprintf("The %s is starting with %d members.", heist.config.Theme.Heist, len(heist.Crew)),
	}); err != nil {
		slog.Error("failed to send message",
			slog.Any("channelID", e.Channel().ID()),
			slog.Any("error", err),
		)
		return err
	}

	time.Sleep(3 * time.Second)

	if err := heistMessage(heist); err != nil {
		slog.Error("failed to update heist message", slog.Any("error", err))
	}

	return sendMemberResults(e, res)
}

// sendMemberResults sends the results of the heist to the channel.
func sendMemberResults(e *handler.CommandEvent, res *Result) error {
	p := message.NewPrinter(language.AmericanEnglish)
	gID := guildID(e)
	channelID := e.Channel().ID()

	theme := GetTheme(gID)
	if theme == nil {
		slog.Error("failed to get heist theme", slog.Any("guildID", gID))
		_, err := e.Client().Rest.CreateMessage(channelID, discord.MessageCreate{
			Content: "Internal error: failed to get the heist theme",
		})
		return err
	}

	if res.Target == nil {
		_, err := e.Client().Rest.CreateMessage(channelID, discord.MessageCreate{
			Content: "Oh no! There are no targets!",
		})
		return err
	}

	slog.Debug("hitting target",
		slog.String("target", res.Target.Name),
		slog.Any("guildID", gID),
	)

	msg := p.Sprintf("The %s has decided to hit **%s**.", theme.Crew, res.Target.Name)
	if _, err := e.Client().Rest.CreateMessage(channelID, discord.MessageCreate{Content: msg}); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)

	for _, result := range res.AllResults {
		name := result.Player.MemberID.String()
		if result.Player.guildMember != nil && result.Player.guildMember.Name != "" {
			name = result.Player.guildMember.Name
		}

		name = strings.ReplaceAll(name, "#", "")
		name = strings.ReplaceAll(name, "*", "")

		msg = p.Sprintf(result.Message+"\n", "**"+name+"**")
		if result.Status == Apprehended {
			msg += p.Sprintf("`%s dropped out of the game.`", name)
		}

		if _, err := e.Client().Rest.CreateMessage(channelID, discord.MessageCreate{Content: msg}); err != nil {
			return err
		}
		time.Sleep(3 * time.Second)
	}

	if len(res.Escaped) == 0 {
		_, err := e.Client().Rest.CreateMessage(channelID, discord.MessageCreate{
			Content: "\nNo one made it out safe.",
		})
		return err
	}

	if _, err := e.Client().Rest.CreateMessage(channelID, discord.MessageCreate{
		Content: "\nThe raid is now over. Distributing player spoils.",
	}); err != nil {
		return err
	}

	if err := sendWinningsTable(e, res); err != nil {
		return err
	}

	for _, result := range res.AllResults {
		result.Player.heist = result.heist

		if len(res.Escaped) > 0 && result.StolenCredits != 0 {
			account := bank.GetAccount(gID, result.Player.MemberID)
			if err := account.Deposit(result.StolenCredits + result.BonusCredits); err != nil {
				slog.Error("failed to deposit stolen credits",
					slog.Any("guildID", gID),
					slog.Any("error", err),
				)
			}
		}
	}

	return heistMessage(res.heist)
}

func sendWinningsTable(e *handler.CommandEvent, res *Result) error {
	p := message.NewPrinter(language.AmericanEnglish)
	channelID := e.Channel().ID()

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
		err := table.Close()
		if err != nil {

		}
	}(table)

	numLines := 0
	table.Header([]string{"Player", "Loot", "Bonus", "Total"})

	for _, result := range res.AllResults {
		if result.Status != Free && result.Status != Apprehended {
			continue
		}

		name := result.Player.MemberID.String()
		if result.Player.guildMember != nil && result.Player.guildMember.Name != "" {
			name = result.Player.guildMember.Name
		}

		data := []string{
			name,
			p.Sprintf("%d", result.StolenCredits),
			p.Sprintf("%d", result.BonusCredits),
			p.Sprintf("%d", result.StolenCredits+result.BonusCredits),
		}
		if err := table.Append(data); err != nil {
			slog.Error("failed to append to table", slog.Any("error", err))
		}

		numLines++
		if numLines >= MaxWinningsPerPage {
			if err := table.Render(); err != nil {
				return err
			}
			if _, err := e.Client().Rest.CreateMessage(channelID, discord.MessageCreate{
				Content: "```\n" + tableBuffer.String() + "\n```",
			}); err != nil {
				return err
			}
			table.Reset()
			tableBuffer.Reset()
			numLines = 0
		}
	}

	if numLines > 0 {
		if err := table.Render(); err != nil {
			return err
		}
		_, err := e.Client().Rest.CreateMessage(channelID, discord.MessageCreate{
			Content: "```\n" + tableBuffer.String() + "```",
		})
		return err
	}

	return nil
}

// joinHeist attempts to join a heist that is being planned.
func joinHeist(e *handler.ComponentEvent) error {
	if err := e.DeferCreateMessage(true); err != nil {
		slog.Error("failed to defer join heist component response", slog.Any("error", err))
	}

	member := e.Member()
	if member == nil {
		return updateComponentResponse(e, "This command can only be used in a server.")
	}

	heist := GetHeist(discordid.NewSnowflakeID(member.GuildID))
	if heist == nil {
		theme := GetTheme(discordid.NewSnowflakeID(member.GuildID))
		content := "No heist is being planned"
		if theme != nil {
			content = fmt.Sprintf("No %s is being planned", theme.Heist)
		}
		return updateComponentResponse(e, content)
	}

	guildMember := resolvedGuildMember(member)
	heistMember := GetMember(discordid.NewSnowflakeID(member.GuildID), discordid.NewSnowflakeID(member.User.ID))
	heistMember.SetGuildMember(guildMember)

	account := bank.GetAccount(discordid.NewSnowflakeID(member.GuildID), heistMember.MemberID)
	if err := account.Withdraw(heist.config.HeistCost); err != nil {
		slog.Error("failed to withdraw heist",
			slog.Any("guildID", member.GuildID),
			slog.Any("error", err),
		)

		return updateComponentResponse(e, fmt.Sprintf("Unable to join the heist. Error: %s", err.Error()))
	}

	if err := heist.AddCrewMember(heistMember); err != nil {
		if depositErr := account.Deposit(heist.config.HeistCost); depositErr != nil {
			slog.Error("failed to refund heist cost after join failure",
				slog.Any("guildID", member.GuildID),
				slog.Any("memberID", member.User.ID),
				slog.Any("error", depositErr),
			)
		}

		return updateComponentResponse(e, firstToUpper(err.Error()))
	}

	if err := heistMessage(heist); err != nil {
		slog.Error("failed to update heist message", slog.Any("error", err))
	}

	p := message.NewPrinter(language.AmericanEnglish)
	return updateComponentResponse(e, p.Sprintf("You have joined the %s at a cost of %d credits.", heist.config.Theme.Heist, heist.config.HeistCost))
}

func updateComponentResponse(e *handler.ComponentEvent, content string) error {
	_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
		Content: &content,
	})
	return err
}

// playerStats shows a player's heist stats.
func playerStats(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	member := e.Member()
	if member == nil {
		return serverOnly(e)
	}

	p := message.NewPrinter(language.AmericanEnglish)

	guildMember := resolvedGuildMember(member)
	player := GetMember(guildMember.GuildID, guildMember.MemberID)
	player.SetGuildMember(guildMember)

	caser := cases.Title(language.Und, cases.NoLower)
	account := bank.GetAccount(guildMember.GuildID, guildMember.MemberID)

	sentence := "None"
	if player.Status == Apprehended {
		if player.RemainingJailTime() <= 0 {
			sentence = "Served"
		} else {
			sentence = format.Duration(player.RemainingJailTime())
		}
	}

	theme := GetTheme(guildMember.GuildID)
	if theme == nil {
		slog.Error("failed to get heist theme", slog.Any("guildID", guildMember.GuildID))
		return e.CreateMessage(discord.MessageCreate{
			Content: "Internal error: failed to get the heist theme",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	inline := true
	embeds := []discord.Embed{
		{
			Type:        discord.EmbedTypeRich,
			Title:       guildMember.Name,
			Description: criminalLevelString(player.CriminalLevel),
			Fields: []discord.EmbedField{
				{Name: "Status", Value: string(player.Status), Inline: &inline},
				{Name: "Spree", Value: p.Sprintf("%d", player.Spree), Inline: &inline},
				{Name: caser.String(theme.Bail), Value: p.Sprintf("%d", player.BailCost), Inline: &inline},
				{Name: caser.String(theme.Sentence), Value: sentence, Inline: &inline},
				{Name: "Apprehended", Value: p.Sprintf("%d", player.JailCounter), Inline: &inline},
				{Name: "Total Deaths", Value: p.Sprintf("%d", player.Deaths), Inline: &inline},
				{Name: "Lifetime Apprehensions", Value: p.Sprintf("%d", player.TotalJail), Inline: &inline},
				{Name: "Credits", Value: p.Sprintf("%d", account.CurrentBalance), Inline: &inline},
			},
		},
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds: embeds,
		Flags:  discord.MessageFlagEphemeral,
	})
}

// bailoutPlayer bails a player out from jail.
func bailoutPlayer(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	member := e.Member()
	if member == nil {
		return serverOnly(e)
	}

	p := message.NewPrinter(language.AmericanEnglish)

	targetID := member.User.ID
	if user, ok := data.OptUser("user"); ok {
		targetID = user.ID
	}

	initiatingGuildMember := resolvedGuildMember(member)
	initiatingHeistMember := GetMember(discordid.NewSnowflakeID(member.GuildID), discordid.NewSnowflakeID(member.User.ID))
	initiatingHeistMember.SetGuildMember(initiatingGuildMember)

	account := bank.GetAccount(initiatingHeistMember.GuildID, initiatingHeistMember.MemberID)

	heistMember := GetMember(initiatingHeistMember.GuildID, discordid.NewSnowflakeID(targetID))
	targetGuildMember, _ := guild.GetMemberByID(discordid.NewSnowflakeID(member.GuildID), discordid.NewSnowflakeID(targetID))
	if targetGuildMember != nil {
		heistMember.SetGuildMember(targetGuildMember)
	}

	targetName := targetID.String()
	if heistMember.guildMember != nil && heistMember.guildMember.Name != "" {
		targetName = heistMember.guildMember.Name
	}

	if heistMember.Status != Apprehended {
		msg := fmt.Sprintf("%s is not in jail", targetName)
		if heistMember.MemberID.ID() == member.User.ID {
			msg = "You are not in jail"
		}
		return e.CreateMessage(discord.MessageCreate{
			Content: msg,
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	if heistMember.RemainingJailTime() <= 0 {
		msg := fmt.Sprintf("%s has already served their sentence.", targetName)
		if heistMember.MemberID.ID() == member.User.ID {
			msg = "You have already served your sentence."
		}
		heistMember.FreeMember()
		return e.CreateMessage(discord.MessageCreate{
			Content: msg,
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	if err := account.Withdraw(heistMember.BailCost); err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: p.Sprintf("You do not have enough credits to pay the bail of %d", heistMember.BailCost),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	bailCost := heistMember.BailCost
	heistMember.ReleaseOnBail()

	if heistMember.MemberID.ID() == initiatingHeistMember.MemberID.ID() {
		return e.CreateMessage(discord.MessageCreate{
			Content: p.Sprintf("Congratulations, you are now free! You spent %d credits on your bail. Enjoy your freedom while it lasts.", bailCost),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	content := p.Sprintf(
		"Congratulations, %s, %s bailed you out by spending %d credits and now you are free! Enjoy your freedom while it lasts.",
		targetName,
		initiatingGuildMember.Name,
		bailCost,
	)

	return e.CreateMessage(discord.MessageCreate{
		Content: content,
	})
}

// heistMessage sends the main message used to plan and join a heist.
func heistMessage(heist *Heist) error {
	heist.mutex.Lock()
	defer heist.mutex.Unlock()

	if heist.interaction == nil {
		return nil
	}

	var status string
	var buttonDisabled bool
	switch heist.State {
	case Planning:
		until := time.Until(heist.StartTime.Add(heist.config.WaitTime))
		status = "Starts in " + format.Duration(until)
		buttonDisabled = false
	case InProgress:
		status = "Started"
		buttonDisabled = true
	case Cancelled:
		status = "Canceled"
		buttonDisabled = true
	case Completed:
		status = "Ended"
		buttonDisabled = true
	default:
		status = "Ended"
		buttonDisabled = true
	}

	crew := make([]string, 0, len(heist.Crew))
	for _, crewMember := range heist.Crew {
		if crewMember.guildMember != nil && crewMember.guildMember.Name != "" {
			crew = append(crew, crewMember.guildMember.Name)
			continue
		}
		crew = append(crew, crewMember.MemberID.String())
	}

	caser := cases.Title(language.Und, cases.NoLower)
	p := message.NewPrinter(language.AmericanEnglish)

	organizer := heist.Organizer.MemberID.String()
	if heist.Organizer.guildMember != nil && heist.Organizer.guildMember.Name != "" {
		organizer = heist.Organizer.guildMember.Name
	}

	description := p.Sprintf(
		"A new %s is being planned by %s. You can join the %s for a cost of %d credits at any time prior to the %s starting.",
		heist.config.Theme.Heist,
		organizer,
		heist.config.Theme.Heist,
		heist.config.HeistCost,
		heist.config.Theme.Heist,
	)

	crewValue := strings.Join(crew, ", ")
	if crewValue == "" {
		crewValue = "No one has joined yet."
	}

	inline := true
	embeds := []discord.Embed{
		{
			Type:        discord.EmbedTypeRich,
			Title:       "Heist",
			Description: description,
			Fields: []discord.EmbedField{
				{
					Name:   "Status",
					Value:  status,
					Inline: &inline,
				},
				{
					Name:   fmt.Sprintf("%s (%d members)", caser.String(heist.config.Theme.Crew), len(crew)),
					Value:  crewValue,
					Inline: &inline,
				},
			},
		},
	}

	components := []discord.LayoutComponent{
		discord.ActionRowComponent{
			Components: []discord.InteractiveComponent{
				discord.ButtonComponent{
					Label:    "Join",
					Style:    discord.ButtonStyleSuccess,
					Disabled: buttonDisabled,
					CustomID: fmt.Sprintf("/heist/join/%s", heist.GuildID),
				},
			},
		},
	}

	_, err := heist.interaction.Client().Rest.UpdateInteractionResponse(
		heist.interaction.ApplicationID(),
		heist.interaction.Token(),
		discord.MessageUpdate{
			Content:    new(""),
			Embeds:     &embeds,
			Components: &components,
		},
	)
	return err
}

/******** ADMIN COMMANDS ********/

// enableBoost enables or disables the boost for the heist game.
func enableBoost(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	gID := guildID(e)
	boostEnabled := data.Bool("enable")

	config := GetConfig(gID)
	config.BoostEnabled = boostEnabled
	writeConfig(config)

	content := "Boosts have been disabled for the heist game."
	if boostEnabled {
		content = "Boosts have been enabled for the heist game."
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: content,
	})
}

// resetHeist resets the heist in case it hangs.
func resetHeist(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	gID := guildID(e)

	heistLock.Lock()
	heist := currentHeists[gID]
	delete(currentHeists, gID)
	heistLock.Unlock()

	if heist == nil {
		theme := GetTheme(gID)
		content := "No heist is being planned"
		if theme != nil {
			content = fmt.Sprintf("No %s is being planned", theme.Heist)
		}
		return e.CreateMessage(discord.MessageCreate{
			Content: content,
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	heist.End()
	if err := heistMessage(heist); err != nil {
		slog.Error("failed to update heist message", slog.Any("error", err))
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("The %s has been reset", heist.config.Theme.Heist),
	})
}

// resetVaults sets the vaults within the guild to their maximum value.
func resetVaults(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	resetVaultsToMaximumValue(guildID(e))
	return e.CreateMessage(discord.MessageCreate{
		Content: "Vaults have been reset to their maximum value",
	})
}

// listTargets displays a list of available heist targets.
func listTargets(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	member := e.Member()
	if member == nil {
		return serverOnly(e)
	}

	theme := GetTheme(discordid.NewSnowflakeID(member.GuildID))
	if theme == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "There aren't any targets!",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	targets := GetTargets(discordid.NewSnowflakeID(member.GuildID))
	if len(targets) == 0 {
		return e.CreateMessage(discord.MessageCreate{
			Content: "There aren't any targets!",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

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
		err := table.Close()
		if err != nil {

		}
	}(table)

	table.Header([]string{"ID", "Max Crew", theme.Vault, "Max " + theme.Vault, "Success Rate"})
	for _, target := range targets {
		data := []string{
			target.Name,
			fmt.Sprintf("%d", target.CrewSize),
			fmt.Sprintf("%d", target.Vault),
			fmt.Sprintf("%d", target.VaultMax),
			fmt.Sprintf("%.2f", target.Success),
		}
		if err := table.Append(data); err != nil {
			slog.Error("failed to append the data to the table", slog.Any("error", err))
		}
	}

	if err := table.Render(); err != nil {
		return err
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: "```\n" + tableBuffer.String() + "\n```",
		Flags:   discord.MessageFlagEphemeral,
	})
}

// clearMember clears the criminal state of the player.
func clearMember(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	user, ok := data.OptUser("user")
	if !ok {
		return e.CreateMessage(discord.MessageCreate{
			Content: "The user you specified does not exist.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	gID := guildID(e)
	member, err := guild.GetMemberByID(gID, discordid.NewSnowflakeID(user.ID))
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "The user you specified does not exist.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	heistMember := GetMember(gID, discordid.NewSnowflakeID(user.ID))
	heistMember.FreeMember()

	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Cleared %s's criminal record", member.Name),
	})
}

// configCost sets the cost to plan or join a heist.
func configCost(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	config := GetConfig(guildID(e))
	cost := data.Int("amount")
	config.HeistCost = cost

	writeConfig(config)

	p := message.NewPrinter(language.AmericanEnglish)
	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Cost set to %d", cost),
	})
}

// configSentence sets the base apprehension time when a player is apprehended.
func configSentence(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	config := GetConfig(guildID(e))
	sentence := data.Int("time")

	if sentence == 0 {
		config.SentenceBase = 0
		writeConfig(config)
		return e.CreateMessage(discord.MessageCreate{
			Content: "Sentence disabled",
		})
	}

	config.SentenceBase = time.Duration(sentence) * time.Second
	writeConfig(config)

	p := message.NewPrinter(language.AmericanEnglish)
	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Sentence set to %d seconds", sentence),
	})
}

// configPatrol sets the time authorities will prevent a new heist following one being completed.
func configPatrol(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	config := GetConfig(guildID(e))
	patrol := data.Int("time")
	config.PoliceAlert = time.Duration(patrol) * time.Second

	writeConfig(config)

	p := message.NewPrinter(language.AmericanEnglish)
	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Patrol set to %d", patrol),
	})
}

// configBail sets the base cost of bail.
func configBail(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	config := GetConfig(guildID(e))
	bail := data.Int("amount")
	config.BailBase = bail

	writeConfig(config)

	p := message.NewPrinter(language.AmericanEnglish)
	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Bail set to %d", bail),
	})
}

// configBoost sets the percentage to increase payouts when boosts are enabled.
func configBoost(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	config := GetConfig(guildID(e))
	boost := data.Float("percentage")
	if boost < 0 {
		boost = 0
	}
	config.BoostPercentage = boost

	writeConfig(config)

	p := message.NewPrinter(language.AmericanEnglish)
	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Boost percentage set to %.2f%%", boost),
	})
}

// configDeath sets how long players remain dead.
func configDeath(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	config := GetConfig(guildID(e))
	death := data.Int("time")
	config.DeathTimer = time.Duration(death) * time.Second

	writeConfig(config)

	p := message.NewPrinter(language.AmericanEnglish)
	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Death set to %d", death),
	})
}

// configBoostVaultRecovery sets the percentage of the vault that is recovered every minute when boosts are enabled.
func configBoostVaultRecovery(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	config := GetConfig(guildID(e))
	boostVaultRecovery := data.Int("percent")
	config.BoostVaultRecovery = float64(boostVaultRecovery) / 100.0

	writeConfig(config)

	p := message.NewPrinter(language.AmericanEnglish)
	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Vault recovery boost set to %d", boostVaultRecovery),
	})
}

// configWait sets how long players wait for others to join the heist.
func configWait(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	config := GetConfig(guildID(e))
	wait := data.Int("time")
	config.WaitTime = time.Duration(wait) * time.Second

	writeConfig(config)

	p := message.NewPrinter(language.AmericanEnglish)
	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Wait set to %d", wait),
	})
}

// configInfo returns the configuration for the Heist bot on this server.
func configInfo(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	config := GetConfig(guildID(e))
	inline := true

	embeds := []discord.Embed{
		{
			Fields: []discord.EmbedField{
				{Name: "bail", Value: fmt.Sprintf("%d", config.BailBase), Inline: &inline},
				{Name: "boost", Value: fmt.Sprintf("%.2f%%", config.BoostPercentage), Inline: &inline},
				{Name: "boost enabled", Value: fmt.Sprintf("%t", config.BoostEnabled), Inline: &inline},
				{Name: "cost", Value: fmt.Sprintf("%d", config.HeistCost), Inline: &inline},
				{Name: "death", Value: fmt.Sprintf("%.f", config.DeathTimer.Seconds()), Inline: &inline},
				{Name: "patrol", Value: fmt.Sprintf("%.f", config.PoliceAlert.Seconds()), Inline: &inline},
				{Name: "sentence", Value: fmt.Sprintf("%.f", config.SentenceBase.Seconds()), Inline: &inline},
				{Name: "boost vault recovery", Value: fmt.Sprintf("%d%%", int(math.Round(config.BoostVaultRecovery*100))), Inline: &inline},
				{Name: "wait", Value: fmt.Sprintf("%.f", config.WaitTime.Seconds()), Inline: &inline},
			},
		},
	}

	return e.CreateMessage(discord.MessageCreate{
		Content: "Heist Configuration",
		Embeds:  embeds,
		Flags:   discord.MessageFlagEphemeral,
	})
}

func requireAdmin(e *handler.CommandEvent) error {
	member := e.Member()
	if member == nil {
		return serverOnly(e)
	}

	guildMember := resolvedGuildMember(member)
	if guildMember == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "You do not have permission to use this command.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	ok, err := guildMember.IsAdmin(e.Client(), guild.GetGuild(discordid.NewSnowflakeID(member.GuildID)))
	if err != nil {
		slog.Error("failed to check admin permissions", slog.Any("error", err))
	}
	if err != nil || !ok {
		return e.CreateMessage(discord.MessageCreate{
			Content: "You do not have permission to use this command.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return nil
}

func serverOnly(e *handler.CommandEvent) error {
	return e.CreateMessage(discord.MessageCreate{
		Content: "This command can only be used in a server.",
		Flags:   discord.MessageFlagEphemeral,
	})
}

func resolvedGuildMember(member *discord.ResolvedMember) *guild.Member {
	if member == nil {
		return nil
	}

	globalName := ""
	if member.User.GlobalName != nil {
		globalName = *member.User.GlobalName
	}

	nickname := ""
	if member.Nick != nil {
		nickname = *member.Nick
	}

	return &guild.Member{
		GuildID:    discordid.NewSnowflakeID(member.GuildID),
		MemberID:   discordid.NewSnowflakeID(member.User.ID),
		UserName:   member.User.Username,
		GlobalName: globalName,
		NickName:   nickname,
		Name:       member.EffectiveName(),
	}
}

func guildID(e *handler.CommandEvent) discordid.SnowflakeID {
	if member := e.Member(); member != nil {
		return discordid.NewSnowflakeID(member.GuildID)
	}
	if id := e.GuildID(); id != nil {
		return discordid.NewSnowflakeID(*id)
	}

	return 0
}

func resetVaultsToMaximumValue(guildID discordid.SnowflakeID) {
	targets := GetTargets(guildID)
	for _, target := range targets {
		target.Vault = target.VaultMax
		target.IsAtMax = true
		writeTarget(target)
	}
}

func criminalLevelString(level CriminalLevel) string {
	switch level {
	case Greenhorn:
		return "Greenhorn"
	case Renegade:
		return "Renegade"
	case Veteran:
		return "Veteran"
	case Commander:
		return "Commander"
	case WarChief:
		return "War Chief"
	case Legend:
		return "Legend"
	case Immortal:
		return "Immortal"
	default:
		return fmt.Sprintf("%d", level)
	}
}

func firstToUpper(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
