package slots

import (
	"goblin2/internal/cache"
	"goblin2/internal/discordid"
	"goblin2/stats"
	"sync"
	"time"

	rslots "github.com/rbrabson/slots"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	memberCacheTTL             = 30 * time.Minute
	memberCacheCleanupInterval = 5 * time.Minute
)

type memberCacheKey struct {
	guildID  discordid.SnowflakeID
	memberID discordid.SnowflakeID
}

var (
	memberCache = cache.New[memberCacheKey, Member](memberCacheTTL, memberCacheCleanupInterval)
	memberMu    sync.RWMutex
)

// Member represents a member's statistics for the slots game.
type Member struct {
	ID                  bson.ObjectID         `bson:"_id,omitempty"`
	GuildID             discordid.SnowflakeID `bson:"guild_id"`
	MemberID            discordid.SnowflakeID `bson:"member_id"`
	CurrentWinStreak    int                   `bson:"current_win_streak"`
	LongestWinStreak    int                   `bson:"longest_win_streak"`
	CurrentLosingStreak int                   `bson:"current_losing_streak"`
	LongestLosingStreak int                   `bson:"longest_losing_streak"`
	TotalWins           int                   `bson:"total_wins"`
	TotalLosses         int                   `bson:"total_losses"`
	TotalBet            int                   `bson:"total_bet"`
	TotalWinnings       int                   `bson:"total_winnings"`
	MaxWin              int                   `bson:"max_win"`
	LastPlayed          time.Time             `bson:"last_played"`
}

// GetMember retrieves the member statistics for a specific guild and user.
// If the member does not exist, a new member is created and returned.
func GetMember(guildID, userID discordid.SnowflakeID) *Member {
	key := memberCacheKey{
		guildID:  guildID,
		memberID: userID,
	}

	if cached, ok := memberCache.Get(key); ok {
		return copyMember(&cached)
	}

	memberMu.Lock()
	defer memberMu.Unlock()

	if cached, ok := memberCache.Get(key); ok {
		return copyMember(&cached)
	}

	member := readMember(guildID, userID)
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

// copyMember returns a copy of the given member.
// This prevents callers from mutating cached member data directly.
func copyMember(member *Member) *Member {
	if member == nil {
		return nil
	}

	return new(*member)
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

// IsInCooldown checks if the member is in cooldown. If not, it updates the LastPlayed time and returns false.
// If the member is in cooldown, it returns true.
func (m *Member) IsInCooldown(config *Config) bool {
	if time.Since(m.LastPlayed) < config.Cooldown {
		return true
	}

	m.LastPlayed = time.Now()
	m.save()

	return false
}

// GetCooldownRemaining returns the remaining cooldown time for the member.
func (m *Member) GetCooldownRemaining(config *Config) time.Duration {
	remaining := config.Cooldown - time.Since(m.LastPlayed)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// AddResults updates the member's statistics based on the results of a spin.
func (m *Member) AddResults(spinResult *rslots.SpinResult) {
	m.TotalBet += spinResult.Bet
	if spinResult.Payout > 0 {
		m.TotalWinnings += spinResult.Payout
		m.TotalWins++
		m.CurrentWinStreak++
		m.LongestWinStreak = max(m.LongestWinStreak, m.CurrentWinStreak)
		m.CurrentLosingStreak = 0
		m.MaxWin = max(m.MaxWin, spinResult.Payout)
	} else {
		m.TotalLosses++
		m.CurrentLosingStreak++
		m.LongestLosingStreak = max(m.LongestLosingStreak, m.CurrentLosingStreak)
		m.CurrentWinStreak = 0
	}

	m.save()

	memberIDs := []discordid.SnowflakeID{m.MemberID}
	stats.UpdateGameStats(m.GuildID, "slots", memberIDs)
}

// CloseMemberCache stops the member cache cleanup goroutine and clears all cached member entries.
func CloseMemberCache() {
	memberCache.Destroy()
}
