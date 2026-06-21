package guild

import (
	"goblin2/database"
	"goblin2/internal/discordid"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	guildCollection  = "guilds"
	memberCollection = "guild_members"
	rolesCollection  = "guild_roles"
)

var (
	db *database.MongoDB
)

// readGuild reads a guild from the database, returning nil if not found.
func readGuild(guildID discordid.SnowflakeID) *Guild {
	filter := bson.M{"guild_id": guildID}
	var g Guild
	if err := db.FindOne(guildCollection, filter, &g); err != nil {
		slog.Debug("guild not found in database",
			slog.Any("guildID", guildID),
			slog.Any("error", err),
		)
		return nil
	}
	return &g
}

// writeGuild inserts a new guild into the database.
func writeGuild(g *Guild) error {
	g.Version = 0

	result, err := db.InsertOne(guildCollection, g)
	if err != nil {
		slog.Error("unable to create guild",
			slog.Any("guildID", g.GuildID),
			slog.Any("error", err),
		)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		g.ID = id
	}

	return nil
}

// updateGuild updates an existing guild using optimistic locking via the version field.
// Returns ErrVersionConflict if another writer updated the guild since it was read.
func updateGuild(g *Guild) error {
	filter := bson.M{
		"guild_id": g.GuildID,
	}

	if g.Version == 0 {
		filter["$or"] = bson.A{
			bson.M{"version": 0},
			bson.M{"version": bson.M{"$exists": false}},
		}
	} else {
		filter["version"] = g.Version
	}

	update := bson.M{
		"$set": bson.M{
			"admin_roles": g.AdminRoles,
		},
		"$inc": bson.M{
			"version": 1,
		},
	}

	result, err := db.UpdateOne(guildCollection, filter, update)
	if err != nil {
		slog.Error("unable to update guild",
			slog.Any("guildID", g.GuildID),
			slog.Any("version", g.Version),
			slog.Any("error", err),
		)
		return err
	}
	if result.MatchedCount == 0 {
		return ErrVersionConflict
	}

	g.Version++

	return nil
}

// readRoles reads the stored roles for a guild, returning nil if not found.
func readRoles(guildID discordid.SnowflakeID) *Roles {
	filter := bson.M{"guild_id": guildID}

	var roles Roles
	if err := db.FindOne(rolesCollection, filter, &roles); err != nil {
		slog.Debug("guild roles not found in database",
			slog.Any("guildID", guildID),
			slog.Any("error", err),
		)
		return nil
	}

	sortRolesByPosition(roles.Roles)

	return &roles
}

// writeRoles upserts the roles for a guild.
func writeRoles(roles *Roles) error {
	sortRolesByPosition(roles.Roles)
	roles.SyncedAt = time.Now().UTC()

	filter := bson.M{"guild_id": roles.GuildID}
	update := bson.M{
		"$set": bson.M{
			"guild_id":  roles.GuildID,
			"roles":     roles.Roles,
			"synced_at": roles.SyncedAt,
		},
		"$inc": bson.M{
			"version": 1,
		},
	}

	if _, err := db.UpdateOneUpsert(rolesCollection, filter, update); err != nil {
		slog.Error("unable to write guild roles",
			slog.Any("guildID", roles.GuildID),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// upsertRole creates or updates a role in guild_roles.
func upsertRole(guildID discordid.SnowflakeID, role Role) error {
	roles := readRoles(guildID)
	if roles == nil {
		roles = &Roles{
			GuildID: guildID,
			Roles:   make([]Role, 0, 1),
		}
	}

	updated := false
	for i := range roles.Roles {
		if roles.Roles[i].RoleID == role.RoleID {
			roles.Roles[i] = role
			updated = true
			break
		}
	}

	if !updated {
		roles.Roles = append(roles.Roles, role)
	}

	return writeRoles(roles)
}

// deleteRole removes a role from guild_roles and from guilds.admin_roles.
func deleteRole(guildID, roleID discordid.SnowflakeID) error {
	roles := readRoles(guildID)
	if roles != nil {
		remaining := make([]Role, 0, len(roles.Roles))
		for _, role := range roles.Roles {
			if role.RoleID != roleID {
				remaining = append(remaining, role)
			}
		}

		roles.Roles = remaining
		if err := writeRoles(roles); err != nil {
			return err
		}
	}

	if err := removeDeletedAdminRole(guildID, roleID); err != nil {
		return err
	}

	return nil
}

// removeDeletedAdminRole removes a deleted role ID from guilds.admin_roles.
func removeDeletedAdminRole(guildID, roleID discordid.SnowflakeID) error {
	update := bson.M{
		"$pull": bson.M{
			"admin_roles": roleID.String(),
		},
	}

	if _, err := db.UpdateOne(guildCollection, bson.M{"guild_id": guildID}, update); err != nil {
		slog.Error("unable to remove deleted role from guild admin roles",
			slog.Any("guildID", guildID),
			slog.Any("roleID", roleID),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// readMember reads a member from the database, returning nil if not found.
func readMember(guildID discordid.SnowflakeID, memberID discordid.SnowflakeID) *Member {
	filter := bson.M{
		"guild_id":  guildID,
		"member_id": memberID,
	}

	var m Member
	if err := db.FindOne(memberCollection, filter, &m); err != nil {
		slog.Debug("member not found in database",
			slog.Any("guildID", guildID),
			slog.Any("memberID", memberID),
			slog.Any("filter", filter),
			slog.Any("error", err),
		)
		return nil
	}

	return &m
}

// writeMember inserts a new member into the database.
func writeMember(m *Member) error {
	m.Version = 0

	result, err := db.InsertOne(memberCollection, m)
	if err != nil {
		slog.Error("unable to create member",
			slog.Any("guildID", m.GuildID),
			slog.Any("memberID", m.MemberID),
			slog.Any("error", err),
		)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		m.ID = id
	}

	return nil
}

// updateMember updates an existing member in the database.
func updateMember(m *Member) error {
	filter := bson.M{
		"guild_id":  m.GuildID,
		"member_id": m.MemberID,
	}

	if m.Version == 0 {
		filter["$or"] = bson.A{
			bson.M{"version": 0},
			bson.M{"version": bson.M{"$exists": false}},
		}
	} else {
		filter["version"] = m.Version
	}

	update := bson.M{
		"$set": bson.M{
			"username":    m.UserName,
			"global_name": m.GlobalName,
			"nickname":    m.NickName,
			"name":        m.Name,
		},
		"$inc": bson.M{
			"version": 1,
		},
	}

	result, err := db.UpdateOne(memberCollection, filter, update)
	if err != nil {
		slog.Error("unable to update member",
			slog.Any("guildID", m.GuildID),
			slog.Any("memberID", m.MemberID),
			slog.Any("version", m.Version),
			slog.Any("error", err),
		)
		return err
	}
	if result.MatchedCount == 0 {
		return ErrVersionConflict
	}

	m.Version++

	return nil
}
