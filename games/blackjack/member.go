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
	ID           bson.ObjectID         `bson:"_id,omitempty"`
	GuildID      discordid.SnowflakeID `bson:"guild_id"`
	MemberID     discordid.SnowflakeID `bson:"member_id"`
	RoundsPlayed int                   `bson:"rounds_played"`
	HandsPlayed  int                   `bson:"hands_played"`
	Wins         int                   `bson:"wins"`
	Losses       int                   `bson:"losses"`
	Pushes       int                   `bson:"pushes"`
	Blackjacks   int                   `bson:"blackjacks"`
	Splits       int                   `bson:"splits"`
	Surrenders   int                   `bson:"surrenders"`
	CreditsBet   int                   `bson:"credits_bet"`
	CreditsWon   int                   `bson:"credits_won"`
	CreditsLost  int                   `bson:"credits_lost"`
	LastPlayed   time.Time             `bson:"last_played"`
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

	memberLoadMu.Lock()
	defer memberLoadMu.Unlock()

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

// cacheKey returns the cache key for this member.
func (m *Member) cacheKey() memberCacheKey {
	return memberCacheKey{
		guildID:  m.GuildID,
		memberID: m.MemberID,
	}
}

// save persists the member and updates the cache.
func (m *Member) save() {
	writeMember(m)
	memberCache.Set(m.cacheKey(), *m)
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

	m.save()
}
