package shop

import (
	"testing"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"goblin2/internal/discordid"
)

type roleLookupRest struct {
	rest.Rest
	roles []discord.Role
}

func (r roleLookupRest) GetRoles(_ snowflake.ID, _ ...rest.RequestOpt) ([]discord.Role, error) {
	return r.roles, nil
}

func TestGetExistingGuildRole(t *testing.T) {
	previous := client
	t.Cleanup(func() { client = previous })
	role := discord.Role{ID: snowflake.ID(724319470528626747), Name: "VIP"}
	client = &bot.Client{Rest: roleLookupRest{roles: []discord.Role{role}}}
	for _, input := range []string{role.Name, role.ID.String(), "<@&724319470528626747>"} {
		t.Run(input, func(t *testing.T) {
			got, err := getExistingGuildRole(discordid.NewSnowflakeID(snowflake.ID(724319470528626738)), input)
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != role.ID || got.Name != role.Name {
				t.Errorf("role = %+v, want %+v", got, role)
			}
		})
	}
	if _, err := getExistingGuildRole(discordid.NewSnowflakeID(snowflake.ID(724319470528626738)), "missing"); err == nil {
		t.Error("missing role lookup succeeded")
	}
}

func TestGetRoleForRemoval(t *testing.T) {
	previous := client
	t.Cleanup(func() { client = previous })
	guildID := discordid.NewSnowflakeID(snowflake.ID(724319470528626738))
	guildRole := discord.Role{ID: snowflake.ID(724319470528626747), Name: "VIP"}
	item := Item{GuildID: guildID, Name: guildRole.Name, Type: roleItemType, Price: 1250}
	key := itemKey(&item)
	itemCache.Set(key, item)
	t.Cleanup(func() { itemCache.Delete(key) })
	client = &bot.Client{Rest: roleLookupRest{roles: []discord.Role{guildRole}}}
	for _, input := range []string{"VIP", "724319470528626747", "<@&724319470528626747>"} {
		t.Run(input, func(t *testing.T) {
			role, err := getRoleForRemoval(guildID, input)
			if err != nil {
				t.Fatal(err)
			}
			if role == nil || role.Name != item.Name || role.Price != item.Price {
				t.Fatalf("got %+v, want stored VIP item", role)
			}
		})
	}
	client = nil
	role, err := getRoleForRemoval(guildID, "VIP")
	if err != nil || role == nil {
		t.Fatalf("removal by stored name must work without Discord: role=%+v, err=%v", role, err)
	}
	if _, err := getRoleForRemoval(guildID, "<@&724319470528626747>"); err == nil {
		t.Fatal("expected role resolution error without Discord")
	}
}

// Constructing a role must not insert it before AddToShop checks for duplicates.
func TestNewRoleDoesNotPersist(t *testing.T) {
	previousDB := db
	db = nil
	t.Cleanup(func() { db = previousDB })
	guildID := discordid.NewSnowflakeID(snowflake.ID(724319470528626738))
	key := itemCacheKey{guildID: guildID, name: "New unsaved role", itemType: roleItemType}
	t.Cleanup(func() { itemCache.Delete(key) })
	role := NewRole(guildID, key.name, "A new role", 1250, "24h", true)
	if role == nil {
		t.Fatal("NewRole returned nil")
	}
	if role.GuildID != guildID || role.Name != key.name || role.Description != "A new role" || role.Type != roleItemType || role.Price != 1250 || role.Duration != "24h" || !role.AutoRenewable {
		t.Fatalf("unexpected role: %+v", role)
	}
	if !role.ID.IsZero() {
		t.Fatalf("unsaved role has database ID: %v", role.ID)
	}
	if _, ok := itemCache.Get(key); ok {
		t.Fatal("unsaved role was added to the item cache")
	}
}

func TestAddToShopRejectsExistingRole(t *testing.T) {
	guildID := discordid.NewSnowflakeID(snowflake.ID(724319470528626738))
	role := NewRole(guildID, "Existing role", "", 1250, "", false)
	item := Item(*role)
	shop := &Shop{GuildID: guildID.String(), Items: []*Item{&item}}
	if err := role.AddToShop(shop); err == nil {
		t.Fatal("expected duplicate role to be rejected")
	}
	if len(shop.Items) != 1 {
		t.Fatalf("shop has %d items, want 1", len(shop.Items))
	}
}
