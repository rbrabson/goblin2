package guild

import (
	"fmt"
	"goblin2/discordid"
	"log/slog"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Member represents a Discord guild member
type Member struct {
	ID         bson.ObjectID         `bson:"_id,omitempty"`
	GuildID    discordid.SnowflakeID `bson:"guild_id"`
	MemberID   discordid.SnowflakeID `bson:"member_id"`
	UserName   string                `bson:"username"`
	GlobalName string                `bson:"global_name"`
	NickName   string                `bson:"nickname"`
	Name       string                `bson:"name"`
}

// GetMember returns the member within a guild.
func GetMember(guildID snowflake.ID, member *discord.Member) *Member {
	if member == nil {
		slog.Error("discord member is nil", slog.Any("guild_id", guildID))
		return nil
	}

	m := readMember(discordid.NewSnowflakeID(guildID), discordid.NewSnowflakeID(member.User.ID))
	if m == nil {
		m = createNewMember(guildID, member)
	}

	m.Update(member)
	return m
}

// GetMemberByID returns the member within a guild by ID.
func GetMemberByID(guildID snowflake.ID, memberID snowflake.ID) (*Member, error) {
	m := readMember(discordid.NewSnowflakeID(guildID), discordid.NewSnowflakeID(memberID))
	if m == nil {
		return nil, ErrMemberNotFound
	}
	return m, nil
}

// createNewMember creates a new member within a guild.
func createNewMember(guildID snowflake.ID, member *discord.Member) *Member {
	m := &Member{
		GuildID:  discordid.NewSnowflakeID(guildID),
		MemberID: discordid.NewSnowflakeID(member.User.ID),
	}
	return m
}

// Update updates the member with the given member information. Returns true if the member was updated.
func (m *Member) Update(member *discord.Member) bool {
	if member == nil {
		slog.Error("discord member is nil")
		return false
	}
	guildID := member.GuildID
	memberID := member.User.ID
	var globalName string
	if member.User.GlobalName != nil {
		globalName = *member.User.GlobalName
	}
	username := member.User.Username
	var nickname string
	if member.Nick != nil {
		nickname = *member.Nick
	}
	name := member.EffectiveName()

	if m.GuildID.ID() != guildID || m.MemberID.ID() != memberID || m.UserName != username || m.GlobalName != globalName || m.NickName != nickname || m.Name != name {
		m.GuildID = discordid.NewSnowflakeID(guildID)
		m.MemberID = discordid.NewSnowflakeID(memberID)
		m.UserName = username
		m.GlobalName = globalName
		m.NickName = nickname
		m.Name = name

		var err error
		if m.ID.IsZero() {
			err = writeMember(m)
		} else {
			err = updateMember(m)
		}
		if err != nil {
			slog.Error("unable to persist guild member",
				slog.Any("guildID", m.GuildID),
				slog.Any("memberID", m.MemberID),
				slog.Any("error", err),
			)
		}

		return true
	}
	return false
}

// GetRoles returns the list of roles for member in a given guild.
func (m *Member) GetRoles(client *bot.Client) ([]discord.Role, error) {
	member, ok := client.Caches.Member(m.GuildID.ID(), m.MemberID.ID())
	if !ok {
		return nil, fmt.Errorf("member %s not found in guild %s", m.MemberID, m.GuildID)
	}

	roleIDs := make(map[snowflake.ID]struct{}, len(member.RoleIDs))
	for _, roleID := range member.RoleIDs {
		roleIDs[roleID] = struct{}{}
	}

	roles := make([]discord.Role, 0, len(roleIDs))
	for role := range client.Caches.Roles(m.GuildID.ID()) {
		if _, ok := roleIDs[role.ID]; ok {
			roles = append(roles, role)
		}
	}

	return roles, nil
}

// HasRole checks if a member has a role with the given name in a given guild.
func (m *Member) HasRole(client *bot.Client, roleName string) (bool, error) {
	roles, err := m.GetRoles(client)
	if err != nil {
		return false, err
	}

	for _, role := range roles {
		if role.Name == roleName {
			return true, nil
		}
	}
	return false, nil
}

// IsAdmin checks if the member has any of the guild's configured admin roles.
func (m *Member) IsAdmin(client *bot.Client, guild *Guild) (bool, error) {
	if guild == nil {
		return false, nil
	}

	adminRoles := make(map[string]struct{}, len(guild.AdminRoles))
	for _, roleName := range guild.AdminRoles {
		adminRoles[roleName] = struct{}{}
	}

	roles, err := m.GetRoles(client)
	if err != nil {
		return false, err
	}

	for _, role := range roles {
		if _, ok := adminRoles[role.Name]; ok {
			return true, nil
		}
	}

	return false, nil
}

// AssignRole assigns the role with the given name to the member.
func (m *Member) AssignRole(client *bot.Client, roleName string) error {
	role, err := m.getGuildRole(client, roleName)
	if err != nil {
		return err
	}

	return client.Rest.AddMemberRole(m.GuildID.ID(), m.MemberID.ID(), role.ID)
}

// UnassignRole removes the role with the given name from the member.
func (m *Member) UnassignRole(client *bot.Client, roleName string) error {
	role, err := m.getGuildRole(client, roleName)
	if err != nil {
		return err
	}

	return client.Rest.RemoveMemberRole(m.GuildID.ID(), m.MemberID.ID(), role.ID)
}

// getGuildRole returns the role with the given name from the guild.
func (m *Member) getGuildRole(client *bot.Client, roleName string) (discord.Role, error) {
	guild := Guild{GuildID: m.GuildID}
	return guild.GetRole(client, roleName)
}

// String returns a string representation of the Member.
func (m *Member) String() string {
	return fmt.Sprintf("Member{ID=%s, GuildID=%v, MemberID=%v, Name=%s}",
		m.ID.Hex(),
		m.GuildID,
		m.MemberID,
		m.Name)
}
