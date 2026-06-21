package guild

import (
	"goblin2/internal/discordid"
	"log/slog"
	"slices"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
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

// sortRolesByPosition sorts the roles by position.
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

// startRoleSync starts a goroutine that syncs all cached guild roles from Discord on startup.
func startRoleSync(client *bot.Client) {
	if client == nil {
		slog.Error("unable to start guild role sync: discord client is nil")
		return
	}

	go func() {
		for guild := range client.Caches.Guilds() {
			guildID := discordid.NewSnowflakeID(guild.ID)

			roles, err := client.Rest.GetRoles(guild.ID)
			if err != nil {
				slog.Error("unable to retrieve guild roles during startup sync",
					slog.Any("guildID", guildID),
					slog.Any("error", err),
				)
				continue
			}

			if err := syncGuildRoles(guildID, roles); err != nil {
				slog.Error("unable to sync guild roles during startup",
					slog.Any("guildID", guildID),
					slog.Any("error", err),
				)
			}
		}
	}()
}

// syncGuildRoles stores the current Discord roles, removes deleted roles from admin_roles,
// and migrates any admin_roles entries that still use role names to role IDs.
func syncGuildRoles(guildID discordid.SnowflakeID, discordRoles []discord.Role) error {
	currentRoles := NewRoles(guildID, discordRoles)

	if err := writeRoles(currentRoles); err != nil {
		return err
	}

	currentRoleIDs := make(map[discordid.SnowflakeID]struct{}, len(currentRoles.Roles))
	currentRoleIDsByName := make(map[string]discordid.SnowflakeID, len(currentRoles.Roles))
	for _, role := range currentRoles.Roles {
		currentRoleIDs[role.RoleID] = struct{}{}
		currentRoleIDsByName[role.RoleName] = role.RoleID
	}

	rawAdminRoles, err := readGuildAdminRolesRaw(guildID)
	if err != nil {
		slog.Error("unable to read admin roles",
			slog.Any("guildID", guildID),
			slog.Any("error", err),
		)
		return err
	}

	adminRoles := make([]discordid.SnowflakeID, 0, len(rawAdminRoles))
	seenAdminRoles := make(map[discordid.SnowflakeID]struct{}, len(rawAdminRoles))

	for _, rawAdminRole := range rawAdminRoles {
		roleID, ok := adminRoleIDFromRaw(rawAdminRole, currentRoleIDsByName)
		if !ok {
			continue
		}

		if _, exists := currentRoleIDs[roleID]; !exists {
			continue
		}

		if _, seen := seenAdminRoles[roleID]; seen {
			continue
		}

		adminRoles = append(adminRoles, roleID)
		seenAdminRoles[roleID] = struct{}{}
	}

	if err := replaceGuildAdminRoles(guildID, adminRoles); err != nil {
		slog.Error("unable to replace admin roles",
			slog.Any("guildID", guildID),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// adminRoleIDFromRaw converts a legacy admin role entry into a role ID.
// Existing role IDs are kept, and legacy role names are resolved against current Discord roles.
func adminRoleIDFromRaw(rawAdminRole any, roleIDsByName map[string]discordid.SnowflakeID) (discordid.SnowflakeID, bool) {
	switch role := rawAdminRole.(type) {
	case discordid.SnowflakeID:
		return role, true
	case snowflake.ID:
		return discordid.NewSnowflakeID(role), true
	case string:
		if roleID, ok := roleIDsByName[role]; ok {
			return roleID, true
		}

		parsedRoleID, err := snowflake.Parse(role)
		if err != nil {
			return discordid.NewSnowflakeID(0), false
		}

		return discordid.NewSnowflakeID(parsedRoleID), true
	default:
		return discordid.NewSnowflakeID(0), false
	}
}

// guildRoleCreateListener is called when a guild role is created.
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

// guildRoleUpdateListener is called when a guild role is updated.
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

// guildRoleDeleteListener is called when a guild role is deleted.
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
