package shop

import (
	"cmp"
	"log/slog"
	"slices"
)

// Shop is the shop for a guild. The shop contains all items available for purchase.
type Shop struct {
	GuildID string  // Guild (server) for the shop
	Items   []*Item // All items available in the shop
}

// GetShop returns the shop for the guild.
func GetShop(guildID string) *Shop {
	var err error

	shop := &Shop{
		GuildID: guildID,
	}

	shop.Items, err = readShopItems(guildID)
	if err != nil {
		slog.Error("unable to read shop items from the database", "guildID", guildID, "error", err)
		shop.Items = make([]*Item, 0)
	}

	cacheShopItems(shop.Items)

	for i, item := range shop.Items {
		shop.Items[i] = copyItem(item)
	}

	shopItemCmp := func(a, b *Item) int {
		return cmp.Or(
			cmp.Compare(a.Type, b.Type),
			cmp.Compare(a.Name, b.Name),
		)
	}
	slices.SortFunc(shop.Items, shopItemCmp)

	return shop
}

// GetShopItem finds an item in the shop. If the item does not exist, then nil is returned.
func (s *Shop) GetShopItem(name string, itemType string) *Item {
	for _, item := range s.Items {
		if item.Name == name && item.Type == itemType {
			return copyItem(item)
		}
	}

	return nil
}
