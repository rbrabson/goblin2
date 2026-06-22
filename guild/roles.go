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

	slog.Info("starting guild role sync")

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

// syncGuildRoles stores the current Discord roles and safely migrates any admin_roles entries
// that still use role names to role IDs. This function is intentionally non-destructive for
// admin_roles because startup role data can be incomplete or legacy data can decode differently.
func syncGuildRoles(guildID discordid.SnowflakeID, discordRoles []discord.Role) error {
	currentRoles := NewRoles(guildID, discordRoles)

	if err := writeRoles(currentRoles); err != nil {
		return err
	}

	if readGuild(guildID) == nil {
		g := &Guild{
			GuildID:    guildID,
			AdminRoles: defaultAdminRoleIDsFromRoles(currentRoles.Roles),
		}
		if err := writeGuild(g); err != nil {
			return err
		}
	}

	roleIDsByName := make(map[string]discordid.SnowflakeID, len(currentRoles.Roles))
	for _, role := range currentRoles.Roles {
		roleIDsByName[role.RoleName] = role.RoleID
	}

	rawAdminRoles, err := readGuildAdminRolesRaw(guildID)
	if err != nil {
		return err
	}
	if len(rawAdminRoles) == 0 {
		return nil
	}

	adminRoles := make([]discordid.SnowflakeID, 0, len(rawAdminRoles))
	seenAdminRoles := make(map[discordid.SnowflakeID]struct{}, len(rawAdminRoles))
	changed := false

	for _, rawAdminRole := range rawAdminRoles {
		roleID, convertedFromName, ok := adminRoleIDFromRaw(rawAdminRole, roleIDsByName)
		if !ok {
			slog.Warn("preserving unknown admin role value during role sync",
				slog.Any("guildID", guildID),
				slog.Any("adminRole", rawAdminRole),
			)
			continue
		}

		if convertedFromName {
			changed = true
		}

		if _, seen := seenAdminRoles[roleID]; seen {
			changed = true
			continue
		}

		adminRoles = append(adminRoles, roleID)
		seenAdminRoles[roleID] = struct{}{}
	}

	if !changed {
		return nil
	}

	if err := replaceGuildAdminRoles(guildID, adminRoles); err != nil {
		return err
	}

	return nil
}

// defaultAdminRoleIDsFromRoles returns role IDs for Discord roles whose names match the default admin role names.
func defaultAdminRoleIDsFromRoles(roles []Role) []discordid.SnowflakeID {
	defaultAdminRoleNameSet := make(map[string]struct{}, len(defaultAdminRoleNames))
	for _, roleName := range defaultAdminRoleNames {
		defaultAdminRoleNameSet[roleName] = struct{}{}
	}

	adminRoleIDs := make([]discordid.SnowflakeID, 0, len(defaultAdminRoleNames))
	seenAdminRoleIDs := make(map[discordid.SnowflakeID]struct{}, len(defaultAdminRoleNames))
	for _, role := range roles {
		if _, ok := defaultAdminRoleNameSet[role.RoleName]; !ok {
			continue
		}
		if _, seen := seenAdminRoleIDs[role.RoleID]; seen {
			continue
		}

		adminRoleIDs = append(adminRoleIDs, role.RoleID)
		seenAdminRoleIDs[role.RoleID] = struct{}{}
	}

	return adminRoleIDs
}

// adminRoleIDFromRaw converts a legacy admin role entry into a role ID.
// It returns convertedFromName=true only when a legacy role name was resolved to a current role ID.
func adminRoleIDFromRaw(rawAdminRole any, roleIDsByName map[string]discordid.SnowflakeID) (roleID discordid.SnowflakeID, convertedFromName bool, ok bool) {
	switch role := rawAdminRole.(type) {
	case discordid.SnowflakeID:
		return role, false, true
	case snowflake.ID:
		return discordid.NewSnowflakeID(role), false, true
	case int64:
		return discordid.NewSnowflakeID(snowflake.ID(role)), false, true
	case int32:
		return discordid.NewSnowflakeID(snowflake.ID(role)), false, true
	case int:
		return discordid.NewSnowflakeID(snowflake.ID(role)), false, true
	case string:
		if roleID, ok := roleIDsByName[role]; ok {
			return roleID, true, true
		}

		parsedRoleID, err := snowflake.Parse(role)
		if err != nil {
			return discordid.NewSnowflakeID(0), false, false
		}

		return discordid.NewSnowflakeID(parsedRoleID), false, true
	default:
		return discordid.NewSnowflakeID(0), false, false
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

// guildReadyListener syncs roles when a guild becomes ready after gateway startup.
func guildReadyListener(e *events.GuildReady) {
	syncGuildRolesFromDiscord(e.Client(), discordid.NewSnowflakeID(e.GuildID))
}

// guildJoinListener syncs roles when the bot joins a new guild.
func guildJoinListener(e *events.GuildJoin) {
	syncGuildRolesFromDiscord(e.Client(), discordid.NewSnowflakeID(e.GuildID))
}

// syncGuildRolesFromDiscord retrieves all roles for a guild from Discord and syncs them to MongoDB.
func syncGuildRolesFromDiscord(client *bot.Client, guildID discordid.SnowflakeID) {
	if client == nil {
		slog.Error("unable to sync guild roles: discord client is nil",
			slog.Any("guildID", guildID),
		)
		return
	}

	roles, err := client.Rest.GetRoles(guildID.ID())
	if err != nil {
		slog.Error("unable to retrieve guild roles during sync",
			slog.Any("guildID", guildID),
			slog.Any("error", err),
		)
		return
	}

	if err := syncGuildRoles(guildID, roles); err != nil {
		slog.Error("unable to sync guild roles",
			slog.Any("guildID", guildID),
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
