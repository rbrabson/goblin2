package guild

import (
	"errors"
	"fmt"
	"goblin2/discordid"
	"log/slog"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	defaultAdminRoles = []string{"Admin", "Admins", "Administrator", "Mod", "Mods", "Moderator"}
)

// Guild represents a Discord guild
type Guild struct {
	ID         bson.ObjectID         `bson:"_id,omitempty"`
	GuildID    discordid.SnowflakeID `bson:"guild_id"`
	AdminRoles []string              `bson:"admin_roles"`
	Version    int                   `bson:"version"`
}

// GetGuild returns the guild with the given guild ID, creating a default one if not found.
func GetGuild(guildID discordid.SnowflakeID) *Guild {
	g := readGuild(guildID)
	if g != nil {
		return g
	}
	return createDefaultGuild(guildID)
}

// createDefaultGuild creates a new guild with the given guild ID and default admin roles.
func createDefaultGuild(guildID discordid.SnowflakeID) *Guild {
	return &Guild{
		GuildID:    guildID,
		AdminRoles: append([]string(nil), defaultAdminRoles...),
	}
}

// UpdateGuild applies the given mutation to the guild atomically, retrying on version conflicts.
func UpdateGuild(guildID discordid.SnowflakeID, mutate func(*Guild) error) error {
	const maxRetries = 3

	for range maxRetries {
		g := GetGuild(guildID)

		if err := mutate(g); err != nil {
			return err // business rule failure — don't retry
		}

		var err error
		if g.ID.IsZero() {
			err = writeGuild(g)
		} else {
			err = updateGuild(g)
		}

		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrVersionConflict) {
			return err
		}

		slog.Warn("version conflict on guild, retrying",
			slog.Any("guildID", guildID),
		)
	}

	return fmt.Errorf("failed to update guild after %d retries: %w", maxRetries, ErrVersionConflict)
}

// AddAdminRole adds the given role name to the guild's admin roles.
func (g *Guild) AddAdminRole(roleName string) error {
	return UpdateGuild(g.GuildID, func(latest *Guild) error {
		for _, r := range latest.AdminRoles {
			if r == roleName {
				return ErrRoleAlreadyExists
			}
		}
		latest.AdminRoles = append(latest.AdminRoles, roleName)
		slog.Info("added admin role",
			slog.Any("guildID", latest.GuildID),
			slog.String("role", roleName),
		)
		return nil
	})
}

// RemoveAdminRole removes the given role name from the guild's admin roles.
func (g *Guild) RemoveAdminRole(roleName string) error {
	return UpdateGuild(g.GuildID, func(latest *Guild) error {
		roles := make([]string, 0, len(latest.AdminRoles))
		for _, r := range latest.AdminRoles {
			if r != roleName {
				roles = append(roles, r)
			}
		}
		if len(roles) == len(latest.AdminRoles) {
			return ErrRoleNotFound{guildID: latest.GuildID, roleName: roleName}
		}
		latest.AdminRoles = roles
		slog.Info("removed admin role",
			slog.Any("guildID", latest.GuildID),
			slog.String("role", roleName),
		)
		return nil
	})
}

// GetRoles returns all roles in the guild
func (g *Guild) GetRoles(client *bot.Client) ([]discord.Role, error) {
	roles := client.Caches.Roles(g.GuildID.ID())

	result := make([]discord.Role, 0)
	for role := range roles {
		result = append(result, role)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no roles found in cache")
	}

	return result, nil
}

// GetRole returns the role with the given name
func (g *Guild) GetRole(client *bot.Client, roleName string) (discord.Role, error) {
	for role := range client.Caches.Roles(g.GuildID.ID()) {
		if role.Name == roleName {
			return role, nil
		}
	}

	return discord.Role{}, ErrRoleNotFound{guildID: g.GuildID, roleName: roleName}
}

// GetMember returns a member within this guild.
func (g *Guild) GetMember(member *discord.Member) *Member {
	return GetMember(g.GuildID, member)
}

// String returns a string representation of the guild.
func (g *Guild) String() string {
	return fmt.Sprintf("Guild{ID=%s, GuildID=%v, AdminRoles=%v}",
		g.ID.Hex(),
		g.GuildID,
		g.AdminRoles)
}
