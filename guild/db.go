package guild

import (
	"goblin2/database"
	"goblin2/internal/discordid"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	guildCollection  = "guilds"
	memberCollection = "guild_members"
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
	_, err := db.InsertOne(guildCollection, g)
	if err != nil {
		slog.Error("unable to create guild",
			slog.Any("guildID", g.GuildID),
			slog.Any("error", err),
		)
	}
	return err
}

// updateGuild updates an existing guild using optimistic locking via the version field.
// Returns ErrVersionConflict if another writer updated the guild since it was read.
func updateGuild(g *Guild) error {
	filter := bson.M{
		"guild_id": g.GuildID,
		"version":  g.Version,
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
			slog.Any("error", err),
		)
		return err
	}
	if result.MatchedCount == 0 {
		return ErrVersionConflict
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
	_, err := db.InsertOne(memberCollection, m)
	if err != nil {
		slog.Error("unable to create member",
			slog.Any("guildID", m.GuildID),
			slog.Any("memberID", m.MemberID),
			slog.Any("error", err),
		)
	}

	return err
}

// updateMember updates an existing member in the database.
func updateMember(m *Member) error {
	versionFilter := bson.M{"version": m.Version}
	if m.Version == 0 {
		versionFilter = bson.M{
			"$or": bson.A{
				bson.M{"version": 0},
				bson.M{"version": bson.M{"$exists": false}},
			},
		}
	}

	filter := bson.M{
		"guild_id":  m.GuildID,
		"member_id": m.MemberID,
	}
	for key, value := range versionFilter {
		filter[key] = value
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
