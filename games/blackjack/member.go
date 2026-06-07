package blackjack

import (
	"goblin2/internal/discordid"
	"strconv"
	"time"

	bj "github.com/rbrabson/blackjack"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Member represents a member's statistics for the blackjack game.
type Member struct {
	ID           bson.ObjectID         `json:"id" bson:"_id,omitempty"`
	GuildID      discordid.SnowflakeID `json:"guild_id" bson:"guild_id"`
	MemberID     discordid.SnowflakeID `json:"member_id" bson:"member_id"`
	RoundsPlayed int                   `json:"rounds_played" bson:"rounds_played"`
	HandsPlayed  int                   `json:"hands_played" bson:"hands_played"`
	Wins         int                   `json:"wins" bson:"wins"`
	Losses       int                   `json:"losses" bson:"losses"`
	Pushes       int                   `json:"pushes" bson:"pushes"`
	Blackjacks   int                   `json:"blackjacks" bson:"blackjacks"`
	Splits       int                   `json:"splits" bson:"splits"`
	Surrenders   int                   `json:"surrenders" bson:"surrenders"`
	CreditsBet   int                   `json:"credits_bet" bson:"credits_bet"`
	CreditsWon   int                   `json:"credits_won" bson:"credits_won"`
	CreditsLost  int                   `json:"credits_lost" bson:"credits_lost"`
	LastPlayed   time.Time             `json:"last_played" bson:"last_played"`
}

// String returns a string representation of the Member struct.
func (m *Member) String() string {
	return "Member{" +
		"ID: " + m.ID.Hex() +
		", GuildID: " + m.GuildID.String() +
		", MemberID: " + m.MemberID.String() +
		", RoundsPlayed: " + strconv.Itoa(m.RoundsPlayed) +
		", HandsPlayed: " + strconv.Itoa(m.HandsPlayed) +
		", Wins: " + strconv.Itoa(m.Wins) +
		", Losses: " + strconv.Itoa(m.Losses) +
		", Pushes: " + strconv.Itoa(m.Pushes) +
		", Blackjacks: " + strconv.Itoa(m.Blackjacks) +
		", Splits: " + strconv.Itoa(m.Splits) +
		", Surrenders: " + strconv.Itoa(m.Surrenders) +
		", CreditsBet: " + strconv.Itoa(m.CreditsBet) +
		", CreditsWon: " + strconv.Itoa(m.CreditsWon) +
		", CreditsLost: " + strconv.Itoa(m.CreditsLost) +
		", LastPlayed: " + m.LastPlayed.String() +
		"}"
}

// GetMember retrieves the member statistics for a specific guild and user.
// If the member does not exist, a new member is created and returned.
func GetMember(guildID, userID discordid.SnowflakeID) *Member {
	key := memberCacheKey{
		guildID:  guildID,
		memberID: userID,
	}

	if member, ok := memberCache.Get(key); ok {
		return copyMember(&member)
	}

	member := readMember(key.guildID, key.memberID)
	if member == nil {
		member = newMember(guildID, userID)
	}

	memberCache.Set(key, *member)
	return copyMember(member)
}

// newMember creates a new Member instance with default values and writes it to the database.
func newMember(guildID, userID discordid.SnowflakeID) *Member {
	member := &Member{
		GuildID:  guildID,
		MemberID: userID,
	}
	writeMember(member)

	return member
}

// RoundPlayed updates the member statistics based on the results of a played round.
func (m *Member) RoundPlayed(game *Game, player *bj.Player) {
	m.RoundsPlayed++
	m.HandsPlayed += len(player.Hands())
	for _, hand := range player.Hands() {
		switch game.EvaluateHand(hand) {
		case bj.PlayerWin, bj.PlayerBlackjack:
			m.Wins++
			m.CreditsWon += hand.Winnings() * game.config.PayoutPercent / 100
		case bj.DealerWin, bj.DealerBlackjack:
			m.Losses++
			m.CreditsLost += -hand.Winnings()
		case bj.Push:
			m.Pushes++
		}
		if hand.IsBlackjack() {
			m.Blackjacks++
		}
		if hand.IsSplit() {
			m.Splits++
		}
		if hand.IsSurrendered() {
			m.Surrenders++
		}
		m.CreditsBet += hand.Bet()
	}
	m.LastPlayed = time.Now()

	writeMember(m)
	memberCache.Set(memberCacheKey{
		guildID:  m.GuildID,
		memberID: m.MemberID,
	}, *m)
}
