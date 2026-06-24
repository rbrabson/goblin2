package blackjack

import (
	"errors"
	"fmt"
	"goblin2/internal/discordid"
	"goblin2/internal/format"
	"goblin2/plugin"
	"goblin2/stats"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
	bj "github.com/rbrabson/blackjack"
	"github.com/rbrabson/cards"
)

var (
	gamesLock sync.Mutex
	games     = make(map[string]*Game)
	// gameEndTimes records when the most recent game for each uid ended, so a new game for
	// the same uid is held off until config.DelayBetweenGames has elapsed. Access is guarded
	// by gamesLock.
	gameEndTimes = make(map[string]time.Time)
)

type GameState int

const (
	_ GameState = iota
	NotStarted
	WaitingForPlayers
	StartingRound
	DealingHands
	Completed
)

type Action int

const (
	_ Action = iota
	Hit
	Stand
	DoubleDown
	Split
	Surrender
)

// Game represents a blackjack game for a specific guild.
type Game struct {
	guildID          discordid.SnowflakeID
	game             *bj.Game
	config           *Config
	state            GameState
	gameStartTime    time.Time
	turnChan         chan Action
	turnDeadline     time.Time
	interaction      *handler.CommandEvent
	messageID        snowflake.ID
	symbols          *Symbols
	joinButton       discord.ButtonComponent
	hitButton        discord.ButtonComponent
	standButton      discord.ButtonComponent
	doubleDownButton discord.ButtonComponent
	splitButton      discord.ButtonComponent
	surrenderButton  discord.ButtonComponent
	uid              string
	lock             sync.Mutex
}

// GetGame retrieves the blackjack game for the specified guild.
// If no game exists, a new one is created.
func GetGame(guildID discordid.SnowflakeID, uid string) *Game {
	gamesLock.Lock()
	defer gamesLock.Unlock()

	config := GetConfig(guildID)
	game := games[uid]
	if game == nil {
		slog.Warn("blackjack game not found", slog.Any("guildID", guildID), slog.String("uid", uid))
		return nil
	}
	game.config = config
	return game
}

// activeGameCount returns the number of active blackjack games.
func activeGameCount() int {
	gamesLock.Lock()
	defer gamesLock.Unlock()

	count := 0
	for _, game := range games {
		game.Lock()
		if !game.NotStarted() {
			count++
		}
		game.Unlock()
	}

	return count
}

// StartGame starts a new blackjack game for the specified guild and member.
func StartGame(guildID discordid.SnowflakeID, memberID discordid.SnowflakeID) (*Game, error) {
	if currentPlugin != nil && currentPlugin.Status() != plugin.Running {
		return nil, ErrGameActive
	}

	gamesLock.Lock()
	defer gamesLock.Unlock()

	uid := getUID(guildID, memberID)
	config := GetConfig(guildID)

	if config.SinglePlayerMode {
		slog.Error("single player mode is enabled", slog.Any("guildID", guildID))
		if remaining := gameCooldownRemaining(uid, config.DelayBetweenGames); remaining > 0 {
			return nil, fmt.Errorf("please wait %s before starting another blackjack game", format.Duration(remaining))
		}
	} else {
		slog.Error("single player mode is disabled", slog.Any("guildID", guildID))
	}

	game := games[uid]
	if game == nil {
		game = newGame(guildID, uid, config.Decks)
		games[uid] = game
		slog.Info("created new blackjack game",
			slog.Any("guildID", guildID),
			slog.String("uid", uid),
		)
	} else {
		slog.Info("retrieved existing blackjack game",
			slog.Any("guildID", guildID),
			slog.String("uid", uid),
		)
	}

	game.Lock()
	defer game.Unlock()

	if !game.NotStarted() {
		slog.Debug("blackjack game has already started", slog.Any("guildID", game.guildID), slog.Any("memberID", memberID))
		return nil, ErrGameActive
	}

	game.config = config

	game.SetState(WaitingForPlayers)
	if err := game.addPlayer(memberID); err != nil {
		game.SetState(NotStarted)
		return nil, err
	}

	return game, nil
}

// gameCooldownRemaining returns how long until a new game for the given uid may start, based
// on when the previous game ended and the configured delay between games. It returns 0 if no
// delay applies. It must be called while holding gamesLock; expired entries are pruned.
func gameCooldownRemaining(uid string, delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}

	endedAt, ok := gameEndTimes[uid]
	if !ok {
		return 0
	}

	if remaining := delay - time.Since(endedAt); remaining > 0 {
		return remaining
	}

	delete(gameEndTimes, uid)
	return 0
}

// newGame creates a new blackjack game for the specified guild.
func newGame(guildID discordid.SnowflakeID, uid string, numDecks int) *Game {
	game := &Game{
		guildID:  guildID,
		uid:      uid,
		game:     bj.New(numDecks),
		state:    NotStarted,
		turnChan: make(chan Action, 5),
		symbols:  GetSymbols(),
		lock:     sync.Mutex{},
	}
	createButtons(game)

	return game
}

// joinGame allows a player to join the blackjack game if it has not started yet.
func (g *Game) joinGame(memberID discordid.SnowflakeID) error {
	g.Lock()
	defer g.Unlock()

	return g.addPlayer(memberID)
}

// addPlayer adds a player to the blackjack game with a chip manager that uses their bank account.
// If the player already exists, no action is taken.
func (g *Game) addPlayer(memberID discordid.SnowflakeID) error {
	playerID := memberID.String()

	if g.GetPlayer(memberID) != nil {
		return ErrPlayerAlreadyInGame
	}
	if g.NotStarted() {
		return ErrGameNotStarted
	}
	if !g.IsWaitingForPlayers() {
		return ErrGameActive
	}
	if len(g.game.Players()) >= g.config.MaxPlayers {
		return ErrGameFull
	}

	cm := NewChipManager(g, memberID)
	g.game.AddPlayer(playerID, bj.WithChipManager(cm))
	player := g.GetPlayer(memberID)
	if err := player.CurrentHand().PlaceBet(g.config.BetAmount); err != nil {
		g.game.RemovePlayer(playerID)
		return err
	}

	// If this is the first player, set the game start time to wait for additional players.
	if len(g.game.Players()) == 1 {
		g.gameStartTime = time.Now().Add(g.config.WaitForPlayers)
	}

	slog.Info("player joined blackjack game", slog.Any("guildID", g.guildID), slog.String("playerName", player.Name()), slog.Int("currentPlayers", len(g.game.Players())))

	return nil
}

// clearPendingActions clears any pending player actions from the turn channel, ensuring that
// no stale actions are processed when a new round starts or when a player takes an action.
func (g *Game) clearPendingActions() {
	for {
		select {
		case <-g.turnChan:
		default:
			return
		}
	}
}

// SetState sets the current state of the blackjack game.
func (g *Game) SetState(state GameState) {
	slog.Debug("setting blackjack game state", slog.Any("guildID", g.guildID), slog.Any("state", state))
	g.state = state
}

// GetPlayer retrieves a player from the blackjack game by their member ID.
func (g *Game) GetPlayer(memberID discordid.SnowflakeID) *bj.Player {
	return g.game.GetPlayer(memberID.String())
}

// GetActivePlayer retrieves the currently active player in the blackjack game.
func (g *Game) GetActivePlayer() *bj.Player {
	return g.game.GetActivePlayer()
}

// Players returns a slice of all players in the blackjack game.
func (g *Game) Players() []*bj.Player {
	return g.game.Players()
}

// StartNewRound starts a new round of blackjack in the game.
func (g *Game) StartNewRound() error {
	g.Lock()
	defer g.Unlock()

	if err := g.game.StartNewRound(); err != nil {
		return err
	}
	g.SetState(StartingRound)

	return nil
}

// EndRound ends the current round of blackjack for the guild, removing all players from the game.
func (g *Game) EndRound() {
	// All state mutation happens while holding gamesLock and the game lock. stopIfIdle is
	// deliberately called afterwards (outside both locks): it invokes activeGameCount, which
	// re-acquires gamesLock and every game's lock. Calling it while EndRound still held those
	// locks self-deadlocks (Go mutexes are not reentrant) and wedges gamesLock for the whole
	// process. This mirrors how heist and race release their map lock before stopIfIdle.
	g.endRoundLocked()

	if currentPlugin != nil {
		currentPlugin.stopIfIdle()
	}
}

// endRoundLocked performs the round teardown that must run under gamesLock and the game lock.
func (g *Game) endRoundLocked() {
	gamesLock.Lock()
	defer gamesLock.Unlock()

	g.Lock()
	defer g.Unlock()

	// Update the member stats
	for _, player := range g.Players() {
		slog.Debug("updating member stats for player",
			slog.Any("guildID", g.guildID),
			slog.String("playerName", player.Name()),
		)
		memberID, err := discordid.SnowflakeIDFromString(player.Name())
		if err != nil {
			slog.Warn("unable to parse blackjack player id for stats",
				slog.Any("guildID", g.guildID),
				slog.String("playerName", player.Name()),
				slog.Any("error", err),
			)
			continue
		}
		member := GetMember(g.guildID, memberID)
		member.RoundPlayed(g, player)
	}

	memberIDs := make([]discordid.SnowflakeID, 0, len(g.Players()))
	for _, player := range g.Players() {
		memberID, err := discordid.SnowflakeIDFromString(player.Name())
		if err != nil {
			slog.Warn("unable to parse blackjack player id for stats",
				slog.Any("guildID", g.guildID),
				slog.String("playerName", player.Name()),
				slog.Any("error", err),
			)
			continue
		}
		memberIDs = append(memberIDs, memberID)
	}
	stats.UpdateGameStats(g.guildID, "blackjack", memberIDs)

	for _, player := range g.game.Players() {
		slog.Debug("removing player from blackjack game",
			slog.Any("guildID", g.guildID),
			slog.String("playerName", player.Name()),
		)
		g.game.RemovePlayer(player.Name())
	}

	g.clearPendingActions()
	slog.Debug("cleared pending blackjack player actions for new round",
		slog.Any("guildID", g.guildID),
	)

	g.interaction = nil
	g.messageID = 0
	g.SetState(NotStarted)

	if g.config.SinglePlayerMode {
		slog.Info("deleting single blackjack player game", slog.Any("guildID", g.guildID), slog.String("uid", g.uid))
		delete(games, g.uid)
	} else {
		slog.Info("clearing multiplayer blackjack game state for new round", slog.Any("guildID", g.guildID))
		g.Dealer().ClearHand()
	}

	// Record when this game ended so the configured delay between games is enforced before a
	// new game for the same uid can be started.
	gameEndTimes[g.uid] = time.Now()
}

// NotStarted returns whether the blackjack game has not yet started.
func (g *Game) NotStarted() bool {
	return g.state == NotStarted
}

// IsWaitingForPlayers returns whether the blackjack game is waiting for players to join.
func (g *Game) IsWaitingForPlayers() bool {
	return g.state == WaitingForPlayers
}

// IsStartingRound returns whether the blackjack game is in the process of starting a new round,
// which occurs after the initial hands have been dealt and before player turns begin.
func (g *Game) IsStartingRound() bool {
	return g.state == StartingRound
}

// IsDealingHands returns whether the blackjack game is currently dealing initial hands to players.
func (g *Game) IsDealingHands() bool {
	return g.state == DealingHands
}

// IsCompleted returns whether the blackjack game has completed.
func (g *Game) IsCompleted() bool {
	return g.state == Completed
}

// SecondsBeforeStart returns the number of seconds remaining to wait for players
// before starting the game. If the wait time has elapsed, it returns 0.
func (g *Game) SecondsBeforeStart() int {
	waitTime := time.Until(g.gameStartTime)
	if waitTime > 0 {
		return int(waitTime.Seconds())
	}
	return 0
}

// DealInitialCards deals the initial cards to all players and the dealer.
func (g *Game) DealInitialCards() error {
	g.Lock()
	defer g.Unlock()

	g.SetState(DealingHands)

	if err := g.game.DealInitialCards(); err != nil {
		return err
	}
	for _, player := range g.game.Players() {
		for _, hand := range player.Hands() {
			hand.SetBet(g.config.BetAmount)
		}
	}
	return nil
}

// Dealer returns the dealer of the blackjack game.
func (g *Game) Dealer() *bj.Dealer {
	return g.game.Dealer()
}

// PlayerHit processes a hit action for the specified player.
func (g *Game) PlayerHit(player *bj.Player) error {
	g.Lock()
	defer g.Unlock()

	if err := g.game.PlayerHit(player.Name()); err != nil {
		return err
	}

	hand := player.CurrentHand()
	if hand.IsBusted() {
		slog.Debug("player busted",
			slog.Any("guildID", g.guildID),
			slog.String("playerName",
				player.Name()),
			slog.Any("hand", hand),
		)
		hand.SetActive(false)
	} else if hand.Value() == 21 {
		slog.Debug("player hand reached 21",
			slog.Any("guildID", g.guildID),
			slog.String("playerName", player.Name()),
			slog.Any("hand", hand),
		)
		hand.SetActive(false)
	}

	return nil
}

// PlayerStand processes a stand action for the specified player.
func (g *Game) PlayerStand(player *bj.Player) error {
	g.Lock()
	defer g.Unlock()

	if err := g.game.PlayerStand(player.Name()); err != nil {
		return err
	}

	hand := player.CurrentHand()
	hand.SetActive(false)

	return nil

}

// PlayerDoubleDown processes a double down hit action for the specified player.
func (g *Game) PlayerDoubleDown(player *bj.Player) error {
	g.Lock()
	defer g.Unlock()

	if !player.CurrentHand().CanDoubleDown() {
		slog.Error("cannot double down",
			slog.Any("guildID", g.guildID),
			slog.String("playerName", player.Name()),
		)
		return ErrCannotDoubleDown
	}
	if err := player.CurrentHand().DoubleDown(); err != nil {
		slog.Error("error processing player double down",
			slog.Any("guildID", g.guildID),
			slog.String("playerName", player.Name()),
			slog.Any("error", err),
		)
		return err
	}
	if err := g.game.PlayerDoubleDownHit(player.Name()); err != nil {
		slog.Error("error processing player double down hit",
			slog.Any("guildID", g.guildID),
			slog.String("playerName", player.Name()),
			slog.Any("error", err),
		)
		return err
	}

	hand := player.CurrentHand()
	if hand.IsBusted() {
		slog.Debug("player busted after double down",
			slog.Any("guildID", g.guildID),
			slog.String("playerName",
				player.Name()),
			slog.Any("hand", hand),
		)
	} else if hand.Value() == 21 {
		slog.Debug("player hand reached 21 after double down",
			slog.Any("guildID", g.guildID),
			slog.String("playerName",
				player.Name()),
			slog.Any("hand", hand),
		)
	}
	hand.SetActive(false)

	return nil
}

// PlayerSplit processes a split action for the specified player.
func (g *Game) PlayerSplit(player *bj.Player) error {
	g.Lock()
	defer g.Unlock()

	if !player.CurrentHand().CanSplit() {
		slog.Error("cannot split",
			slog.Any("guildID", g.guildID),
			slog.String("playerName", player.Name()),
		)
		return ErrCannotSplit
	}

	if err := g.game.PlayerSplit(player.Name()); err != nil {
		slog.Error("error processing player split",
			slog.Any("guildID", g.guildID),
			slog.String("playerName", player.Name()),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// PlayerSurrender processes a surrender action for the specified player.
func (g *Game) PlayerSurrender(player *bj.Player) error {
	g.Lock()
	defer g.Unlock()

	if !player.CurrentHand().CanSurrender() {
		slog.Error("cannot surrender",
			slog.Any("guildID", g.guildID),
			slog.String("playerName", player.Name()),
		)
		return ErrCannotSurrender
	}

	if err := g.game.PlayerSurrender(player.Name()); err != nil {
		return err
	}

	hand := player.CurrentHand()
	hand.SetActive(false)

	return nil
}

// PlayerActionRequest processes a request from a player to take an action, ensuring that the player is active and the game is in progress.
func (g *Game) PlayerActionRequest(memberID discordid.SnowflakeID, action Action) error {
	g.Lock()

	if !g.IsDealingHands() {
		g.Unlock()
		return ErrGameNotStarted
	}

	player := g.GetPlayer(memberID)
	activePlayer := g.GetActivePlayer()
	if player == nil || player != activePlayer {
		g.Unlock()
		return ErrNotActivePlayer
	}

	g.Unlock()

	select {
	case g.turnChan <- action:
		return nil
	default:
		return errors.New("too many pending blackjack actions")
	}
}

// DealerPlay processes the dealer's play according to blackjack rules.
func (g *Game) DealerPlay() error {
	g.Lock()
	defer g.Unlock()

	if !g.hasNonbustedPlayers() {
		return ErrAllPlayersBusted
	}
	if err := g.game.DealerPlay(); err != nil {
		return err
	}

	return nil
}

// hasNonbustedPlayers checks if there are any players in the game who have not busted, surrendered, or gotten blackjack.
func (g *Game) hasNonbustedPlayers() bool {
	for _, player := range g.Players() {
		for _, hand := range player.Hands() {
			if !(hand.IsBusted() || hand.IsSurrendered() || hand.IsBlackjack()) {
				return true
			}
		}
	}
	return false
}

// PayoutResults pays out the results of the blackjack game.
func (g *Game) PayoutResults() {
	g.Lock()
	defer g.Unlock()

	for _, player := range g.game.Players() {
		for _, hand := range player.Hands() {
			// Skip hands with no bet or already settled.
			if hand.Bet() == 0 || hand.Winnings() != 0 {
				continue
			}

			switch g.game.EvaluateHand(hand) {
			case bj.PlayerWin, bj.PlayerBlackjack:
				payout := 1.0
				if hand.IsBlackjack() {
					payout = 1.5
				}
				hand.WinBet(payout)
			case bj.Push:
				hand.PushBet()
			case bj.DealerWin, bj.DealerBlackjack:
				hand.LoseBet()
			}
		}
	}
}

// EvaluateHand evaluates the result of a specific hand for a player.
func (g *Game) EvaluateHand(hand *bj.Hand) bj.GameResult {
	return g.game.EvaluateHand(hand)
}

// Round returns the current round number of the blackjack game.
func (g *Game) Round() int {
	return g.game.Round()
}

// Lock locks the game's mutex.
func (g *Game) Lock() {
	g.lock.Lock()
}

// Unlock unlocks the game's mutex.
func (g *Game) Unlock() {
	g.lock.Unlock()
}

// setTurnDeadline records when the active player's turn will time out, so the rendered
// message can show a countdown beneath their name.
func (g *Game) setTurnDeadline(deadline time.Time) {
	g.Lock()
	defer g.Unlock()
	g.turnDeadline = deadline
}

// clearTurnDeadline stops the turn countdown from being shown.
func (g *Game) clearTurnDeadline() {
	g.Lock()
	defer g.Unlock()
	g.turnDeadline = time.Time{}
}

// TurnTimeRemaining returns how long the active player has left to act, or 0 if no turn
// timer is currently running.
func (g *Game) TurnTimeRemaining() time.Duration {
	g.Lock()
	defer g.Unlock()
	if g.turnDeadline.IsZero() {
		return 0
	}
	if remaining := time.Until(g.turnDeadline); remaining > 0 {
		return remaining
	}
	return 0
}

// handValue calculates the value of a blackjack hand.
func handValue(hand *bj.Hand, hidden bool) int {
	visibleValue := 0
	aces := 0
	for idx, card := range hand.Cards() {
		if hidden && idx == 0 {
			continue
		}

		rank := card.Rank
		switch rank {
		case cards.Jack, cards.Queen, cards.King:
			visibleValue += 10
		case cards.Ace:
			aces++
			visibleValue += 11
		default:
			visibleValue += int(rank)
		}
	}

	// Adjust for aces
	for aces > 0 && visibleValue > 21 {
		visibleValue -= 10
		aces--
	}

	return visibleValue
}

// createButtons creates the action buttons for the blackjack game.
func createButtons(game *Game) {
	game.joinButton = discord.ButtonComponent{
		Label:    "Join Game",
		Style:    discord.ButtonStyleSuccess,
		CustomID: "/blackjack/join/" + game.uid,
	}

	game.hitButton = discord.ButtonComponent{
		Label:    "Hit",
		Style:    discord.ButtonStylePrimary,
		CustomID: "/blackjack/hit/" + game.uid,
	}

	game.standButton = discord.ButtonComponent{
		Label:    "Stand",
		Style:    discord.ButtonStylePrimary,
		CustomID: "/blackjack/stand/" + game.uid,
	}

	game.doubleDownButton = discord.ButtonComponent{
		Label:    "Double Down",
		Style:    discord.ButtonStylePrimary,
		CustomID: "/blackjack/double-down/" + game.uid,
	}

	game.splitButton = discord.ButtonComponent{
		Label:    "Split",
		Style:    discord.ButtonStylePrimary,
		CustomID: "/blackjack/split/" + game.uid,
	}

	game.surrenderButton = discord.ButtonComponent{
		Label:    "Surrender",
		Style:    discord.ButtonStyleDanger,
		CustomID: "/blackjack/surrender/" + game.uid,
	}
}

// getUID generates the unique identifier for the blackjack game based on the guild and member IDs.
func getUID(guildID discordid.SnowflakeID, memberID discordid.SnowflakeID) string {
	config := GetConfig(guildID)
	if config.SinglePlayerMode {
		return guildID.String() + "-" + memberID.String()
	}
	return guildID.String()
}

// getUIDFromCustomID extracts the unique game identifier from a component custom ID.
func getUIDFromCustomID(customID string) string {
	parts := strings.Split(customID, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func getUIDFromComponent(e *handler.ComponentEvent) string {
	return getUIDFromCustomID(e.Data.CustomID())
}

// String returns a string representation of the Action type.
func (a Action) String() string {
	switch a {
	case Hit:
		return "Hit"
	case Stand:
		return "Stand"
	case DoubleDown:
		return "Double Down"
	case Split:
		return "Split"
	case Surrender:
		return "Surrender"
	default:
		return "Unknown"
	}
}

// String returns a string representation of the GameState type for logging and debugging purposes.
func (s GameState) String() string {
	switch s {
	case NotStarted:
		return "Not Started"
	case WaitingForPlayers:
		return "Waiting For Players"
	case StartingRound:
		return "Starting Round"
	case DealingHands:
		return "Dealing Hands"
	case Completed:
		return "Completed"
	default:
		return "Unknown State"
	}
}
