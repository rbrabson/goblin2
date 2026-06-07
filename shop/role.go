package shop

import (
	"fmt"
	"goblin2/internal/discordid"
	"log/slog"

	"github.com/disgoorg/disgo/discord"
)

const (
	roleItemType = "role"
)

// Role represents a role item in the shop.
type Role Item

// GetRole retrieves a role from the shop by its name for a specific guild.
func GetRole(guildID discordid.SnowflakeID, name string) *Role {
	item := getShopItem(guildID, name, roleItemType)
	if item == nil {
		return nil
	}
	return new(Role(*item))
}

// NewRole creates a new role for the shop.
func NewRole(guildID discordid.SnowflakeID, name string, description string, price int, duration string, autoRenewable bool) *Role {
	item := newShopItem(guildID, name, description, roleItemType, price, duration, autoRenewable, 0)
	role := (*Role)(item)
	return role
}

// Update updates the role's properties in the shop.
func (r *Role) Update(name string, description string, price int, duration string, autoRenewable bool) error {
	item := (*Item)(r)
	return item.update(name, description, roleItemType, price, duration, autoRenewable)
}

// Purchase allows a member to purchase the role from the shop.
func (r *Role) Purchase(memberID discordid.SnowflakeID, renew bool) (*Purchase, error) {
	item := Item(*r)
	return item.purchase(memberID, PURCHASED, renew)
}

// AddToShop adds the role to the shop. If the role already exists, an error is returned.
func (r *Role) AddToShop(s *Shop) error {
	item := (*Item)(r)
	return item.addToShop(s)
}

// RemoveFromShop removes the role from the shop. If the role does not exist, an error is returned.
func (r *Role) RemoveFromShop(s *Shop) error {
	item := (*Item)(r)
	return item.removeFromShop(s)
}

// roleExistsChecks performs checks to see if a role can be added to the shop.
func roleExistsChecks(guildID discordid.SnowflakeID, roleName string) error {
	if _, err := getExistingGuildRole(guildID, roleName); err != nil {
		return err
	}

	return createChecks(guildID, roleName, roleItemType)
}

// rolePurchaseChecks performs checks to see if a role can be purchased.
func rolePurchaseChecks(guildID, memberID discordid.SnowflakeID, roleName string) error {
	guildRole, err := getExistingGuildRole(guildID, roleName)
	if err != nil {
		return err
	}

	member, err := client.Rest.GetMember(guildID.ID(), memberID.ID())
	if err != nil {
		slog.Error("member not found on server",
			slog.Any("guildID", guildID),
			slog.Any("memberID", memberID),
			slog.Any("error", err),
		)
		return fmt.Errorf("member not found on the server")
	}

	for _, roleID := range member.RoleIDs {
		if roleID == guildRole.ID {
			return fmt.Errorf("you already have the `%s` role", roleName)
		}
	}

	shopItem := getShopItem(guildID, roleName, roleItemType)
	if shopItem == nil {
		slog.Error("failed to read role from shop",
			slog.Any("guildID", guildID),
			slog.String("roleName", roleName),
		)
		return fmt.Errorf("role `%s` not found in the shop", roleName)
	}

	if err := purchaseChecks(guildID, memberID, roleItemType, roleName); err != nil {
		return err
	}

	return nil
}

// getExistingGuildRole retrieves an existing role from the guild. If the role does not exist, an error is returned.
func getExistingGuildRole(guildID discordid.SnowflakeID, roleName string) (discord.Role, error) {
	if client == nil {
		return discord.Role{}, fmt.Errorf("discord client is nil")
	}

	roles, err := client.Rest.GetRoles(guildID.ID())
	if err != nil {
		slog.Error("unable to get guild roles",
			slog.Any("guildID", guildID),
			slog.Any("error", err),
		)
		return discord.Role{}, fmt.Errorf("unable to get roles for the server")
	}

	for _, role := range roles {
		if role.Name == roleName {
			return role, nil
		}
	}

	slog.Error("role not found on server",
		slog.Any("guildID", guildID),
		slog.String("roleName", roleName),
	)
	return discord.Role{}, fmt.Errorf("role `%s` not found on the server", roleName)
}
