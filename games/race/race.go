package race

import (
	"errors"
	"goblin2/discordid"
	"goblin2/internal/cache"
	"goblin2/stats"
	"log/slog"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"github.com/disgoorg/disgo/handler"
)

const (
	raceWaitingForRacers = iota
	raceWaitingForBets
	raceInProgress
	raceFinished
)

const (
	raceCacheTTL             = 2 * time.Hour
	raceCacheCleanupInterval = 5 * time.Minute
)

type raceCacheKey struct {
	guildID discordid.SnowflakeID
}

var (
	lastRaceTimes = cache.New[raceCacheKey, time.Time](raceCacheTTL, raceCacheCleanupInterval)
	currentRaces  = cache.New[raceCacheKey, *Race](raceCacheTTL, raceCacheCleanupInterval)
	raceLock      = sync.Mutex{}
)

// Race represents a race currently in progress.
// It contains a list of racers who are particpaing in the race as well as
// betters on the outcome of the race.
type Race struct {
	GuildID       discordid.SnowflakeID // Guild (server) on which the race is taking place
	Racers        []*Participant        // The list of participants who are racing
	Betters       []*Better             // The list of members who are betting on the outcome of the race
	RaceLegs      []*Leg                // The list of legs in the race
	RaceResult    *Result               // The results of the race
	RaceStartTime time.Time             // The time at which the race is started (first created)
	state         int                   // The state of the race
	raceAvatars   []*Avatar             // The avatars of the racers
	interaction   *handler.CommandEvent // Original interaction used for editing the race planning message
	config        *Config               // Race configuration (avoids having to read from the database)
	mutex         sync.Mutex            // Lock used to synchronize access to the race
}

// Participant is a member who is racing. This includes the member and the racer assigned to them.
type Participant struct {
	Member *Member // Member who is racing
	Racer  *Avatar // Racer assigned to the member
}

// Better is a member betting on the outcome of the race.
type Better struct {
	Member   *Member      // Member who is betting on the outcome of the race
	Racer    *Participant // Racer on which the member is betting
	Winnings int          // Amount won by the better
}

// Result is the final results of the race. This includes the winner, 2nd place, and 3rd place finishers, as
// well as the speed at which they finished.
type Result struct {
	Win   *ParticipantResult // First place in the race
	Place *ParticipantResult // Second place in the race
	Show  *ParticipantResult // Third place in the race
}

type ParticipantResult struct {
	Participant *Participant // Participant in the race
	RaceTime    float64      // Time at which the participant finished
	Winnings    int          // Amount the participant won
}

// Leg is a single leg in a race. This covers the movement for all racers during the given turn.
type Leg struct {
	ParticipantPositions []*ParticipantPosition // The results for each member in a given leg of the race
}

// ParticipantPosition is used to track the movement of a given member during a single leg of a race.
type ParticipantPosition struct {
	RaceParticipant *Participant // Member who is racing
	Position        int          // Position of the member on the track for a given leg of the race
	Movement        int          // Amount of movement for the member on the track for a given leg of the race
	Speed           float64      // Speed at which the member moved during the leg of the race
	Turn            int          // Turn in which the member is racing
	Finished        bool         // The member has crossed the finish line
}

// GetCurrentRace gets the race for the guild. If a race isn't in progress, then a new one is created.
func GetCurrentRace(guildID discordid.SnowflakeID) *Race {
	key := raceCacheKey{
		guildID: guildID,
	}

	race, _ := currentRaces.Get(key)
	return race
}

// CreateNewRace creates a new race for the guild. If a race is already in progress or the racers are resting,
// then an error is returned.
func CreateNewRace(guildID discordid.SnowflakeID) (*Race, error) {
	raceLock.Lock()
	defer raceLock.Unlock()

	if err := raceStartChecks(guildID); err != nil {
		return nil, err
	}

	config := GetConfig(guildID)

	race := &Race{
		GuildID:       guildID,
		Racers:        make([]*Participant, 0, 10),
		Betters:       make([]*Better, 0, 10),
		RaceStartTime: time.Now(),
		RaceResult:    &Result{},
		state:         raceWaitingForRacers,
		raceAvatars:   getRaceAvatars(guildID, config.Theme),
		interaction:   nil,
		config:        config,
		mutex:         sync.Mutex{},
	}

	key := raceCacheKey{
		guildID: guildID,
	}
	currentRaces.Set(key, race)

	return race, nil
}

// Set the race state
func (r *Race) setState(state int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.state = state
}

// addRaceParticipant returns a new race participant for a member in the race. The race
// participant is added to the race.
func (r *Race) addRaceParticipant(member *Member) (*Participant, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if err := raceJoinChecks(r, member.MemberID); err != nil {
		slog.Warn("unable to join the race",
			slog.Any("guildID", r.GuildID),
			slog.Any("memberID", member.MemberID),
			slog.Any("error", err),
		)
		return nil, err
	}

	currentRace := GetCurrentRace(r.GuildID)
	if r != currentRace {
		slog.Warn("current race has changed since addRaceParticipant was called",
			slog.Any("guildID", r.GuildID),
		)
		return nil, errors.New("current race has changed")
	}

	participant := &Participant{
		Member: member,
		Racer:  getRaceAvatar(r),
	}
	r.Racers = append(r.Racers, participant)

	return participant, nil
}

// getRaceParticipant returns a racer for a given race. If the member isn't in the race, then
// nil is returned.
func (r *Race) getRaceParticipant(memberID discordid.SnowflakeID) *Participant {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, racer := range r.Racers {
		if racer.Member.MemberID == memberID {
			return racer
		}
	}
	return nil
}

// getRaceBetter returns a new better for a race.
func getRaceBetter(member *Member, racer *Participant) *Better {
	raceBetter := &Better{
		Member: member,
		Racer:  racer,
	}

	return raceBetter
}

// addBetter adds a better for the given race.
func (r *Race) addBetter(better *Better) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.Betters = append(r.Betters, better)
	slog.Debug("add better to current race",
		slog.Any("guildID", r.GuildID),
		slog.Any("memberID", better.Member.MemberID),
	)

	return nil
}

// runRace runs a race, calculating the results of each leg of the race and the
// ultimate winners of the race.
func (r *Race) runRace(trackLength int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Create the initial starting positions and add them to an initial race leg
	raceLeg := &Leg{
		ParticipantPositions: make([]*ParticipantPosition, 0, len(r.Racers)),
	}
	for _, racer := range r.Racers {
		participantPosition := &ParticipantPosition{
			RaceParticipant: racer,
			Position:        trackLength,
		}
		raceLeg.ParticipantPositions = append(raceLeg.ParticipantPositions, participantPosition)
	}
	r.RaceLegs = append(r.RaceLegs, raceLeg)
	previousLeg := raceLeg

	// Run the race until all racers cross the finish line
	turn := 0
	stillRacing := true
	for stillRacing {
		turn++

		// Create and add a new race leg
		newRaceLeg := &Leg{
			ParticipantPositions: make([]*ParticipantPosition, 0, len(r.Racers)),
		}

		// Run the new race leg
		stillRacing = false
		for _, previousPosition := range previousLeg.ParticipantPositions {
			newPosition := moveRacer(previousPosition, turn)
			newRaceLeg.ParticipantPositions = append(newRaceLeg.ParticipantPositions, newPosition)
			if !newPosition.Finished {
				stillRacing = true
			}
		}

		r.RaceLegs = append(r.RaceLegs, newRaceLeg)
		previousLeg = newRaceLeg
	}

	calculateWinnings(r, previousLeg)

	if len(r.Racers) <= 2 {
		slog.Info("race finished",
			slog.Any("guildID", r.GuildID),
			slog.Int("numRacers", len(r.Racers)),
			slog.String("first", r.RaceResult.Win.Participant.Member.guildMember.Name),
			slog.String("second", r.RaceResult.Place.Participant.Member.guildMember.Name),
		)
	} else {
		slog.Info("race finished",
			slog.Any("guildID", r.GuildID),
			slog.Int("numRacers", len(r.Racers)),
			slog.String("first", r.RaceResult.Win.Participant.Member.guildMember.Name),
			slog.String("second", r.RaceResult.Place.Participant.Member.guildMember.Name),
			slog.String("third", r.RaceResult.Show.Participant.Member.guildMember.Name),
		)
	}
	lastLeg := r.RaceLegs[len(r.RaceLegs)-1]
	for i, position := range lastLeg.ParticipantPositions {
		slog.Info("race result",
			slog.Any("guildID", r.GuildID),
			slog.Any("memberID", position.RaceParticipant.Member.MemberID),
			slog.String("memberName", position.RaceParticipant.Member.guildMember.Name),
			slog.Int("position", i+1),
			slog.Float64("raceTime", position.Speed),
		)
	}
}

// End ends the current race.
func (r *Race) End() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	key := raceCacheKey{
		guildID: r.GuildID,
	}

	// The race runs if there are 2 or more racers. If that is the case, then reset the time the last
	// successful race ran.
	raceLock.Lock()
	if len(r.Racers) >= r.config.MinNumRacers {
		lastRaceTimes.SetWithTTL(key, time.Now(), r.config.WaitBetweenRaces)
	}
	currentRaces.Delete(key)
	raceLock.Unlock()

	if r.RaceResult != nil && len(r.Racers) >= r.config.MinNumRacers {
		slog.Debug("processing race results",
			slog.Any("guildID", r.GuildID),
			slog.Int("numRacers", len(r.Racers)),
		)
		for _, racer := range r.Racers {
			switch {
			case r.RaceResult.Win != nil && racer.Member.MemberID == r.RaceResult.Win.Participant.Member.MemberID:
				racer.Member.WinRace(r.RaceResult.Win.Winnings)
			case r.RaceResult.Place != nil && racer.Member.MemberID == r.RaceResult.Place.Participant.Member.MemberID:
				racer.Member.PlaceInRace(r.RaceResult.Place.Winnings)
			case r.RaceResult.Show != nil && racer.Member.MemberID == r.RaceResult.Show.Participant.Member.MemberID:
				racer.Member.ShowInRace(r.RaceResult.Show.Winnings)
			default:
				racer.Member.LoseRace()
			}
		}

		slog.Debug("processing race bets",
			slog.Any("guildID", r.GuildID),
			slog.Int("numBetters", len(r.Betters)),
		)
		// Pay the winning bets
		for _, better := range r.Betters {
			if better.Winnings != 0 {
				better.Member.WinBet(better.Winnings)
			} else {
				better.Member.LoseBet()
			}
		}
	}

	memberIDs := make([]discordid.SnowflakeID, 0, len(r.Racers))
	for _, racer := range r.Racers {
		memberIDs = append(memberIDs, racer.Member.MemberID)
	}
	if len(memberIDs) > 0 {
		stats.UpdateGameStats(r.GuildID, "race", memberIDs)
	}
}

// ResetRace resets a hung race for a given guild.
func ResetRace(guildID discordid.SnowflakeID) {
	raceLock.Lock()
	defer raceLock.Unlock()

	key := raceCacheKey{
		guildID: guildID,
	}

	currentRaces.Delete(key)
	lastRaceTimes.Delete(key)
	slog.Info("reset race",
		slog.Any("guildID", guildID),
	)
}

// IsFull checks to see if the race has already reached the maximum number of racers.
func (r *Race) IsFull() bool {
	raceLock.Lock()
	defer raceLock.Unlock()

	return len(r.Racers) >= r.config.MaxNumRacers
}

// GetRacerNames returns the names of the racers in the race.
func (r *Race) GetRacerNames() []string {
	raceLock.Lock()
	defer raceLock.Unlock()

	racerNames := make([]string, 0, len(r.Racers))
	for _, racer := range r.Racers {
		racerNames = append(racerNames, racer.Member.guildMember.Name)
	}
	return racerNames
}

// getRaceAvatar returns a random race avatar to be used by a race participant.
func getRaceAvatar(race *Race) *Avatar {
	if len(race.raceAvatars) == 0 {
		race.raceAvatars = getRaceAvatars(race.GuildID, race.config.Theme)
	}

	index := len(race.raceAvatars) - 1
	avatar := race.raceAvatars[index]
	race.raceAvatars[index] = nil
	race.raceAvatars = race.raceAvatars[:index]
	return avatar
}

// moveRacer returns the new race position for a participant based on the previous position and the current turn.
func moveRacer(previousPosition *ParticipantPosition, turn int) *ParticipantPosition {
	// Already done with the race
	if previousPosition.Position <= 0 {
		newPosition := &ParticipantPosition{
			RaceParticipant: previousPosition.RaceParticipant,
			Finished:        true,
			Speed:           previousPosition.Speed,
		}
		return newPosition
	}

	movement := previousPosition.RaceParticipant.Racer.calculateMovement(turn)
	newPosition := &ParticipantPosition{
		RaceParticipant: previousPosition.RaceParticipant,
		Position:        previousPosition.Position - movement,
		Movement:        movement,
		Turn:            previousPosition.Turn + 1,
		Finished:        false,
	}
	newPosition.Speed = float64(newPosition.Turn)
	if newPosition.Position <= 0 {
		newPosition.Speed += float64(previousPosition.Position) / float64(movement)
	}

	return newPosition
}

// raceStartChecks checks to see if a race can be started.
func raceStartChecks(guildID discordid.SnowflakeID) error {
	config := GetConfig(guildID)

	key := raceCacheKey{
		guildID: guildID,
	}

	race, _ := currentRaces.Get(key)
	if race != nil {
		slog.Debug("race already in progress",
			slog.Any("guildID", guildID),
		)
		return ErrRaceAlreadyInProgress
	}

	lastRaceTime, ok := lastRaceTimes.Get(key)
	if ok && time.Since(lastRaceTime) < config.WaitBetweenRaces {
		timeSinceLastRace := time.Since(lastRaceTime)
		timeUntilRaceCanStart := config.WaitBetweenRaces - timeSinceLastRace
		slog.Debug("racers are resting",
			slog.Any("guildID", guildID),
			slog.Duration("timeUntilRaceCanStart", timeUntilRaceCanStart),
		)
		return ErrRacersAreResting{timeUntilRaceCanStart}
	}
	lastRaceTimes.Delete(key)

	return nil
}

// raceJoinChecks checks to see if a racer is able to join the race.
func raceJoinChecks(race *Race, memberID discordid.SnowflakeID) error {
	if race.state == raceWaitingForBets {
		slog.Debug("betting has opened",
			slog.Any("guildID", race.GuildID),
		)
		return ErrBettingHasOpened
	}

	if race.state > raceWaitingForBets {
		slog.Debug("race has started",
			slog.Any("guildID", race.GuildID),
		)
		return ErrRaceHasStarted
	}

	if len(race.Racers) >= race.config.MaxNumRacers {
		slog.Debug("too many racers already joined",
			slog.Any("guildID", race.GuildID),
			slog.Int("maxNumRacers", race.config.MaxNumRacers),
			slog.Int("numRacers", len(race.Racers)),
		)
		return ErrRaceAlreadyFull
	}

	for _, r := range race.Racers {
		if r.Member.MemberID == memberID {
			return ErrAlreadyJoinedRace
		}
	}

	return nil
}

// placeBet processes a bet placed by a member on the race
func placeBet(race *Race, better *Better) error {
	if err := better.Member.PlaceBet(race.config.BetAmount); err != nil {
		return err
	}

	if err := race.addBetter(better); err != nil {
		slog.Debug("failed to add better",
			slog.Any("guildID", race.GuildID),
			slog.Any("memberID", better.Member.MemberID),
			slog.Any("error", err),
		)
		return err
	}
	return nil
}

// raceBetChecks checks to see if a better is able to place a bet on the current race.
func raceBetChecks(race *Race, memberID discordid.SnowflakeID) error {
	race.mutex.Lock()
	defer race.mutex.Unlock()

	if race.state < raceWaitingForBets {
		slog.Debug("betting has not opened yet",
			slog.Any("guildID", race.GuildID),
		)
		return ErrBettingNotOpened
	}

	if race.state > raceWaitingForBets {
		slog.Debug("race has started, so not accepting bets",
			slog.Any("guildID", race.GuildID),
		)
		return ErrRaceHasStarted
	}

	for _, b := range race.Betters {
		if b.Member.MemberID == memberID {
			return ErrAlreadyBetOnRace
		}
	}

	return nil
}

// calculateWinnings calculates the earnings for the racers that win, place, or show.
func calculateWinnings(race *Race, lastLeg *Leg) {
	source := rand.NewPCG(rand.Uint64(), rand.Uint64())
	r := rand.New(source)

	// sort the participants in the final race leg
	sort.Slice(lastLeg.ParticipantPositions, func(i, j int) bool {
		if lastLeg.ParticipantPositions[i].Speed == lastLeg.ParticipantPositions[j].Speed {
			return r.IntN(2) == 0
		}
		return lastLeg.ParticipantPositions[i].Speed < lastLeg.ParticipantPositions[j].Speed
	})

	// Calculate the winners of the race and save in the results
	prize := r.IntN(race.config.MaxPrizeAmount-race.config.MinPrizeAmount) + race.config.MinPrizeAmount
	prize *= len(race.Racers)

	// Assign the purse for the winner
	if len(lastLeg.ParticipantPositions) > 0 {
		racePosition := lastLeg.ParticipantPositions[0]
		race.RaceResult.Win = &ParticipantResult{
			Participant: racePosition.RaceParticipant,
			RaceTime:    racePosition.Speed,
			Winnings:    prize,
		}
	}

	// Assign the purse for the second place finisher
	if len(lastLeg.ParticipantPositions) > 1 {
		racePosition := lastLeg.ParticipantPositions[1]
		race.RaceResult.Place = &ParticipantResult{
			Participant: racePosition.RaceParticipant,
			RaceTime:    racePosition.Speed,
			Winnings:    int(float64(prize) * 0.75),
		}
	}

	// Assign the purse for the third place finisher
	if len(lastLeg.ParticipantPositions) > 2 {
		racePosition := lastLeg.ParticipantPositions[2]
		race.RaceResult.Show = &ParticipantResult{
			Participant: racePosition.RaceParticipant,
			RaceTime:    racePosition.Speed,
			Winnings:    int(float64(prize) * 0.50),
		}
	}

	// Pay the winning bets
	if race.RaceResult.Win != nil {
		winner := race.RaceResult.Win.Participant
		winningBet := race.config.BetAmount * len(race.Racers)
		for _, better := range race.Betters {
			if better.Racer == winner {
				better.Winnings = winningBet
			}
		}
	}
}

// CloseRaceCaches stops the race cache cleanup goroutines and clears cached entries.
func CloseRaceCaches() {
	currentRaces.Destroy()
	lastRaceTimes.Destroy()
}
