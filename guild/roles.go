package guild

import (
	"goblin2/internal/discordid"
	"log/slog"
	"slices"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Role is a role in a guild.
type Role struct {
	RoleID   discordid.SnowflakeID `bson:"role_id"`
	RoleName string                `bson:"role_name"`
	Position int                   `bson:"position"`
}

// Roles is a list of all roles in a guild.
type Roles struct {
	ID       bson.ObjectID         `bson:"_id,omitempty"`
	GuildID  discordid.SnowflakeID `bson:"guild_id"`
	Roles    []Role                `bson:"roles"`
	SyncedAt time.Time             `bson:"synced_at"`
	Version  int                   `bson:"version"`
}

// NewRole creates a role record from a Discord role.
func NewRole(role discord.Role) Role {
	return Role{
		RoleID:   discordid.NewSnowflakeID(role.ID),
		RoleName: role.Name,
		Position: role.Position,
	}
}

// NewRoles creates a sorted guild roles document from Discord roles.
func NewRoles(guildID discordid.SnowflakeID, discordRoles []discord.Role) *Roles {
	roles := make([]Role, 0, len(discordRoles))
	for _, role := range discordRoles {
		roles = append(roles, NewRole(role))
	}

	sortRolesByPosition(roles)

	return &Roles{
		GuildID:  guildID,
		Roles:    roles,
		SyncedAt: time.Now().UTC(),
	}
}

// RoleNames returns the role names in stored position order.
func (r *Roles) RoleNames() []string {
	if r == nil {
		return nil
	}

	roles := append([]Role(nil), r.Roles...)
	sortRolesByPosition(roles)

	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.RoleName)
	}

	return names
}

func sortRolesByPosition(roles []Role) {
	slices.SortFunc(roles, func(a Role, b Role) int {
		if a.Position < b.Position {
			return -1
		}
		if a.Position > b.Position {
			return 1
		}
		return 0
	})
}

func guildRoleCreateListener(e *events.RoleCreate) {
	guildID := discordid.NewSnowflakeID(e.GuildID)

	if err := upsertRole(guildID, NewRole(e.Role)); err != nil {
		e.Client().Logger.Error("unable to persist created guild role",
			slog.Any("guildID", guildID),
			slog.Any("roleID", e.Role.ID),
			slog.Any("error", err),
		)
	}
}

func guildRoleUpdateListener(e *events.RoleUpdate) {
	guildID := discordid.NewSnowflakeID(e.GuildID)

	if err := upsertRole(guildID, NewRole(e.Role)); err != nil {
		e.Client().Logger.Error("unable to persist updated guild role",
			slog.Any("guildID", guildID),
			slog.Any("roleID", e.Role.ID),
			slog.Any("error", err),
		)
	}
}

func guildRoleDeleteListener(e *events.RoleDelete) {
	guildID := discordid.NewSnowflakeID(e.GuildID)
	roleID := discordid.NewSnowflakeID(e.RoleID)

	if err := deleteRole(guildID, roleID); err != nil {
		e.Client().Logger.Error("unable to remove deleted guild role",
			slog.Any("guildID", guildID),
			slog.Any("roleID", roleID),
			slog.Any("error", err),
		)
	}
}
