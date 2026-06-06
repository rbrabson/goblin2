package shop

import (
	"errors"
	"fmt"
	"goblin2/bank"
	"goblin2/discordid"
	"log/slog"

	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Item represents an item in the shop, which represents any purchasable item.
type Item struct {
	ID            bson.ObjectID         `json:"_id,omitempty" bson:"_id,omitempty"`
	GuildID       discordid.SnowflakeID `json:"guild_id" bson:"guild_id"`
	Name          string                `json:"name" bson:"name"`
	Description   string                `json:"description" bson:"description"`
	Type          string                `json:"type" bson:"type"`
	Price         int                   `json:"price" bson:"price"`
	Duration      string                `json:"duration,omitempty" bson:"duration,omitempty"`
	AutoRenewable bool                  `json:"auto_renewable,omitempty" bson:"auto_renewable,omitempty"`
	MaxPurchases  int                   `json:"max_purchases,omitempty" bson:"max_purchases,omitempty"`
	Version       int                   `json:"version" bson:"version"`
}

// getShopItem returns the shop item with the given guild ID, name, and type. If the item does
// not exist, nil is returned.
func getShopItem(guildID discordid.SnowflakeID, name string, itemType string) *Item {
	key := itemCacheKey{
		guildID:  guildID,
		name:     name,
		itemType: itemType,
	}

	if item, ok := itemCache.Get(key); ok {
		return copyItem(&item)
	}

	item, err := readShopItem(guildID, name, itemType)
	if err != nil || item == nil {
		return nil
	}

	itemCache.Set(key, *item)
	return copyItem(item)
}

// newShopItem creates a new ShopItem with the given guild ID, name, description, type, and price.
func newShopItem(guildID snowflake.ID, name string, description string, itemType string, price int, duration string, autoRenewable bool, maxPurchases int) *Item {
	item := &Item{
		GuildID:       discordid.NewSnowflakeID(guildID),
		Name:          name,
		Description:   description,
		Type:          itemType,
		Price:         price,
		Duration:      duration,
		AutoRenewable: autoRenewable,
		MaxPurchases:  maxPurchases,
	}

	err := writeShopItem(item)
	if err != nil {
		slog.Error("unable to write shop item to the database", "guild", guildID, "name", name, "type", itemType, "error", err)
		return nil
	}

	itemCache.Set(itemKey(item), *item)

	slog.Info("new shop item created", "guild", guildID, "name", name, "type", itemType)

	return copyItem(item)
}

// UpdateShopItem updates the shop item with the given mutation, retrying on version conflicts.
func UpdateShopItem(guildID snowflake.ID, name string, itemType string, mutate func(*Item) error) error {
	const maxRetries = 3

	itemMu.RLock()
	defer itemMu.RUnlock()

	key := itemCacheKey{
		guildID:  discordid.NewSnowflakeID(guildID),
		name:     name,
		itemType: itemType,
	}

	for range maxRetries {
		item := getShopItem(key.guildID, key.name, key.itemType)
		if item == nil {
			return fmt.Errorf("%s `%s` not found in the shop", itemType, name)
		}

		oldKey := itemKey(item)

		if err := mutate(item); err != nil {
			return err
		}

		err := updateShopItem(item)
		if err == nil {
			itemCache.Delete(oldKey)
			itemCache.Set(itemKey(item), *item)
			return nil
		}
		if !errors.Is(err, bank.ErrVersionConflict) {
			itemCache.Delete(oldKey)
			return err
		}

		itemCache.Delete(oldKey)

		slog.Warn("version conflict on shop item, retrying",
			slog.Any("guildID", guildID),
			slog.String("name", name),
			slog.String("type", itemType),
		)
	}

	return fmt.Errorf("failed to update shop item after %d retries: %w", maxRetries, bank.ErrVersionConflict)
}

// update updates the shop item with the given name and type. If the item does not exist, an error is returned.
func (item *Item) update(name string, description string, itemType string, price int, duration string, autoRenewable bool) error {
	if item.Name == name && item.Description == description && item.Type == itemType && item.Price == price && duration == item.Duration && autoRenewable == item.AutoRenewable {
		slog.Warn("no change to the shop item", "guild", item.GuildID, "name", item.Name, "type", item.Type)
		return fmt.Errorf("no change to the shop item")
	}

	oldName := item.Name
	oldType := item.Type

	err := UpdateShopItem(item.GuildID.ID(), oldName, oldType, func(latest *Item) error {
		latest.Name = name
		latest.Description = description
		latest.Type = itemType
		latest.Price = price
		latest.Duration = duration
		latest.AutoRenewable = autoRenewable
		return nil
	})
	if err != nil {
		slog.Error("unable to update shop item to the database", "guild", item.GuildID, "name", item.Name, "type", item.Type, "error", err)
		return fmt.Errorf("unable to add item")
	}

	slog.Info("shop item updated", "guild", item.GuildID, "name", name, "type", itemType)
	return nil
}

// addToShop adds the shop item to the given shop. If the item already exists in the shop, an error is returned.
func (item *Item) addToShop(s *Shop) error {
	existingItem := s.GetShopItem(item.Name, item.Type)
	if existingItem != nil {
		return fmt.Errorf("%s already exists in the shop", item.Type)
	}

	err := writeShopItem(item)
	if err != nil {
		slog.Error("unable to write shop item to the database", "guild", item.GuildID, "name", item.Name, "type", item.Type, "error", err)
		return fmt.Errorf("unable to add %s to shop", item.Type)
	}

	itemCache.Set(itemKey(item), *item)
	s.Items = append(s.Items, copyItem(item))
	slog.Info("shop item added to shop", "guild", item.GuildID, "name", item.Name, "type", item.Type)
	return nil
}

// removeFromShop removes the shop item from the given shop. If the item does not exist in the shop, an error is returned.
func (item *Item) removeFromShop(s *Shop) error {
	existingItem := s.GetShopItem(item.Name, item.Type)
	if existingItem == nil {
		return fmt.Errorf("%s does not exist in the shop", item.Type)
	}

	err := deleteShopItem(item)
	if err != nil {
		slog.Error("unable to remove shop item from the database", "guild", item.GuildID, "name", item.Name, "type", item.Type, "error", err)
		return fmt.Errorf("unable to remove %s from shop", item.Type)
	}

	itemCache.Delete(itemKey(item))

	for i, it := range s.Items {
		if it.ID == item.ID {
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
			break
		}
	}

	slog.Info("shop item removed from shop", "guild", item.GuildID, "name", item.Name, "type", item.Type)
	return nil
}

// purchase purchases the shop item for the given member. If the purchase is successful, a purchase
// object is returned. If the purchase fails, an error is returned.
func (item *Item) purchase(memberID snowflake.ID, status string, renew bool) (*Purchase, error) {
	purchase, err := PurchaseItem(item.GuildID.ID(), memberID, item, status, renew)
	if err != nil {
		slog.Error("unable to create purchase", "guild", item.GuildID, "member", memberID, "item", item.Name, "error", err)
		return nil, err
	}

	return purchase, nil
}

// createChecks performs checks to see if a role can be added to the shop.
func createChecks(guildID snowflake.ID, itemName string, itemType string) error {
	shopItem := getShopItem(discordid.NewSnowflakeID(guildID), itemName, itemType)
	if shopItem != nil {
		slog.Error("item already exists in the shop", "guild", guildID, "name", itemName, "type", itemType)
		return fmt.Errorf("%s `%s` already exists in the shop", itemType, itemName)
	}

	return nil
}

// purchaseChecks performs checks to see if a member can purchase the shop item.
func purchaseChecks(guildID snowflake.ID, memberID snowflake.ID, itemType string, itemName string) error {
	purchase := getPurchase(discordid.NewSnowflakeID(guildID), discordid.NewSnowflakeID(memberID), itemName, itemType)
	if purchase != nil && !purchase.IsExpired {
		slog.Debug("item already purchased", "guild", guildID, "member", memberID, "name", itemName, "type", itemType)
		return fmt.Errorf("you have already purchased %s `%s`", itemType, itemName)
	}

	// Make sure the member has enough funds to purchase the item
	item := getShopItem(discordid.NewSnowflakeID(guildID), itemName, itemType)
	if item == nil {
		slog.Error("failed to read item from shop", "guildID", guildID, "itemName", itemName, "itemType", itemType)
		return fmt.Errorf("%s `%s` not found in the shop", itemType, itemName)
	}
	bankAccount := bank.GetAccount(guildID, memberID)
	if bankAccount.CurrentBalance < item.Price {
		slog.Debug("insufficient funds to purchase item", "guild", guildID, "name", itemName, "type", itemType, "member", memberID)
		return fmt.Errorf("you do not have enough credits to purchase the `%s` %s", itemName, itemType)
	}
	return nil
}

// deleteShopItem deletes the shop item from the database. If the item does not exist, an error is returned.

// String returns a string representation of the Role.
func (item *Item) String() string {
	return fmt.Sprintf("ShopItem{ID: %s, Guild: %s, Type: %s, Name: %s, Price: %d Description: %s, Duration: %s, AutoRenewable: %t}",
		item.ID.Hex(),
		item.GuildID,
		item.Type,
		item.Name,
		item.Price,
		item.Description,
		item.Duration,
		item.AutoRenewable,
	)
}
