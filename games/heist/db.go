package heist

import (
	"goblin2/database"
	"goblin2/discordid"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	configCollection = "heist_configs"
	memberCollection = "heist_members"
	targetCollection = "heist_targets"
	themeCollection  = "heist_themes"
)

var (
	db *database.MongoDB
)

// readConfig loads the heist configuration from the database. If it does not exist, then
// a `nil` value is returned.
func readConfig(guildID discordid.SnowflakeID) *Config {
	filter := bson.M{"guild_id": guildID}
	var config Config
	err := db.FindOne(configCollection, filter, &config)
	if err != nil {
		slog.Debug("heist configuration not found in the database",
			slog.Any("guildID", guildID),
			slog.Any("error", err),
		)
		return nil
	}

	return &config
}

// writeConfig stores the configuration in the database.
func writeConfig(config *Config) {
	var filter bson.M
	if config.ID != bson.NilObjectID {
		filter = bson.M{"_id": config.ID}
	} else {
		filter = bson.M{"guild_id": config.GuildID}
	}
	if _, err := db.ReplaceOneUpsert(configCollection, filter, config); err != nil {
		slog.Error("error writing heist configuration to database",
			slog.Any("guildID", config.GuildID),
			slog.Any("error", err),
		)
		return
	}

	configCache.Set(configCacheKey{guildID: config.GuildID}, *config)
}

// readMember loads the heist member from the database. If it does not exist, then
// a `nil` value is returned.
func readMember(guildID, memberID discordid.SnowflakeID) *Member {
	var heistMember Member
	filter := bson.M{"guild_id": guildID, "member_id": memberID}
	err := db.FindOne(memberCollection, filter, &heistMember)
	if err != nil {
		slog.Debug("heist member not found in the database",
			slog.Any("guildID", guildID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		return nil
	}

	return &heistMember
}

// Write creates or updates the heist member in the database
func writeMember(member *Member) {
	var filter bson.M
	if member.ID != bson.NilObjectID {
		filter = bson.M{"_id": member.ID}
	} else {
		filter = bson.M{"guild_id": member.GuildID, "member_id": member.MemberID}
	}
	if _, err := db.ReplaceOneUpsert(memberCollection, filter, member); err != nil {
		slog.Error("error writing heist member to the database",
			slog.Any("guildID", member.GuildID),
			slog.Any("memberID", member.MemberID),
			slog.Any("error", err),
		)
		return
	}

	memberCache.Set(memberCacheKey{guildID: member.GuildID, memberID: member.MemberID}, *member)
}

// readAllTargets loads the targets that may be used in heists for all guilds
func readAllTargets(filter bson.M) ([]*Target, error) {
	var targets []*Target
	sort := bson.M{"crew_size": 1}
	err := db.FindMany(targetCollection, filter, &targets, sort, 0)
	if err != nil {
		slog.Error("unable to read targets", slog.Any("error", err), slog.Any("filter", filter))
		return nil, err
	}

	return targets, nil
}

// readTargets loads the targets that may be used in heists by the given guild
func readTargets(guildID discordid.SnowflakeID, theme string) ([]*Target, error) {
	var targets []*Target
	sort := bson.M{"crew_size": 1}
	filter := bson.M{"guild_id": guildID, "theme": theme}
	err := db.FindMany(targetCollection, filter, &targets, sort, 0)
	if err != nil {
		slog.Error("unable to read targets",
			slog.Any("guildID", guildID),
			slog.String("theme", theme),
			slog.Any("error", err),
		)
		return nil, err
	}

	return targets, nil
}

// writeTarget writes the set of targets to the database. If they already exist, they are updated; otherwise, the set is created.
func writeTarget(target *Target) {
	var filter bson.M
	if target.ID != bson.NilObjectID {
		filter = bson.M{"_id": target.ID}
	} else {
		filter = bson.M{"guild_id": target.GuildID, "target_id": target.Name}
	}

	if _, err := db.ReplaceOneUpsert(targetCollection, filter, target); err != nil {
		slog.Error("error writing target to database",
			slog.Any("guildID", target.GuildID),
			slog.String("targetID", target.Name),
			slog.Any("error", err),
		)
		return
	}

	targetsCache.Delete(targetsCacheKey{guildID: target.GuildID, theme: target.Theme})
}

// readAllThemes loads all available themes for a guild
func readAllThemes(guildID discordid.SnowflakeID) ([]*Theme, error) {
	var themes []*Theme
	filter := bson.M{"guild_id": guildID}
	err := db.FindMany(themeCollection, filter, &themes, bson.M{}, 0)
	if err != nil {
		slog.Error("unable to read themes",
			slog.Any("guildID", guildID),
			slog.Any("error", err),
		)
		return nil, err
	}

	return themes, nil
}

// readTheme loads the requested theme for a guild
func readTheme(guildID discordid.SnowflakeID) (*Theme, error) {
	var theme Theme
	filter := bson.M{"guild_id": guildID}
	err := db.FindOne(themeCollection, filter, &theme)
	if err != nil {
		slog.Error("unable to read theme",
			slog.Any("guildID", guildID),
			slog.Any("error", err),
		)
		return nil, err
	}

	return &theme, nil
}

// write creates or updates the theme in the database
func writeTheme(theme *Theme) {
	var filter bson.M
	if theme.ID != bson.NilObjectID {
		filter = bson.M{"_id": theme.ID}
	} else {
		filter = bson.M{"guild_id": theme.GuildID, "name": theme.Name}
	}
	if _, err := db.ReplaceOneUpsert(themeCollection, filter, theme); err != nil {
		slog.Error("error writing theme to the database",
			slog.Any("guildID", theme.GuildID),
			slog.String("name", theme.Name),
			slog.Any("error", err),
		)
		return
	}

	themeCache.Set(themeCacheKey{guildID: theme.GuildID}, *theme)
	targetsCache.Delete(targetsCacheKey{guildID: theme.GuildID, theme: theme.Name})
}
