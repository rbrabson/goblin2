package race

import (
	"fmt"
	"goblin2/bank"
	"goblin2/guild"
	"goblin2/internal/discordid"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Member represents a member of a guild that is assigned a racer
type Member struct {
	ID            bson.ObjectID         `json:"_id,omitempty" bson:"_id,omitempty"`
	GuildID       discordid.SnowflakeID `json:"guild_id" bson:"guild_id"`
	MemberID      discordid.SnowflakeID `json:"member_id" bson:"member_id"`
	RacesLost     int                   `json:"races_lost" bson:"races_lost"`
	RacesPlaced   int                   `json:"races_placed" bson:"races_placed"`
	RacesShowed   int                   `json:"races_showed" bson:"races_showed"`
	RacesWon      int                   `json:"races_won" bson:"races_won"`
	TotalRaces    int                   `json:"total_races" bson:"total_races"`
	BetsEarnings  int                   `json:"bets_earnings" bson:"bets_earnings"`
	BetsMade      int                   `json:"bets_made" bson:"bets_made"`
	BetsWon       int                   `json:"bets_won" bson:"bets_won"`
	TotalEarnings int                   `json:"total_earnings" bson:"total_earnings"`
	guildMember   *guild.Member         `bson:"-"`
}

// getMember gets a race member. THe member is created if it doesn't exist.
func getMember(guildID discordid.SnowflakeID, guildMember *guild.Member) *Member {
	member := readRaceMember(guildID, guildMember.MemberID)
	if member == nil {
		member = newMember(guildID, guildMember.MemberID)
	}
	var err error
	member.guildMember, err = guild.GetMemberByID(guildID, guildMember.MemberID)
	if err != nil {
		slog.Error("error getting guild member",
			slog.Any("guildID", guildID),
			slog.Any("memberID", guildMember.MemberID),
			slog.Any("error", err),
		)
	}
	return member
}

// newMember returns a new race member for the guild. The member is saved to
// the database.
func newMember(guildID, memberID discordid.SnowflakeID) *Member {
	member := &Member{
		GuildID:  guildID,
		MemberID: memberID,
	}

	writeRaceMember(member)
	slog.Debug("new race member",
		slog.Any("guildID", guildID),
		slog.Any("memberID", memberID),
	)

	return member
}

// WinRace is called when the race member won a race.
func (m *Member) WinRace(amount int) {
	bankAccount := bank.GetAccount(m.GuildID, m.MemberID)
	if err := bankAccount.Deposit(amount); err != nil {
		slog.Error("error depositing race win amount",
			slog.Any("guildID", m.GuildID),
			slog.Any("memberID", m.MemberID),
			slog.Int("amount", amount),
			slog.Any("error", err),
		)
	}

	m.TotalRaces++
	m.RacesWon++
	m.TotalEarnings += amount
	writeRaceMember(m)

	slog.Debug("won race",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
		slog.Int("winnings", amount),
	)
}

// PlaceInRace is called when the race member places (comes in 2nd) in a race.
func (m *Member) PlaceInRace(amount int) {
	bankAccount := bank.GetAccount(m.GuildID, m.MemberID)
	if err := bankAccount.Deposit(amount); err != nil {
		slog.Error("error depositing race place amount",
			slog.Any("guildID", m.GuildID),
			slog.Any("memberID", m.MemberID),
			slog.Int("amount", amount),
			slog.Any("error", err),
		)
	}

	m.TotalRaces++
	m.RacesPlaced++
	m.TotalEarnings += amount
	writeRaceMember(m)

	slog.Debug("placed in race",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
		slog.Int("winnings", amount),
	)
}

// ShowInRace is called when the race member shows (comes in 3rd) in a race.
func (m *Member) ShowInRace(amount int) {
	bankAccount := bank.GetAccount(m.GuildID, m.MemberID)
	if err := bankAccount.Deposit(amount); err != nil {
		slog.Error("error depositing race show amount",
			slog.Any("guildID", m.GuildID),
			slog.Any("memberID", m.MemberID),
			slog.Int("amount", amount),
			slog.Any("error", err),
		)
	}

	m.TotalRaces++
	m.RacesShowed++
	m.TotalEarnings += amount
	writeRaceMember(m)

	slog.Debug("showed in race",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
		slog.Int("winnings", amount),
	)
}

// LoseRace is called when the race member fails to win, place, or show in a race.
func (m *Member) LoseRace() {
	m.TotalRaces++
	m.RacesLost++
	writeRaceMember(m)

	slog.Debug("lost race",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
	)
}

// PlaceBet is used to place a bet on a member of a race.
func (m *Member) PlaceBet(betAmount int) error {
	bankAccount := bank.GetAccount(m.GuildID, m.MemberID)
	err := bankAccount.Withdraw(betAmount)
	if err != nil {
		return err
	}

	m.BetsMade++
	m.TotalEarnings -= betAmount

	slog.Debug("placed bet",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
		slog.Int("betAmount", betAmount),
	)

	return nil
}

// WinBet is used when a member wins a bet on a race.
func (m *Member) WinBet(winnings int) {
	bankAccount := bank.GetAccount(m.GuildID, m.MemberID)
	if err := bankAccount.Deposit(winnings); err != nil {
		slog.Error("error depositing race win bet amount",
			slog.Any("guildID", m.GuildID),
			slog.Any("memberID", m.MemberID),
			slog.Int("amount", winnings),
			slog.Any("error", err),
		)
	}

	m.BetsWon++
	m.BetsEarnings += winnings
	m.TotalEarnings += winnings
	writeRaceMember(m)

	slog.Debug("won bet",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
		slog.Int("winnings", winnings),
	)
}

// LoseBet is used when a member loses a bet on a race.
func (m *Member) LoseBet() {
	writeRaceMember(m)

	slog.Debug("lost bet",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
	)
}

func (m *Member) String() string {
	return fmt.Sprintf("RaceMember{GuildID: %s, MemberID: %s, RacesLost: %d, RacesPlaced: %d, RacesShowed: %d, RacesWon: %d, TotalRaces: %d, BetsEarnings: %d, BetsMade: %d, BetsWon: %d, TotalEarnings: %d}",
		m.GuildID,
		m.MemberID,
		m.RacesLost,
		m.RacesPlaced,
		m.RacesShowed,
		m.RacesWon,
		m.TotalRaces,
		m.BetsEarnings,
		m.BetsMade,
		m.BetsWon,
		m.TotalEarnings,
	)
}
