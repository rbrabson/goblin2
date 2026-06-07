package shop

import (
	"errors"
	"fmt"
	"goblin2/bank"
	"goblin2/internal/discordid"
	"log/slog"
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	showBan = "shop"
)

// Member represents a member of a guild with restrictions on what they can or cannot do in a shop.
type Member struct {
	ID           bson.ObjectID         `json:"_id,omitempty" bson:"_id,omitempty"`
	GuildID      discordid.SnowflakeID `json:"guild_id,omitempty" bson:"guild_id,omitempty"`
	MemberID     discordid.SnowflakeID `json:"member_id,omitempty" bson:"member_id,omitempty"`
	Restrictions []string              `json:"restrictions,omitempty" bson:"restrictions,omitempty"`
	Version      int                   `json:"version" bson:"version"`
}

// GetMember retrieves a member from the database, creating one if it doesn't exist.
func GetMember(guildID, memberID discordid.SnowflakeID) *Member {
	key := memberCacheKey{
		guildID:  guildID,
		memberID: memberID,
	}

	if member, ok := memberCache.Get(key); ok {
		return copyMember(&member)
	}

	member, err := readMember(key.guildID, key.memberID)
	if err != nil {
		member = newMember(guildID, memberID)
		memberCache.Set(key, *member)
		return copyMember(member)
	}

	memberCache.Set(key, *member)
	return copyMember(member)
}

// newMember creates a new member with the given guild ID and member ID.
func newMember(guildID, memberID discordid.SnowflakeID) *Member {
	return &Member{
		GuildID:      guildID,
		MemberID:     memberID,
		Restrictions: []string{},
	}
}

// UpdateMember updates the shop member with the given mutation, retrying on version conflicts.
func UpdateMember(guildID, memberID discordid.SnowflakeID, mutate func(*Member) error) error {
	const maxRetries = 3

	memberMu.RLock()
	defer memberMu.RUnlock()

	key := memberCacheKey{
		guildID:  guildID,
		memberID: memberID,
	}

	for range maxRetries {
		member := GetMember(guildID, memberID)

		if err := mutate(member); err != nil {
			return err
		}

		var err error
		if len(member.Restrictions) == 0 && !member.ID.IsZero() {
			err = deleteMember(member)
		} else if member.ID.IsZero() {
			err = writeMember(member)
		} else {
			err = updateMember(member)
		}

		if err == nil {
			if len(member.Restrictions) == 0 {
				memberCache.Delete(key)
			} else {
				memberCache.Set(key, *member)
			}
			return nil
		}
		if !errors.Is(err, bank.ErrVersionConflict) {
			memberCache.Delete(key)
			return err
		}

		memberCache.Delete(key)

		slog.Warn("version conflict on shop member, retrying",
			slog.Any("guildID", guildID),
			slog.Any("memberID", memberID),
		)
	}

	return fmt.Errorf("failed to update shop member after %d retries: %w", maxRetries, bank.ErrVersionConflict)
}

// AddRestriction adds a restriction to the member.
func (m *Member) AddRestriction(restriction string) error {
	return UpdateMember(m.GuildID, m.MemberID, func(latest *Member) error {
		if latest.HasRestriction(restriction) {
			return fmt.Errorf("the user already has the `%s` restriction", restriction)
		}
		latest.Restrictions = append(latest.Restrictions, restriction)
		return nil
	})
}

// RemoveRestriction removes a restriction from the member.
func (m *Member) RemoveRestriction(restriction string) error {
	return UpdateMember(m.GuildID, m.MemberID, func(latest *Member) error {
		for i, r := range latest.Restrictions {
			if r == restriction {
				latest.Restrictions = append(latest.Restrictions[:i], latest.Restrictions[i+1:]...)
				return nil
			}
		}

		return fmt.Errorf("the user does not have the `%s` restriction", restriction)
	})
}

// HasRestriction checks if the member has a specific restriction.
func (m *Member) HasRestriction(restriction string) bool {
	return slices.Contains(m.Restrictions, restriction)
}
