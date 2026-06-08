package blackjack

import (
	"errors"
	"fmt"
	"goblin2/disgobot"
	"goblin2/guild"
	"goblin2/internal/discordid"
	"goblin2/internal/format"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	bj "github.com/rbrabson/blackjack"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	// active and inactive hands
	active   = "🟢"
	inactive = "⚪"

	// tree structure
	intermediate = "├─"
	final        = "└─"

	// dash used for spacing
	indent = "\u2003"

	// color for active player
	activePlayerColor = 0x00ff00
)

var (
	memberCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "blackjack",
			Description: "Interacts with the blackjack table.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "play",
					Description: "Play the blackjack game.",
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

	adminCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "blackjack-admin",
			Description: "Configures the blackjack game.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommandGroup{
					Name:        "config",
					Description: "Configures the blackjack game.",
					Options: []discord.ApplicationCommandOptionSubCommand{
						{
							Name:        "info",
							Description: "Returns the configuration information for the server.",
						},
						{
							Name:        "bet",
							Description: "Sets the bet amount.",
							Options: []discord.ApplicationCommandOption{
								discord.ApplicationCommandOptionInt{
									Name:        "amount",
									Description: "The amount to set the bet to.",
									Required:    true,
								},
							},
						},
						{
							Name:        "payout",
							Description: "The base payout percentage when winning a game.",
							Options: []discord.ApplicationCommandOption{
								discord.ApplicationCommandOptionInt{
									Name:        "percent",
									Description: "The amount to set the payout percentage to.",
									Required:    true,
								},
							},
						},
						{
							Name:        "single-player",
							Description: "Controls whether the game is single-player only or allows multiple players to join.",
							Options: []discord.ApplicationCommandOption{
								discord.ApplicationCommandOptionBool{
									Name:        "enabled",
									Description: "Whether single-player mode is enabled.",
									Required:    true,
								},
							},
						},
					},
				},
			},
		},
	}
)

// playBlackjackHandler starts a blackjack game that other members may join.
func playBlackjackHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	guildID := discordid.NewSnowflakeID(member.GuildID)
	memberID := discordid.NewSnowflakeID(member.User.ID)

	guild.GetGuild(guildID).GetMember(&member.Member)

	game, err := StartGame(guildID, memberID)
	if err != nil {
		slog.Debug("failed to start blackjack game",
			slog.Any("guildID", guildID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: format.FirstToUpper(err.Error()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	game.interaction = e

	if err := e.CreateMessage(discord.MessageCreate{
		Content:    "Starting blackjack...",
		Embeds:     blackjackEmbeds(game, false),
		Components: blackjackJoinComponents(game),
	}); err != nil {
		game.EndRound()
		return err
	}

	slog.Info("blackjack game started",
		slog.Any("guildID", guildID),
		slog.Any("memberID", memberID),
	)

	go runBlackjack(game)

	return nil
}

// blackjackStatsHandler returns a player's blackjack stats.
func blackjackStatsHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	targetUserID := member.User.ID
	targetUserName := member.User.Username

	if user, ok := data.OptUser("user"); ok {
		targetUserID = user.ID
		targetUserName = user.Username
	}

	guildID := discordid.NewSnowflakeID(member.GuildID)
	stats := GetMember(guildID, discordid.NewSnowflakeID(targetUserID))

	totalGames := stats.Wins + stats.Losses + stats.Pushes
	winRate := 0.0
	if totalGames > 0 {
		winRate = float64(stats.Wins) / float64(totalGames) * 100
	}

	netCredits := stats.CreditsWon - stats.CreditsLost

	displayName := "Your"
	if targetUserID != member.User.ID {
		displayName = targetUserName + "'s"

		if guildMember, err := guild.GetMemberByID(guildID, discordid.NewSnowflakeID(targetUserID)); err == nil && guildMember != nil && guildMember.Name != "" {
			displayName = guildMember.Name + "'s"
		}
	}

	inlineFalse := false
	inlineTrue := true

	embed := discord.Embed{
		Type:  discord.EmbedTypeRich,
		Title: fmt.Sprintf("🃏 %s Blackjack Statistics", displayName),
		Color: 0x2f3136,
		Fields: []discord.EmbedField{
			{
				Name: "📊 Game Summary",
				Value: fmt.Sprintf("**Rounds Played:** %s\n**Hands Played:** %s\n**Win Rate:** %.1f%%",
					p.Sprintf("%d", stats.RoundsPlayed),
					p.Sprintf("%d", stats.HandsPlayed),
					winRate,
				),
				Inline: &inlineFalse,
			},
			{
				Name: "🎯 Hand Results",
				Value: fmt.Sprintf("**Wins:** %s\n**Losses:** %s\n**Pushes:** %s",
					p.Sprintf("%d", stats.Wins),
					p.Sprintf("%d", stats.Losses),
					p.Sprintf("%d", stats.Pushes),
				),
				Inline: &inlineTrue,
			},
			{
				Name: "🎴 Special Hands",
				Value: fmt.Sprintf("**Blackjacks:** %s\n**Splits:** %s\n**Surrenders:** %s",
					p.Sprintf("%d", stats.Blackjacks),
					p.Sprintf("%d", stats.Splits),
					p.Sprintf("%d", stats.Surrenders),
				),
				Inline: &inlineTrue,
			},
			{
				Name: "💰 Credits",
				Value: fmt.Sprintf("**Total Bet:** %s\n**Credits Won:** %s\n**Credits Lost:** %s\n**Net:** %s",
					p.Sprintf("%d", stats.CreditsBet),
					p.Sprintf("%d", stats.CreditsWon),
					p.Sprintf("%d", stats.CreditsLost),
					formatNetCredits(netCredits, p),
				),
				Inline: &inlineFalse,
			},
		},
	}

	if !stats.LastPlayed.IsZero() {
		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name:   "🕒 Last Played",
			Value:  fmt.Sprintf("<t:%d:R>", stats.LastPlayed.Unix()),
			Inline: &inlineFalse,
		})
	}

	if stats.RoundsPlayed == 0 {
		embed.Description = "*No blackjack games played yet. Join a game to start tracking statistics!*"
	} else {
		avgHandsPerRound := float64(stats.HandsPlayed) / float64(stats.RoundsPlayed)
		embed.Footer = &discord.EmbedFooter{
			Text: fmt.Sprintf("Average %.1f hands per round", avgHandsPerRound),
		}
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
		Flags:  discord.MessageFlagEphemeral,
	})
}

// configBetAmountHandler sets the bet amount for the blackjack game on this server.
func configBetAmountHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	guildID := discordid.NewSnowflakeID(member.GuildID)
	betAmount := data.Int("amount")

	config := GetConfig(guildID)
	config.BetAmount = betAmount
	writeConfig(config)

	p := message.NewPrinter(language.AmericanEnglish)
	slog.Info("blackjack bet amount updated", slog.Any("guildID", guildID), slog.Int("betAmount", betAmount))

	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Bet amount set to %d", betAmount),
	})
}

// configPayoutPercentHandler sets the payout percent for the blackjack game on this server.
func configPayoutPercentHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	guildID := discordid.NewSnowflakeID(member.GuildID)
	payoutPercent := data.Int("percent")

	config := GetConfig(guildID)
	config.PayoutPercent = payoutPercent
	writeConfig(config)

	p := message.NewPrinter(language.AmericanEnglish)
	slog.Info("blackjack payout percent updated", slog.Any("guildID", guildID), slog.Int("payoutPercent", payoutPercent))

	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Payout percent set to %d", payoutPercent),
	})
}

// configSinglePlayerHandler sets the single-player mode for the blackjack game on this server.
func configSinglePlayerHandler(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	guildID := discordid.NewSnowflakeID(member.GuildID)
	singlePlayer := data.Bool("enabled")

	config := GetConfig(guildID)
	config.SinglePlayerMode = singlePlayer
	writeConfig(config)

	p := message.NewPrinter(language.AmericanEnglish)
	slog.Info("blackjack single-player mode updated", slog.Any("guildID", guildID), slog.Bool("singlePlayerMode", singlePlayer))

	return e.CreateMessage(discord.MessageCreate{
		Content: p.Sprintf("Single-player mode set to %t", singlePlayer),
	})
}

// configInfoHandler returns the configuration for the blackjack game on this server.
func configInfoHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	config := GetConfig(discordid.NewSnowflakeID(member.GuildID))
	inline := true

	return e.CreateMessage(discord.MessageCreate{
		Content: "Blackjack Configuration",
		Embeds: []discord.Embed{
			{
				Fields: []discord.EmbedField{
					{Name: "bet amount", Value: fmt.Sprintf("%d", config.BetAmount), Inline: &inline},
					{Name: "payout percent", Value: fmt.Sprintf("%d", config.PayoutPercent), Inline: &inline},
					{Name: "single player", Value: fmt.Sprintf("%t", config.SinglePlayerMode), Inline: &inline},
				},
			},
		},
		Flags: discord.MessageFlagEphemeral,
	})
}

// blackjackJoinButtonHandler handles the join button.
func blackjackJoinButtonHandler(e *handler.ComponentEvent) error {
	if err := e.DeferCreateMessage(true); err != nil {
		slog.Error("failed to defer blackjack join component response", slog.Any("error", err))
	}

	member := e.Member()
	if member == nil {
		return updateComponentResponse(e, "This command can only be used in a server.")
	}

	guildID := discordid.NewSnowflakeID(member.GuildID)
	memberID := discordid.NewSnowflakeID(member.User.ID)
	guild.GetGuild(guildID).GetMember(&member.Member)

	game := GetGame(guildID, getUIDFromComponent(e))
	if game == nil {
		return updateComponentResponse(e, "No blackjack game is being planned.")
	}

	if err := game.joinGame(memberID); err != nil {
		return updateComponentResponse(e, format.FirstToUpper(err.Error()))
	}

	if err := updateBlackjackMessage(game, false); err != nil {
		slog.Error("failed to update blackjack message after join",
			slog.Any("guildID", guildID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
	}

	slog.Info("blackjack game joined",
		slog.Any("guildID", guildID),
		slog.Any("memberID", memberID),
	)

	return updateComponentResponse(e, "You joined the blackjack game.")
}

// blackjackHitButtonHandler handles the hit button.
func blackjackHitButtonHandler(e *handler.ComponentEvent) error {
	return blackjackAction(e, Hit)
}

// blackjackStandButtonHandler handles the stand button.
func blackjackStandButtonHandler(e *handler.ComponentEvent) error {
	return blackjackAction(e, Stand)
}

// blackjackDoubleDownButtonHandler handles the double down button.
func blackjackDoubleDownButtonHandler(e *handler.ComponentEvent) error {
	return blackjackAction(e, DoubleDown)
}

// blackjackSplitButtonHandler handles the split button.
func blackjackSplitButtonHandler(e *handler.ComponentEvent) error {
	return blackjackAction(e, Split)
}

// blackjackSurrenderButtonHandler handles the surrender button.
func blackjackSurrenderButtonHandler(e *handler.ComponentEvent) error {
	return blackjackAction(e, Surrender)
}

// blackjackAction handles a player action button.
func blackjackAction(e *handler.ComponentEvent, action Action) error {
	member := e.Member()
	if member == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "This command can only be used in a server.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	guildID := discordid.NewSnowflakeID(member.GuildID)
	memberID := discordid.NewSnowflakeID(member.User.ID)

	game := GetGame(guildID, getUIDFromComponent(e))
	if game == nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "No blackjack game is in progress.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	if err := game.PlayerActionRequest(memberID, action); err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: format.FirstToUpper(err.Error()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return e.DeferUpdateMessage()
}

// runBlackjack runs the blackjack lifecycle after the slash command interaction has been acknowledged.
func runBlackjack(game *Game) {
	defer game.EndRound()

	if game.config.WaitForPlayers > 0 {
		waitForBlackjackPlayers(game)
	}

	if len(game.Players()) == 0 {
		if err := updateBlackjackMessage(game, false); err != nil {
			slog.Error("failed to update empty blackjack game", slog.Any("error", err))
		}
		return
	}

	if err := game.StartNewRound(); err != nil {
		slog.Error("failed to start blackjack round",
			slog.Any("guildID", game.guildID),
			slog.Any("error", err),
		)
		return
	}

	if err := game.DealInitialCards(); err != nil {
		slog.Error("failed to deal blackjack hands",
			slog.Any("guildID", game.guildID),
			slog.Any("error", err),
		)
		return
	}

	if err := updateBlackjackMessage(game, true); err != nil {
		slog.Error("failed to update blackjack message after deal", slog.Any("error", err))
	}

	playBlackjackPlayers(game)
	playBlackjackDealer(game)

	game.PayoutResults()
	game.SetState(Completed)

	if err := updateBlackjackMessage(game, false); err != nil {
		slog.Error("failed to update final blackjack message", slog.Any("error", err))
	}

	slog.Info("blackjack game ended",
		slog.Any("guildID", game.guildID),
	)
}

// waitForBlackjackPlayers waits for players to join before starting the round.
func waitForBlackjackPlayers(game *Game) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	count := 0
	for range ticker.C {
		count++
		if count%5 == 0 {
			if err := updateBlackjackMessage(game, false); err != nil {
				slog.Error("failed to update blackjack wait message", slog.Any("error", err))
			}
		}

		if time.Until(game.gameStartTime) <= 0 {
			break
		}
		if len(game.Players()) >= game.config.MaxPlayers {
			break
		}
	}
}

// playBlackjackPlayers processes player turns.
func playBlackjackPlayers(game *Game) {
	for _, player := range game.Players() {
		if !player.IsActive() {
			continue
		}

		for player.HasActiveHands() && player.IsActive() {
			hand := player.CurrentHand()
			if hand == nil {
				break
			}

			hand.SetActive(true)

			for hand.IsActive() {
				if err := updateBlackjackMessage(game, true); err != nil {
					slog.Error("failed to update blackjack active player message", slog.Any("error", err))
				}

				select {
				case action := <-game.turnChan:
					if err := applyBlackjackAction(game, player, action); err != nil {
						slog.Warn("failed to apply blackjack action",
							slog.Any("guildID", game.guildID),
							slog.String("player", player.Name()),
							slog.Any("action", action),
							slog.Any("error", err),
						)
					}

					if err := updateBlackjackMessage(game, true); err != nil {
						slog.Error("failed to update blackjack message after player action", slog.Any("error", err))
					}

				case <-time.After(game.config.PlayerTimeout):
					if err := game.PlayerStand(player); err != nil {
						slog.Warn("failed to auto-stand blackjack player",
							slog.Any("guildID", game.guildID),
							slog.String("player", player.Name()),
							slog.Any("error", err),
						)
					}

					if err := updateBlackjackMessage(game, true); err != nil {
						slog.Error("failed to update blackjack message after player timeout", slog.Any("error", err))
					}
				}
			}

			hand.SetActive(false)
			if !player.MoveToNextActiveHand() {
				player.SetActive(false)
			}
		}
	}
}

// applyBlackjackAction applies the selected blackjack action.
func applyBlackjackAction(game *Game, player *bj.Player, action Action) error {
	switch action {
	case Hit:
		return game.PlayerHit(player)
	case Stand:
		return game.PlayerStand(player)
	case DoubleDown:
		return game.PlayerDoubleDown(player)
	case Split:
		return game.PlayerSplit(player)
	case Surrender:
		return game.PlayerSurrender(player)
	default:
		return fmt.Errorf("unknown blackjack action: %d", action)
	}
}

// playBlackjackDealer processes the dealer turn.
func playBlackjackDealer(game *Game) {
	if game.config.ShowDealerTurn > 0 {
		time.Sleep(game.config.ShowDealerTurn)
	}

	if err := game.DealerPlay(); err != nil && !errors.Is(err, ErrAllPlayersBusted) {
		slog.Error("failed during blackjack dealer play",
			slog.Any("guildID", game.guildID),
			slog.Any("error", err),
		)
	}
}

// updateBlackjackMessage updates the original blackjack game message.
func updateBlackjackMessage(game *Game, hideDealerCard bool) error {
	if game.interaction == nil {
		return nil
	}

	embeds := blackjackEmbeds(game, hideDealerCard)
	components := blackjackComponents(game)
	content := ""

	_, err := game.interaction.Client().Rest.UpdateInteractionResponse(
		game.interaction.ApplicationID(),
		game.interaction.Token(),
		discord.MessageUpdate{
			Content:    &content,
			Embeds:     &embeds,
			Components: &components,
		},
	)
	return err
}

// blackjackEmbeds returns the blackjack game embeds.
func blackjackEmbeds(game *Game, hideDealerCard bool) []discord.Embed {
	inline := false
	fields := []discord.EmbedField{
		{
			Name:   blackjackStatus(game),
			Value:  "\u200b",
			Inline: &inline,
		},
	}

	if game.Dealer() != nil && len(game.Dealer().Hand().Cards()) > 0 {
		fields = append(fields, discord.EmbedField{
			Name:   "Dealer",
			Value:  game.symbols.GetHand(game.Dealer().Hand(), hideDealerCard),
			Inline: &inline,
		})
	}

	embeds := []discord.Embed{
		{
			Type:   discord.EmbedTypeRich,
			Title:  symbols.Cards.Multiple + " Blackjack " + symbols.Cards.Multiple,
			Fields: fields,
		},
	}

	for _, player := range game.Players() {
		embeds = append(embeds, discord.Embed{
			Type:        discord.EmbedTypeRich,
			Title:       blackjackPlayerTitle(game, player),
			Description: blackjackPlayerHands(game, player),
			Color:       blackjackPlayerEmbedColor(game, player),
		})
	}

	return embeds
}

// blackjackPlayerEmbedColor returns the embed color for a player.
func blackjackPlayerEmbedColor(game *Game, player *bj.Player) int {
	if !game.IsDealingHands() {
		return 0
	}
	if game.GetActivePlayer() != player {
		return 0
	}
	if !player.HasActiveHands() {
		return 0
	}

	return activePlayerColor
}

// blackjackStatus returns a short status line for the game.
func blackjackStatus(game *Game) string {
	switch {
	case game.IsWaitingForPlayers():
		return fmt.Sprintf("A blackjack game is starting! Click Join Game to play. Starts in %s.", format.Duration(time.Until(game.gameStartTime)))
	case game.IsStartingRound():
		return "Starting the blackjack round..."
	case game.IsDealingHands():
		activePlayer := game.GetActivePlayer()
		if activePlayer != nil {
			return fmt.Sprintf("It is %s's turn.", blackjackPlayerName(game, activePlayer))
		}
		return "Blackjack is in progress."
	case game.IsCompleted():
		return "Blackjack has ended."
	default:
		return "Blackjack has ended."
	}
}

// blackjackPlayerTitle returns the title for a player embed.
func blackjackPlayerTitle(game *Game, player *bj.Player) string {
	name := blackjackPlayerName(game, player)
	if game.GetActivePlayer() == player {
		return fmt.Sprintf("%s %s", active, name)
	}
	return fmt.Sprintf("%s %s", inactive, name)
}

// blackjackPlayerName returns a readable display name for a blackjack player.
func blackjackPlayerName(game *Game, player *bj.Player) string {
	if player == nil {
		return "Unknown Player"
	}

	memberID, err := discordid.SnowflakeIDFromString(player.Name())
	if err != nil {
		return player.Name()
	}

	member, err := guild.GetMemberByID(game.guildID, memberID)
	if err != nil || member == nil || member.Name == "" {
		return player.Name()
	}

	return member.Name
}

// blackjackPlayerHands returns the rendered hands for a player.
func blackjackPlayerHands(game *Game, player *bj.Player) string {
	hands := make([]string, 0, len(player.Hands()))
	activePlayer := game.GetActivePlayer()
	activeHandIndex := -1
	if activePlayer == player {
		activeHandIndex = player.GetCurrentHandNumber()
	}

	for idx, hand := range player.Hands() {
		treePrefix := intermediate
		if idx == len(player.Hands())-1 {
			treePrefix = final
		}

		statusPrefix := inactive
		if activePlayer == player && idx == activeHandIndex && hand.IsActive() {
			statusPrefix = active
		}

		handText := fmt.Sprintf("%s %s Hand %d: %s", treePrefix, statusPrefix, idx+1, game.symbols.GetHand(hand, false))
		if !game.IsWaitingForPlayers() && !game.IsStartingRound() && !game.IsDealingHands() {
			result := blackjackHandResult(game, hand)
			if result != "" {
				handText = fmt.Sprintf("%s\n%s%s%s %s", handText, indent, indent, indent, result)
			}
		}
		hands = append(hands, handText)
	}
	if len(hands) == 0 {
		return "\u200b"
	}
	return strings.Join(hands, "\n")
}

// blackjackHandResult returns a readable result for a completed hand.
func blackjackHandResult(game *Game, hand *bj.Hand) string {
	winnings := hand.Winnings()

	switch game.EvaluateHand(hand) {
	case bj.PlayerBlackjack:
		return blackjackCreditResult(winnings, game.config.PayoutPercent)
	case bj.PlayerWin:
		return blackjackCreditResult(winnings, game.config.PayoutPercent)
	case bj.DealerBlackjack:
		return blackjackCreditResult(winnings, game.config.PayoutPercent)
	case bj.DealerWin:
		return blackjackCreditResult(winnings, game.config.PayoutPercent)
	case bj.Push:
		return "Push"
	default:
		return "No result"
	}
}

// blackjackCreditResult formats the hand result with the amount won or lost.
func blackjackCreditResult(winnings int, payoutPercent int) string {
	switch {
	case winnings > 0:
		winnings = winnings * payoutPercent / 100
		if winnings == 1 {
			return "Won 1 credit"
		}
		return fmt.Sprintf("Won %d credits", winnings)
	case winnings < 0:
		loss := -winnings
		if loss == 1 {
			return "Lost 1 credit"
		}
		return fmt.Sprintf("Lost %d credits", loss)
	default:
		return ""
	}
}

// blackjackJoinComponents returns only the join button row.
func blackjackJoinComponents(game *Game) []discord.LayoutComponent {
	return []discord.LayoutComponent{
		discord.ActionRowComponent{
			Components: []discord.InteractiveComponent{
				game.joinButton,
			},
		},
	}
}

// blackjackComponents returns the component rows for the current game state.
func blackjackComponents(game *Game) []discord.LayoutComponent {
	if game.IsWaitingForPlayers() {
		return blackjackJoinComponents(game)
	}

	if !game.IsDealingHands() {
		return []discord.LayoutComponent{}
	}

	buttons := blackjackActionButtons(game)
	if len(buttons) == 0 {
		return []discord.LayoutComponent{}
	}

	return []discord.LayoutComponent{
		discord.ActionRowComponent{
			Components: buttons,
		},
	}
}

// blackjackActionButtons returns only the action buttons valid for the current active hand.
func blackjackActionButtons(game *Game) []discord.InteractiveComponent {
	activePlayer := game.GetActivePlayer()
	if activePlayer == nil {
		return nil
	}

	currentHand := activePlayer.CurrentHand()
	if currentHand == nil || !currentHand.IsActive() || currentHand.IsBusted() || currentHand.IsBlackjack() {
		return nil
	}

	buttons := []discord.InteractiveComponent{
		game.hitButton,
		game.standButton,
	}

	if currentHand.CanDoubleDown() {
		buttons = append(buttons, game.doubleDownButton)
	}
	if currentHand.CanSplit() {
		buttons = append(buttons, game.splitButton)
	}
	if currentHand.CanSurrender() {
		buttons = append(buttons, game.surrenderButton)
	}

	return buttons
}

// updateComponentResponse updates the interaction response with the given content.
func updateComponentResponse(e *handler.ComponentEvent, content string) error {
	_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
		Content: &content,
	})
	return err
}

// formatNetCredits formats the net credits with the appropriate color coding.
func formatNetCredits(netCredits int, p *message.Printer) string {
	switch {
	case netCredits > 0:
		return fmt.Sprintf("**+%s** 📈", p.Sprintf("%d", netCredits))
	case netCredits < 0:
		return fmt.Sprintf("**%s** 📉", p.Sprintf("%d", netCredits))
	default:
		return "**0** ➖"
	}
}
