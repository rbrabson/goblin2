package heist

import (
	"fmt"
	"goblin2/discordid"
	"goblin2/guild"
	"log/slog"
	"time"

	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type CriminalLevel int

const (
	Greenhorn CriminalLevel = 0
	Renegade  CriminalLevel = 1
	Veteran   CriminalLevel = 10
	Commander CriminalLevel = 25
	WarChief  CriminalLevel = 50
	Legend    CriminalLevel = 75
	Immortal  CriminalLevel = 100
)

type MemberStatus string

const (
	Escaped     MemberStatus = "Escaped"
	Free        MemberStatus = "Free"
	Dead        MemberStatus = "Dead"
	Apprehended MemberStatus = "Apprehended"
	OOB         MemberStatus = "Out on Bail"
)

// HeistMember is the heist-specific state for a guild member who has
// participated in, or attempted to participate in, a heist.
type HeistMember struct {
	ID            bson.ObjectID         `json:"_id,omitempty" bson:"_id,omitempty"`
	GuildID       discordid.SnowflakeID `json:"guild_id" bson:"guild_id"`
	MemberID      discordid.SnowflakeID `json:"member_id" bson:"member_id"`
	BailCost      int                   `json:"bail_cost" bson:"bail_cost"`
	CriminalLevel CriminalLevel         `json:"criminal_level" bson:"criminal_level"`
	Deaths        int                   `json:"deaths" bson:"deaths"`
	DeathTimer    time.Time             `json:"death_timer" bson:"death_timer"`
	JailCounter   int                   `json:"jail_counter" bson:"jail_counter"`
	JailTimer     time.Time             `json:"jail_timer" bson:"jail_timer"`
	Sentence      time.Duration         `json:"sentence" bson:"sentence"`
	Spree         int                   `json:"spree" bson:"spree"`
	Status        MemberStatus          `json:"status" bson:"status"`
	TotalJail     int                   `json:"total_jail" bson:"total_jail"`

	heist       *Heist
	guildMember *guild.Member
}

// GetHeistMember returns the heist member for the given guild/member pair,
// creating a new in-memory member if one does not already exist.
//
// New members are not persisted until their state changes.
func GetHeistMember(guildID, memberID snowflake.ID) *HeistMember {
	member := readMember(discordid.NewSnowflakeID(guildID), discordid.NewSnowflakeID(memberID))
	if member != nil {
		member.UpdateStatus()
		return member
	}

	return NewHeistMember(guildID, memberID)
}

// NewHeistMember creates a new heist member with a default state.
func NewHeistMember(guildID snowflake.ID, memberID snowflake.ID) *HeistMember {
	return &HeistMember{
		GuildID:       discordid.NewSnowflakeID(guildID),
		MemberID:      discordid.NewSnowflakeID(memberID),
		CriminalLevel: Greenhorn,
		Status:        Free,
	}
}

// SetGuildMember attaches the resolved guild member data to this heist member.
// This is runtime-only data and is not persisted.
func (m *HeistMember) SetGuildMember(member *guild.Member) {
	m.guildMember = member
}

// UpdateStatus refreshes the member's status based on jail/death timers.
// If the member becomes free, the updated state is persisted.
func (m *HeistMember) UpdateStatus() {
	now := time.Now()

	switch m.Status {
	case Dead:
		if !m.DeathTimer.IsZero() && !m.DeathTimer.After(now) {
			m.Status = Free
			m.DeathTimer = time.Time{}
			writeMember(m)
		}
	case Apprehended, OOB:
		if !m.JailTimer.IsZero() && !m.JailTimer.After(now) {
			m.Status = Free
			m.JailTimer = time.Time{}
			m.Sentence = 0
			m.BailCost = 0
			writeMember(m)
		}
	}
}

// RemainingJailTime returns the time remaining before the member is released
// from jail. If the member is not jailed, or the timer has expired, zero is
// returned.
func (m *HeistMember) RemainingJailTime() time.Duration {
	if m.JailTimer.IsZero() {
		return 0
	}

	remaining := time.Until(m.JailTimer)
	if remaining <= 0 {
		return 0
	}

	return remaining
}

// RemainingDeathTime returns the time remaining before the member returns from
// death. If the member is not dead, or the timer has expired, zero is returned.
func (m *HeistMember) RemainingDeathTime() time.Duration {
	if m.DeathTimer.IsZero() {
		return 0
	}

	remaining := time.Until(m.DeathTimer)
	if remaining <= 0 {
		return 0
	}

	return remaining
}

// SendToJail marks the member as apprehended for the given sentence duration.
func (m *HeistMember) SendToJail(sentence time.Duration, bailCost int) {
	now := time.Now()

	m.Status = Apprehended
	m.JailCounter++
	m.TotalJail++
	m.Sentence = sentence
	m.JailTimer = now.Add(sentence)
	m.BailCost = bailCost
	m.Spree = 0

	writeMember(m)

	slog.Debug("heist member sent to jail",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
		slog.Duration("sentence", sentence),
		slog.Int("bailCost", bailCost),
	)
}

// ReleaseOnBail marks the member as out on bail.
func (m *HeistMember) ReleaseOnBail() {
	m.Status = OOB
	m.JailTimer = time.Time{}
	m.Sentence = 0
	m.BailCost = 0

	writeMember(m)

	slog.Debug("heist member released on bail",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
	)
}

// Kill marks the member as dead for the given duration.
func (m *HeistMember) Kill(duration time.Duration) {
	m.Status = Dead
	m.Deaths++
	m.DeathTimer = time.Now().Add(duration)
	m.Spree = 0

	writeMember(m)

	slog.Debug("heist member killed",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
		slog.Duration("duration", duration),
	)
}

// MarkEscaped records that the member escaped successfully.
func (m *HeistMember) MarkEscaped() {
	m.Status = Free
	m.Spree++
	m.CriminalLevel = calculateCriminalLevel(m.Spree)

	writeMember(m)

	slog.Debug("heist member escaped",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
		slog.Int("spree", m.Spree),
		slog.Any("criminalLevel", m.CriminalLevel),
	)
}

// FreeMember clears the jail / death state and marks the member as free.
func (m *HeistMember) FreeMember() {
	m.Status = Free
	m.BailCost = 0
	m.DeathTimer = time.Time{}
	m.JailTimer = time.Time{}
	m.Sentence = 0

	writeMember(m)

	slog.Debug("heist member freed",
		slog.Any("guildID", m.GuildID),
		slog.Any("memberID", m.MemberID),
	)
}

func calculateCriminalLevel(spree int) CriminalLevel {
	switch {
	case spree >= int(Immortal):
		return Immortal
	case spree >= int(Legend):
		return Legend
	case spree >= int(WarChief):
		return WarChief
	case spree >= int(Commander):
		return Commander
	case spree >= int(Veteran):
		return Veteran
	case spree >= int(Renegade):
		return Renegade
	default:
		return Greenhorn
	}
}

// String returns a string representation of the HeistMember.
func (m *HeistMember) String() string {
	return fmt.Sprintf(
		"HeistMember{ID=%s, GuildID=%s, MemberID=%s, Status=%s, CriminalLevel=%d, Spree=%d, Deaths=%d, JailCounter=%d}",
		m.ID.Hex(),
		m.GuildID,
		m.MemberID,
		m.Status,
		m.CriminalLevel,
		m.Spree,
		m.Deaths,
		m.JailCounter,
	)
}
