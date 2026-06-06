package race

import (
	"errors"
	"fmt"
	"goblin2/discordid"
	"goblin2/guild"
	"goblin2/internal/format"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	betButtons     = make(map[discordid.SnowflakeID]map[string]*raceButton) // guild -> label -> button
	betButtonMutex = sync.Mutex{}

	memberCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "race",
			Description: "Race game commands.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "start",
					Description: "Starts a new race.",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "stats",
					Description: "Returns the race stats for the player.",
				},
			},
		},
	}

	adminCommands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "race-admin",
			Description: "Race game admin commands.",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "reset",
					Description: "Resets a hung race.",
				},
			},
		},
	}
)

type raceButton struct {
	label string
	racer *RaceParticipant
}

// resetRace resets a hung race.
func resetRace(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	gID := guildID(e)
	ResetRace(gID)

	return e.CreateMessage(discord.MessageCreate{
		Content: "Race has been reset",
		Flags:   discord.MessageFlagEphemeral,
	})
}

// startRace starts a race that other members may join.
func startRace(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	member := e.Member()
	if member == nil {
		return serverOnly(e)
	}

	gID := discordid.NewSnowflakeID(member.GuildID)
	memberID := discordid.NewSnowflakeID(member.User.ID)

	race, err := CreateNewRace(gID)
	if err != nil {
		slog.Warn("failed to create new race",
			slog.Any("guildID", gID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		return e.CreateMessage(discord.MessageCreate{
			Content: firstToLower(err.Error()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}
	defer race.End()

	race.interaction = e

	guildMember := resolvedGuildMember(member)
	racer := getRaceMember(gID, guildMember)
	if _, err := race.addRaceParticipant(racer); err != nil {
		slog.Error("failed to add the race starter as a participant",
			slog.Any("guildID", gID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		race.End()
		return e.CreateMessage(discord.MessageCreate{
			Content: "Failed to add you as a participant to the race",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	if err := e.CreateMessage(discord.MessageCreate{
		Content: "Starting a race...",
	}); err != nil {
		race.End()
		return err
	}

	if err := raceMessage(race, "start"); err != nil {
		race.End()
		return err
	}

	slog.Info("waiting for members to join the race", slog.Any("guildID", gID))
	waitForMembersToJoin(race)

	if len(race.Racers) < race.config.MinNumRacers {
		slog.Info("race cancelled due to insufficient racers",
			slog.Any("guildID", gID),
			slog.Int("racers", len(race.Racers)),
		)
		return raceMessage(race, "cancelled")
	}

	race.setState(RaceWaitingForBets)
	if err := raceMessage(race, "betting"); err != nil {
		return err
	}
	defer removeBetButtons(race)

	slog.Info("waiting for bets",
		slog.Any("guildID", gID),
		slog.Int("racers", len(race.Racers)),
	)
	waitForBetsToBePlaced(race)

	race.setState(RaceInProgress)
	if err := raceMessage(race, "started"); err != nil {
		return err
	}

	slog.Info("race starting",
		slog.Any("guildID", gID),
		slog.Int("racers", len(race.Racers)),
		slog.Int("betsPlaced", len(race.Betters)),
	)

	race.runRace(len([]rune(race.config.Track)))
	sendRaceLegs(race)

	race.setState(RaceFinished)
	if err := raceMessage(race, "ended"); err != nil {
		return err
	}

	slog.Info("race ended", slog.Any("guildID", gID))

	return sendRaceResults(race)
}

// waitForMembersToJoin waits until members join the race before proceeding to taking bets.
func waitForMembersToJoin(race *Race) {
	memberJoinTime := time.Now().Add(race.config.WaitToStart)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	count := 0
	if err := raceMessage(race, "update"); err != nil {
		slog.Error("failed to update race message", slog.Any("error", err))
	}
	for range ticker.C {
		count++
		if count%5 == 0 {
			if err := raceMessage(race, "update"); err != nil {
				slog.Error("failed to update race message", slog.Any("error", err))
			}
		}
		if time.Until(memberJoinTime) <= 0 {
			break
		}
		if race.IsFull() {
			break
		}
	}
}

// waitForBetsToBePlaced waits until bets are placed before starting the race.
func waitForBetsToBePlaced(race *Race) {
	betEndTime := time.Now().Add(race.config.WaitForBets)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	count := 0
	if err := raceMessage(race, "betting"); err != nil {
		slog.Error("failed to update race betting message", slog.Any("error", err))
	}
	for range ticker.C {
		count++
		if count%5 == 0 {
			if err := raceMessage(race, "betting"); err != nil {
				slog.Error("failed to update race betting message", slog.Any("error", err))
			}
		}
		if time.Until(betEndTime) <= 0 {
			break
		}
	}
}

// joinRace attempts to join a race that is getting ready to start.
func joinRace(e *handler.ComponentEvent) error {
	if err := e.DeferCreateMessage(true); err != nil {
		slog.Error("failed to defer race join component response", slog.Any("error", err))
	}

	member := e.Member()
	if member == nil {
		return updateComponentResponse(e, "This command can only be used in a server.")
	}

	gID := discordid.NewSnowflakeID(member.GuildID)
	memberID := discordid.NewSnowflakeID(member.User.ID)

	race := GetCurrentRace(gID)
	if race == nil {
		slog.Warn("no race is planned",
			slog.Any("guildID", gID),
			slog.Any("memberID", memberID),
		)
		return updateComponentResponse(e, "No race is planned")
	}

	guildMember := resolvedGuildMember(member)
	raceMember := getRaceMember(gID, guildMember)

	if _, err := race.addRaceParticipant(raceMember); err != nil {
		return updateComponentResponse(e, firstToUpper(err.Error()))
	}

	slog.Debug("joined the race",
		slog.Any("guildID", gID),
		slog.Any("memberID", memberID),
	)

	if err := raceMessage(race, "join"); err != nil {
		slog.Error("failed to update race message", slog.Any("error", err))
	}

	return updateComponentResponse(e, "You have joined the race")
}

// raceStats returns a player's race stats.
func raceStats(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	member := e.Member()
	if member == nil {
		return serverOnly(e)
	}

	p := message.NewPrinter(language.AmericanEnglish)

	guildMember := resolvedGuildMember(member)
	raceMember := getRaceMember(guildMember.GuildID, guildMember)

	var totalRaces float64
	if raceMember.TotalRaces == 0 {
		totalRaces = 1
	} else {
		totalRaces = float64(raceMember.TotalRaces)
	}

	var betsMade float64
	if raceMember.BetsMade == 0 {
		betsMade = 1
	} else {
		betsMade = float64(raceMember.BetsMade)
	}

	inline := true
	embeds := []discord.Embed{
		{
			Type:  discord.EmbedTypeRich,
			Title: guildMember.Name,
			Fields: []discord.EmbedField{
				{Name: "First", Value: p.Sprintf("%d (%.0f%%)", raceMember.RacesWon, 100*float64(raceMember.RacesWon)/totalRaces), Inline: &inline},
				{Name: "Second", Value: p.Sprintf("%d (%.0f%%)", raceMember.RacesPlaced, 100*float64(raceMember.RacesPlaced)/totalRaces), Inline: &inline},
				{Name: "Third", Value: p.Sprintf("%d (%.0f%%)", raceMember.RacesShowed, 100*float64(raceMember.RacesShowed)/totalRaces), Inline: &inline},
				{Name: "Losses", Value: p.Sprintf("%d (%.0f%%)", raceMember.RacesLost, 100*float64(raceMember.RacesLost)/totalRaces), Inline: &inline},
				{Name: "Earnings", Value: p.Sprintf("%d", raceMember.TotalEarnings), Inline: &inline},
				{Name: "Bets Won", Value: p.Sprintf("%d (%.0f%%)", raceMember.BetsWon, 100*float64(raceMember.BetsWon)/betsMade), Inline: &inline},
				{Name: "Bet Earnings", Value: p.Sprintf("%d", raceMember.BetsEarnings), Inline: &inline},
			},
		},
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds: embeds,
		Flags:  discord.MessageFlagEphemeral,
	})
}

// betOnRace processes a bet placed by a member on the race.
func betOnRace(e *handler.ComponentEvent) error {
	if err := e.DeferCreateMessage(true); err != nil {
		slog.Error("failed to defer race bet component response", slog.Any("error", err))
	}

	member := e.Member()
	if member == nil {
		return updateComponentResponse(e, "This command can only be used in a server.")
	}

	gID := discordid.NewSnowflakeID(member.GuildID)
	memberID := discordid.NewSnowflakeID(member.User.ID)

	race := GetCurrentRace(gID)
	if race == nil {
		slog.Warn("no race is planned",
			slog.Any("guildID", gID),
			slog.Any("memberID", memberID),
		)
		return updateComponentResponse(e, "No race is planned")
	}

	if err := raceBetChecks(race, memberID); err != nil {
		slog.Error("unable to place bet",
			slog.Any("guildID", gID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		return updateComponentResponse(e, firstToUpper(err.Error()))
	}

	participant := race.getRaceParticipant(memberID)
	var betMember *RaceMember
	if participant != nil && participant.Member != nil {
		betMember = participant.Member
	} else {
		guildMember := resolvedGuildMember(member)
		betMember = getRaceMember(gID, guildMember)
	}

	customID := e.Data.CustomID()
	raceParticipant := getCurrentRaceParticipant(race, customID)
	if raceParticipant == nil {
		slog.Error("race participant not found",
			slog.Any("guildID", gID),
			slog.Any("memberID", memberID),
			slog.String("customID", customID),
		)
		return updateComponentResponse(e, "Race participant not found")
	}

	better := getRaceBetter(betMember, raceParticipant)
	if err := placeBet(race, better); err != nil {
		slog.Error("unable to place bet",
			slog.Any("guildID", gID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		return updateComponentResponse(e, fmt.Sprintf("Unable to place a bet. Error: %s", err.Error()))
	}

	p := message.NewPrinter(language.AmericanEnglish)
	return updateComponentResponse(e, p.Sprintf("You have placed a %d credit bet on %s", race.config.BetAmount, raceParticipant.Member.guildMember.Name))
}

// createBetButtons returns the buttons for the racers.
func createBetButtons(race *Race) []discord.LayoutComponent {
	buttonsPerRow := 5
	rows := make([]discord.LayoutComponent, 0, (len(race.Racers)+buttonsPerRow-1)/buttonsPerRow)

	racersIncludedInButtons := 0
	for len(race.Racers) > racersIncludedInButtons {
		racersNotInButtons := len(race.Racers) - racersIncludedInButtons
		buttonsForNextRow := min(buttonsPerRow, racersNotInButtons)

		buttons := make([]discord.InteractiveComponent, 0, buttonsForNextRow)
		for i := range buttonsForNextRow {
			index := i + racersIncludedInButtons
			racer := race.Racers[index]
			buttons = append(buttons, discord.ButtonComponent{
				Label:    racer.Member.guildMember.Name,
				Style:    discord.ButtonStylePrimary,
				CustomID: createBetButton(racer).label,
			})
		}
		racersIncludedInButtons += buttonsForNextRow

		rows = append(rows, discord.ActionRowComponent{
			Components: buttons,
		})
	}

	return rows
}

// createBetButton creates and returns a new race button for the racer.
func createBetButton(rp *RaceParticipant) *raceButton {
	betButtonMutex.Lock()
	defer betButtonMutex.Unlock()

	buttons := betButtons[rp.Member.GuildID]
	if buttons == nil {
		buttons = make(map[string]*raceButton)
		betButtons[rp.Member.GuildID] = buttons
	}

	label := fmt.Sprintf("/race/bet/%s/%s", rp.Member.GuildID, rp.Member.MemberID)
	button := buttons[label]
	if button != nil {
		return button
	}

	button = &raceButton{
		label: label,
		racer: rp,
	}
	buttons[button.label] = button

	return button
}

// removeBetButtons removes the buttons for the current race.
func removeBetButtons(race *Race) {
	betButtonMutex.Lock()
	defer betButtonMutex.Unlock()

	betButtons[race.GuildID] = make(map[string]*raceButton)
}

// raceMessage sends or edits the main race message.
//
// Important: Race.interaction is the original *handler.CommandEvent. We keep that
// event so later updates can call UpdateInteractionResponse using the original
// application ID and interaction token.
func raceMessage(race *Race, action string) error {
	if race.interaction == nil {
		return errors.New("interaction is nil for the race")
	}

	p := message.NewPrinter(language.AmericanEnglish)

	racerNames := race.GetRacerNames()
	if len(racerNames) == 0 {
		racerNames = []string{"None yet"}
	}

	var msg string
	switch action {
	case "start", "join", "update":
		until := time.Until(race.RaceStartTime.Add(race.config.WaitToStart))
		msg = p.Sprintf(":triangular_flag_on_post: A race is starting! Click the button to join the race! :triangular_flag_on_post:\nThe race will begin in %s!", format.Duration(until))
	case "betting":
		until := time.Until(race.RaceStartTime.Add(race.config.WaitToStart + race.config.WaitForBets))
		msg = p.Sprintf(":triangular_flag_on_post: The racers have been set - betting is now open! :triangular_flag_on_post:\nYou have %s to place a %d credit bet!", format.Duration(until), race.config.BetAmount)
	case "started":
		msg = ":checkered_flag: The race is now in progress! :checkered_flag:"
	case "ended":
		msg = ":checkered_flag: The race has ended - let's find out the results. :checkered_flag:"
	case "cancelled":
		msg = "Not enough players entered the race, so it was cancelled."
	default:
		return fmt.Errorf("unrecognized action: %s", action)
	}

	inline := false
	embeds := []discord.Embed{
		{
			Type:  discord.EmbedTypeRich,
			Title: "Race",
			Fields: []discord.EmbedField{
				{
					Name:   msg,
					Value:  "\u200b",
					Inline: &inline,
				},
				{
					Name:   p.Sprintf("Racers (%d)", len(race.Racers)),
					Value:  strings.Join(racerNames, ", "),
					Inline: &inline,
				},
			},
		},
	}

	components := []discord.LayoutComponent{}
	switch action {
	case "start", "join", "update":
		components = []discord.LayoutComponent{
			discord.ActionRowComponent{
				Components: []discord.InteractiveComponent{
					discord.ButtonComponent{
						Label:    "Join",
						Style:    discord.ButtonStyleSuccess,
						CustomID: fmt.Sprintf("/race/join/%s", race.GuildID),
					},
				},
			},
		}
	case "betting":
		components = createBetButtons(race)
	}

	_, err := race.interaction.Client().Rest.UpdateInteractionResponse(
		race.interaction.ApplicationID(),
		race.interaction.Token(),
		discord.MessageUpdate{
			Content:    new(string),
			Embeds:     &embeds,
			Components: &components,
		},
	)
	return err
}

// sendRaceLegs sends the race so guild members can watch it play out.
func sendRaceLegs(race *Race) {
	channelID := race.interaction.Channel().ID()
	track := getCurrentTrack(race.RaceLegs[0], race.config)

	m, err := race.interaction.Client().Rest.CreateMessage(channelID, discord.MessageCreate{
		Content: fmt.Sprintf("%s\n", track),
	})
	if err != nil {
		slog.Error("failed to send message at the start of the race",
			slog.Any("guildID", race.GuildID),
			slog.Any("channelID", channelID),
			slog.Any("error", err),
		)
		return
	}

	slog.Debug("preparing to send race legs", slog.Any("guildID", race.GuildID))
	for _, raceLeg := range race.RaceLegs {
		time.Sleep(2 * time.Second)
		track = getCurrentTrack(raceLeg, race.config)

		content := fmt.Sprintf("%s\n", track)
		if _, err = race.interaction.Client().Rest.UpdateMessage(channelID, m.ID, discord.MessageUpdate{
			Content: &content,
		}); err != nil {
			slog.Error("failed to update race message",
				slog.Any("guildID", race.GuildID),
				slog.Any("channelID", channelID),
				slog.Any("error", err),
			)
		}
	}
}

// getCurrentTrack returns the current position of all racers on the track.
func getCurrentTrack(raceLeg *RaceLeg, config *Config) string {
	var track strings.Builder
	for _, pos := range raceLeg.ParticipantPositions {
		name := pos.RaceParticipant.Member.guildMember.Name
		racer := pos.RaceParticipant.Racer

		position := max(0, pos.Position)

		start, end := splitString(config.Track, position)
		currentTrackLine := start + racer.Emoji + end

		line := fmt.Sprintf("%s **%s %s** [%s]\n", config.EndingLine, currentTrackLine, config.StartingLine, name)
		track.WriteString(line)
	}
	return track.String()
}

// sendRaceResults sends the results of a race to the Discord server.
func sendRaceResults(race *Race) error {
	p := message.NewPrinter(language.English)
	raceResults := make([]discord.EmbedField, 0, 4)

	racers := race.Racers
	results := race.RaceResult
	inline := true

	if results.Win != nil {
		raceParticipant := results.Win.Participant
		memberName := raceParticipant.Member.guildMember.Name
		raceResults = append(raceResults, discord.EmbedField{
			Name:   p.Sprintf(":first_place: %s", memberName),
			Value:  p.Sprintf("%s\n%.2fs\nPrize: %d", raceParticipant.Racer.Emoji, results.Win.RaceTime, results.Win.Winnings),
			Inline: &inline,
		})
	}

	if results.Place != nil {
		raceParticipant := results.Place.Participant
		memberName := raceParticipant.Member.guildMember.Name
		raceResults = append(raceResults, discord.EmbedField{
			Name:   p.Sprintf(":second_place: %s", memberName),
			Value:  p.Sprintf("%s\n%.2fs\nPrize: %d", raceParticipant.Racer.Emoji, results.Place.RaceTime, results.Place.Winnings),
			Inline: &inline,
		})
	}

	if results.Show != nil {
		raceParticipant := results.Show.Participant
		memberName := raceParticipant.Member.guildMember.Name
		raceResults = append(raceResults, discord.EmbedField{
			Name:   p.Sprintf(":third_place: %s", memberName),
			Value:  p.Sprintf("%s\n%.2fs\nPrize: %d", raceParticipant.Racer.Emoji, results.Show.RaceTime, results.Show.Winnings),
			Inline: &inline,
		})
	}

	betWinners := make([]string, 0, len(race.Betters))
	for _, bet := range race.Betters {
		if bet.Winnings > 0 {
			memberName := bet.Member.guildMember.Name
			betWinners = append(betWinners, memberName)
		}
	}

	winners := "No one guessed the winner."
	if len(betWinners) > 0 {
		winners = strings.Join(betWinners, "\n")
	}

	block := false
	betEarnings := race.config.BetAmount * len(racers)
	raceResults = append(raceResults, discord.EmbedField{
		Name:   p.Sprintf("Bet earnings of %d", betEarnings),
		Value:  winners,
		Inline: &block,
	})

	_, err := race.interaction.Client().Rest.CreateMessage(race.interaction.Channel().ID(), discord.MessageCreate{
		Embeds: []discord.Embed{
			{
				Title:  "Race Results",
				Fields: raceResults,
			},
		},
	})
	return err
}

// getCurrentRaceParticipant takes a custom button ID and returns the corresponding racer.
func getCurrentRaceParticipant(race *Race, customID string) *RaceParticipant {
	betButtonMutex.Lock()
	defer betButtonMutex.Unlock()

	buttons := betButtons[race.GuildID]
	if buttons == nil {
		return nil
	}

	button := buttons[customID]
	if button == nil {
		return nil
	}

	return button.racer
}

func updateComponentResponse(e *handler.ComponentEvent, content string) error {
	_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
		Content: &content,
	})
	return err
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

	ok, err := guildMember.IsAdmin(e.Client(), guild.GetGuild(guildMember.GuildID))
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

func splitString(s string, pos int) (string, string) {
	runes := []rune(s)
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}

	return string(runes[:pos]), string(runes[pos:])
}

func firstToUpper(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func firstToLower(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
