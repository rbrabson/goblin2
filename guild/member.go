package guild

import (
	"errors"
	"fmt"
	"goblin2/internal/cache"
	"goblin2/internal/discordid"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
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

// Member represents a Discord guild member
type Member struct {
	ID         bson.ObjectID         `bson:"_id,omitempty"`
	GuildID    discordid.SnowflakeID `bson:"guild_id"`
	MemberID   discordid.SnowflakeID `bson:"member_id"`
	UserName   string                `bson:"username"`
	GlobalName string                `bson:"global_name"`
	NickName   string                `bson:"nickname"`
	Name       string                `bson:"name"`
	Version    int                   `bson:"version"`
}

// GetMember returns the member within a guild.
func GetMember(guildID discordid.SnowflakeID, member *discord.Member) *Member {
	if member == nil {
		slog.Error("discord member is nil", slog.Any("guild_id", guildID))
		return nil
	}

	key := memberCacheKey{
		guildID:  guildID,
		memberID: discordid.NewSnowflakeID(member.User.ID),
	}

	if cached, ok := memberCache.Get(key); ok {
		m := copyMember(&cached)
		m.Update(member)
		return m
	}

	m := readMember(key.guildID, key.memberID)
	if m == nil {
		m = createNewMember(guildID, member)
	}

	m.Update(member)
	memberCache.Set(key, *m)

	return copyMember(m)
}

// GetMemberByID returns the member within a guild by ID.
func GetMemberByID(guildID, memberID discordid.SnowflakeID) (*Member, error) {
	key := memberCacheKey{
		guildID:  guildID,
		memberID: memberID,
	}

	if cached, ok := memberCache.Get(key); ok {
		return copyMember(&cached), nil
	}

	m := readMember(key.guildID, key.memberID)
	if m == nil {
		return nil, ErrMemberNotFound
	}

	memberCache.Set(key, *m)
	return copyMember(m), nil
}

// createNewMember creates a new member within a guild.
func createNewMember(guildID discordid.SnowflakeID, member *discord.Member) *Member {
	m := &Member{
		GuildID:  guildID,
		MemberID: discordid.NewSnowflakeID(member.User.ID),
	}
	return m
}

// copyMember returns a copy of the given member. This prevents callers from
// mutating the cached member directly.
func copyMember(member *Member) *Member {
	if member == nil {
		return nil
	}

	return new(*member)
}

// CloseMemberCache stops the member cache cleanup goroutine and clears all
// cached member entries.
func CloseMemberCache() {
	memberCache.Destroy()
}

// Update updates the member with the given member information. Returns true if the member was updated.
func (m *Member) Update(member *discord.Member) bool {
	if member == nil {
		slog.Error("discord member is nil")
		return false
	}

	changed := false
	if err := UpdateMember(discordid.NewSnowflakeID(member.GuildID), discordid.NewSnowflakeID(member.User.ID), member, func(latest *Member) error {
		changed = updateMemberFields(latest, member)
		return nil
	}); err != nil {
		slog.Error("unable to persist guild member",
			slog.Any("guildID", m.GuildID),
			slog.Any("memberID", m.MemberID),
			slog.Any("error", err),
		)
		return false
	}

	key := memberCacheKey{
		guildID:  discordid.NewSnowflakeID(member.GuildID),
		memberID: discordid.NewSnowflakeID(member.User.ID),
	}
	if cached, ok := memberCache.Get(key); ok {
		*m = cached
	}

	return changed
}

func updateMemberFields(m *Member, member *discord.Member) bool {
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

	if m.GuildID.ID() == guildID &&
		m.MemberID.ID() == memberID &&
		m.UserName == username &&
		m.GlobalName == globalName &&
		m.NickName == nickname &&
		m.Name == name {
		return false
	}

	m.GuildID = discordid.NewSnowflakeID(guildID)
	m.MemberID = discordid.NewSnowflakeID(memberID)
	m.UserName = username
	m.GlobalName = globalName
	m.NickName = nickname
	m.Name = name

	return true
}

// UpdateMember applies the given mutation to the member, retrying on version conflicts.
func UpdateMember(guildID, memberID discordid.SnowflakeID, discordMember *discord.Member, mutate func(*Member) error) error {
	const maxRetries = 3

	memberMu.RLock()
	defer memberMu.RUnlock()

	key := memberCacheKey{
		guildID:  guildID,
		memberID: memberID,
	}

	for range maxRetries {
		member, err := GetMemberByID(guildID, memberID)
		if err != nil {
			if !errors.Is(err, ErrMemberNotFound) || discordMember == nil {
				return err
			}
			member = createNewMember(guildID, discordMember)
		}

		if err := mutate(member); err != nil {
			return err
		}

		var writeErr error
		if member.ID.IsZero() {
			writeErr = writeMember(member)
		} else {
			writeErr = updateMember(member)
		}

		if writeErr == nil {
			memberCache.Set(key, *member)
			return nil
		}
		if !errors.Is(writeErr, ErrVersionConflict) {
			memberCache.Delete(key)
			return writeErr
		}

		memberCache.Delete(key)

		slog.Warn("version conflict on guild member, retrying",
			slog.Any("guildID", guildID),
			slog.Any("memberID", memberID),
		)
	}

	return fmt.Errorf("failed to update guild member after %d retries: %w", maxRetries, ErrVersionConflict)
}

// GetRoles returns the list of roles for member in a given guild.
func (m *Member) GetRoles(client *bot.Client) ([]discord.Role, error) {
	member, ok := client.Caches.Member(m.GuildID.ID(), m.MemberID.ID())
	if !ok {
		return nil, fmt.Errorf("member %s not found in guild %s", m.MemberID, m.GuildID)
	}

	roleIDs := make(map[discordid.SnowflakeID]struct{}, len(member.RoleIDs))
	for _, roleID := range member.RoleIDs {
		roleIDs[discordid.NewSnowflakeID(roleID)] = struct{}{}
	}

	roles := make([]discord.Role, 0, len(roleIDs))
	for role := range client.Caches.Roles(m.GuildID.ID()) {
		if _, ok := roleIDs[discordid.NewSnowflakeID(role.ID)]; ok {
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
