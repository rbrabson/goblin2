package shop

import (
	"fmt"
	"goblin2/discordid"
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
}

// GetMember retrieves a member from the database, creating one if it doesn't exist.
func GetMember(guildID, memberID snowflake.ID) *Member {
	member, err := readMember(discordid.NewSnowflakeID(guildID), discordid.NewSnowflakeID(memberID))
	if err != nil {
		member = newMember(guildID, memberID)
	}
	return member
}

// getMember retrieves a member from the database.
func getMember(guildID, memberID snowflake.ID) (*Member, error) {
	return readMember(discordid.NewSnowflakeID(guildID), discordid.NewSnowflakeID(memberID))
}

// newMember creates a new member with the given guild ID and member ID.
func newMember(guildID, memberID snowflake.ID) *Member {
	// Don't write the member to the database, since it doesn't have any restrictions yet.
	return &Member{
		GuildID:      discordid.NewSnowflakeID(guildID),
		MemberID:     discordid.NewSnowflakeID(memberID),
		Restrictions: []string{},
	}
}

// AddRestriction adds a restriction to the member.
func (m *Member) AddRestriction(restriction string) error {
	if m.HasRestriction(restriction) {
		return fmt.Errorf("the user already has the `%s` restriction", restriction)
	}
	m.Restrictions = append(m.Restrictions, restriction)
	return writeMember(m)
}

// RemoveRestriction removes a restriction from the member.
func (m *Member) RemoveRestriction(restriction string) error {
	for i, r := range m.Restrictions {
		if r == restriction {
			m.Restrictions = append(m.Restrictions[:i], m.Restrictions[i+1:]...)

			if len(m.Restrictions) == 0 {
				return deleteMember(m)
			}
			return writeMember(m)
		}
	}

	return fmt.Errorf("the user does not have the `%s` restriction", restriction)
}

// ... existing code ...

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
