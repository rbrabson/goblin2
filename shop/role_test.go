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
