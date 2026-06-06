package shop

import (
	"errors"
	"fmt"
	"goblin2/bank"
	"goblin2/discordid"
	"log/slog"
	"slices"

	"github.com/disgoorg/snowflake/v2"
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
func GetMember(guildID, memberID snowflake.ID) *Member {
	key := memberCacheKey{
		guildID:  discordid.NewSnowflakeID(guildID),
		memberID: discordid.NewSnowflakeID(memberID),
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

// getMember retrieves a member from the database.
func getMember(guildID, memberID snowflake.ID) (*Member, error) {
	key := memberCacheKey{
		guildID:  discordid.NewSnowflakeID(guildID),
		memberID: discordid.NewSnowflakeID(memberID),
	}

	if member, ok := memberCache.Get(key); ok {
		return copyMember(&member), nil
	}

	member, err := readMember(key.guildID, key.memberID)
	if err != nil {
		return nil, err
	}

	memberCache.Set(key, *member)
	return copyMember(member), nil
}

// newMember creates a new member with the given guild ID and member ID.
func newMember(guildID, memberID snowflake.ID) *Member {
	return &Member{
		GuildID:      discordid.NewSnowflakeID(guildID),
		MemberID:     discordid.NewSnowflakeID(memberID),
		Restrictions: []string{},
	}
}

// UpdateMember updates the shop member with the given mutation, retrying on version conflicts.
func UpdateMember(guildID, memberID snowflake.ID, mutate func(*Member) error) error {
	const maxRetries = 3

	memberMu.RLock()
	defer memberMu.RUnlock()

	key := memberCacheKey{
		guildID:  discordid.NewSnowflakeID(guildID),
		memberID: discordid.NewSnowflakeID(memberID),
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
	return UpdateMember(m.GuildID.ID(), m.MemberID.ID(), func(latest *Member) error {
		if latest.HasRestriction(restriction) {
			return fmt.Errorf("the user already has the `%s` restriction", restriction)
		}
		latest.Restrictions = append(latest.Restrictions, restriction)
		return nil
	})
}

// RemoveRestriction removes a restriction from the member.
func (m *Member) RemoveRestriction(restriction string) error {
	return UpdateMember(m.GuildID.ID(), m.MemberID.ID(), func(latest *Member) error {
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

// GetRestrictedMembers retrieves all members with a specific restriction in a guild.
func GetRestrictedMembers(guildID, restriction string) ([]*Member, error) {
	allMembers, err := listMembers(guildID)
	if err != nil {
		return nil, err
	}

	restrictedMembers := make([]*Member, 0, len(allMembers))
	for _, member := range allMembers {
		if member.HasRestriction(restriction) {
			restrictedMembers = append(restrictedMembers, member)
		}
	}

	slices.SortFunc(restrictedMembers, func(a, b *Member) int {
		if a.MemberID < b.MemberID {
			return -1
		} else if a.MemberID > b.MemberID {
			return 1
		}
		return 0
	})

	return restrictedMembers, nil
}
