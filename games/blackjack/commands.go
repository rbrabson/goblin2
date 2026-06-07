package blackjack

import (
	"errors"
	"fmt"
	"goblin2/disgobot"
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

var (
	memberCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "blackjack",
			Description: "Blackjack game commands.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "start",
					Description: "Starts a new blackjack game.",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "stats",
					Description: "Returns your blackjack stats.",
				},
			},
		},
	}
)

// startBlackjackHandler starts a blackjack game that other members may join.
func startBlackjackHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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
func blackjackStatsHandler(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
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

	stats := GetMember(guildID, memberID)
	p := message.NewPrinter(language.AmericanEnglish)

	inline := true
	embeds := []discord.Embed{
		{
			Type:  discord.EmbedTypeRich,
			Title: member.EffectiveName(),
			Fields: []discord.EmbedField{
				{Name: "Rounds Played", Value: p.Sprintf("%d", stats.RoundsPlayed), Inline: &inline},
				{Name: "Hands Played", Value: p.Sprintf("%d", stats.HandsPlayed), Inline: &inline},
				{Name: "Wins", Value: p.Sprintf("%d", stats.Wins), Inline: &inline},
				{Name: "Losses", Value: p.Sprintf("%d", stats.Losses), Inline: &inline},
				{Name: "Pushes", Value: p.Sprintf("%d", stats.Pushes), Inline: &inline},
				{Name: "Blackjacks", Value: p.Sprintf("%d", stats.Blackjacks), Inline: &inline},
				{Name: "Splits", Value: p.Sprintf("%d", stats.Splits), Inline: &inline},
				{Name: "Surrenders", Value: p.Sprintf("%d", stats.Surrenders), Inline: &inline},
				{Name: "Credits Bet", Value: p.Sprintf("%d", stats.CreditsBet), Inline: &inline},
				{Name: "Credits Won", Value: p.Sprintf("%d", stats.CreditsWon), Inline: &inline},
				{Name: "Credits Lost", Value: p.Sprintf("%d", stats.CreditsLost), Inline: &inline},
			},
		},
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds: embeds,
		Flags:  discord.MessageFlagEphemeral,
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
	if err := e.DeferCreateMessage(true); err != nil {
		slog.Error("failed to defer blackjack action component response", slog.Any("error", err))
	}

	member := e.Member()
	if member == nil {
		return updateComponentResponse(e, "This command can only be used in a server.")
	}

	guildID := discordid.NewSnowflakeID(member.GuildID)
	memberID := discordid.NewSnowflakeID(member.User.ID)

	game := GetGame(guildID, getUIDFromComponent(e))
	if game == nil {
		return updateComponentResponse(e, "No blackjack game is in progress.")
	}

	if err := game.PlayerActionRequest(memberID, action); err != nil {
		return updateComponentResponse(e, format.FirstToUpper(err.Error()))
	}

	return updateComponentResponse(e, fmt.Sprintf("You chose %d.", action))
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
		for _, hand := range player.Hands() {
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

				case <-time.After(game.config.PlayerTimeout):
					if err := game.PlayerStand(player); err != nil {
						slog.Warn("failed to auto-stand blackjack player",
							slog.Any("guildID", game.guildID),
							slog.String("player", player.Name()),
							slog.Any("error", err),
						)
					}
				}
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

	_, err := game.interaction.Client().Rest.UpdateInteractionResponse(
		game.interaction.ApplicationID(),
		game.interaction.Token(),
		discord.MessageUpdate{
			Content:    new(string),
			Embeds:     new(blackjackEmbeds(game, hideDealerCard)),
			Components: new(blackjackComponents(game)),
		},
	)
	return err
}

// blackjackEmbeds returns the blackjack game embeds.
func blackjackEmbeds(game *Game, hideDealerCard bool) []discord.Embed {
	inline := false
	fields := make([]discord.EmbedField, 0, len(game.Players())+2)

	fields = append(fields, discord.EmbedField{
		Name:   blackjackStatus(game),
		Value:  "\u200b",
		Inline: &inline,
	})

	if game.Dealer() != nil && len(game.Dealer().Hand().Cards()) > 0 {
		fields = append(fields, discord.EmbedField{
			Name:   "Dealer",
			Value:  game.symbols.GetHand(game.Dealer().Hand(), hideDealerCard),
			Inline: &inline,
		})
	}

	for _, player := range game.Players() {
		fields = append(fields, discord.EmbedField{
			Name:   blackjackPlayerTitle(game, player),
			Value:  blackjackPlayerHands(game, player),
			Inline: &inline,
		})
	}

	return []discord.Embed{
		{
			Type:   discord.EmbedTypeRich,
			Title:  "Blackjack",
			Fields: fields,
		},
	}
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
			return fmt.Sprintf("It is <@%s>'s turn.", activePlayer.Name())
		}
		return "Blackjack is in progress."
	default:
		return "Blackjack has ended."
	}
}

// blackjackPlayerTitle returns the title for a player field.
func blackjackPlayerTitle(game *Game, player *bj.Player) string {
	activePlayer := game.GetActivePlayer()
	if activePlayer != nil && activePlayer == player {
		return fmt.Sprintf("▶ <@%s>", player.Name())
	}
	return fmt.Sprintf("<@%s>", player.Name())
}

// blackjackPlayerHands returns the rendered hands for a player.
func blackjackPlayerHands(game *Game, player *bj.Player) string {
	hands := make([]string, 0, len(player.Hands()))
	for idx, hand := range player.Hands() {
		hands = append(hands, fmt.Sprintf("Hand %d: %s", idx+1, game.symbols.GetHand(hand, false)))
	}
	if len(hands) == 0 {
		return "\u200b"
	}
	return strings.Join(hands, "\n")
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

	if game.IsDealingHands() {
		return []discord.LayoutComponent{
			discord.ActionRowComponent{
				Components: []discord.InteractiveComponent{
					game.hitButton,
					game.standButton,
					game.doubleDownButton,
					game.splitButton,
					game.surrenderButton,
				},
			},
		}
	}

	return []discord.LayoutComponent{}
}

// updateComponentResponse updates the interaction response with the given content.
func updateComponentResponse(e *handler.ComponentEvent, content string) error {
	_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
		Content: &content,
	})
	return err
}
