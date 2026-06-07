package shop

import (
	"goblin2/bank"
	"goblin2/database"
	"goblin2/internal/discordid"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	configCollection   = "shop_configs"
	shopItemCollection = "shop_items"
	purchaseCollection = "shop_purchases"
	memberCollection   = "shop_members"
)

var (
	db *database.MongoDB
)

// readConfig reads the configuration from the database. If the config does not exist, it returns nil.
func readConfig(guildID discordid.SnowflakeID) (*Config, error) {
	filter := bson.M{"guild_id": guildID}
	var config Config
	if err := db.FindOne(configCollection, filter, &config); err != nil {
		slog.Debug("shop config not found in the database", "guildID", guildID, "filter", filter, "error", err)
		return nil, err
	}
	slog.Debug("read shop config from the database", "guildID", guildID)

	return &config, nil
}

// writeConfig inserts the configuration into the database.
func writeConfig(config *Config) error {
	config.Version = 0

	result, err := db.InsertOne(configCollection, config)
	if err != nil {
		slog.Error("unable to create shop config", "guildID", config.GuildID, "error", err)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		config.ID = id
	}

	return nil
}

// updateConfig updates the configuration using optimistic locking.
func updateConfig(config *Config) error {
	filter := bson.M{"guild_id": config.GuildID}
	addVersionFilter(filter, config.Version)

	update := bson.M{
		"$set": bson.M{
			"channel_id":      config.ChannelID,
			"message_id":      config.MessageID,
			"mod_channel_id":  config.ModChannelID,
			"notification_id": config.NotificationID,
		},
		"$inc": bson.M{"version": 1},
	}

	result, err := db.UpdateOne(configCollection, filter, update)
	if err != nil {
		slog.Error("unable to update shop config", "guildID", config.GuildID, "version", config.Version, "error", err)
		return err
	}
	if result.MatchedCount == 0 {
		return bank.ErrVersionConflict
	}

	config.Version++
	return nil
}

// readShopItems reads all the shop items for the given guild.
func readShopItems(guildID string) ([]*Item, error) {
	filter := bson.M{"guild_id": guildID}
	sortBy := bson.M{"name": 1}
	var items []*Item
	err := db.FindMany(shopItemCollection, filter, &items, sortBy, 0)
	if err != nil {
		slog.Error("unable to read shop items from the database", "guildID", guildID, "filter", filter, "error", err)
		return nil, err
	}

	return items, nil
}

// readShopItem reads the shop item with the given name and type for the given guild.
func readShopItem(guildID discordid.SnowflakeID, name string, itemType string) (*Item, error) {
	filter := bson.D{{Key: "guild_id", Value: guildID}, {Key: "name", Value: name}, {Key: "type", Value: itemType}}
	var item Item
	if err := db.FindOne(shopItemCollection, filter, &item); err != nil {
		slog.Debug("unable to read shop item from the database", "guildID", guildID, "filter", filter, "error", err)
		return nil, err
	}

	return &item, nil
}

// writeShopItem inserts the shop item into the database.
func writeShopItem(item *Item) error {
	item.Version = 0

	result, err := db.InsertOne(shopItemCollection, item)
	if err != nil {
		slog.Error("unable to create shop item", "guildID", item.GuildID, "item", item, "error", err)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		item.ID = id
	}

	return nil
}

// updateShopItem updates the shop item using optimistic locking.
func updateShopItem(item *Item) error {
	filter := bson.M{"_id": item.ID}
	addVersionFilter(filter, item.Version)

	update := bson.M{
		"$set": bson.M{
			"guild_id":       item.GuildID,
			"name":           item.Name,
			"description":    item.Description,
			"type":           item.Type,
			"price":          item.Price,
			"duration":       item.Duration,
			"auto_renewable": item.AutoRenewable,
			"max_purchases":  item.MaxPurchases,
		},
		"$inc": bson.M{"version": 1},
	}

	result, err := db.UpdateOne(shopItemCollection, filter, update)
	if err != nil {
		slog.Error("unable to update shop item", "guildID", item.GuildID, "item", item, "version", item.Version, "error", err)
		return err
	}
	if result.MatchedCount == 0 {
		return bank.ErrVersionConflict
	}

	item.Version++
	return nil
}

// deleteShopItem deletes the shop item from the database.
func deleteShopItem(item *Item) error {
	var filter bson.D
	if !isZeroObjectID(item.ID) {
		filter = bson.D{{Key: "_id", Value: item.ID}}
	} else {
		filter = bson.D{{Key: "guild_id", Value: item.GuildID}, {Key: "name", Value: item.Name}, {Key: "type", Value: item.Type}}
	}
	_, err := db.DeleteOne(shopItemCollection, filter)
	if err != nil {
		slog.Error("unable to delete shop item from the database", "guildID", item.GuildID, "filter", filter, "error", err)
		return err
	}

	itemCache.Delete(itemKey(item))

	return nil
}

// readAllPurchases reads all the purchases from the database that match the input filter
func readAllPurchases(filter interface{}) ([]*Purchase, error) {
	var items []*Purchase
	err := db.FindMany(purchaseCollection, filter, &items, bson.D{}, 0)
	if err != nil {
		slog.Error("unable to read all purchases from the database", "filter", filter, "error", err)
		return nil, err
	}

	return items, nil
}

// readPurchases reads all the purchases for the member in the given guild.
func readPurchases(guildID string, memberID string) ([]*Purchase, error) {
	filter := bson.M{"guild_id": guildID, "member_id": memberID}
	sortBy := bson.M{"name": 1}
	var items []*Purchase
	err := db.FindMany(purchaseCollection, filter, &items, sortBy, 0)
	if err != nil {
		slog.Error("unable to read purchases from the database", "guildID", guildID, "memberID", memberID, "filter", filter, "error", err)
		return nil, err
	}

	return items, nil
}

// readPurchase reads the purchase with the given name and type for the given guild.
func readPurchase(guildID discordid.SnowflakeID, memberID discordid.SnowflakeID, itemName string, itemType string) (*Purchase, error) {
	filter := bson.D{{Key: "guild_id", Value: guildID}, {Key: "member_id", Value: memberID}, {Key: "name", Value: itemName}, {Key: "type", Value: itemType}, {Key: "is_expired", Value: false}}
	var item Purchase
	if err := db.FindOne(purchaseCollection, filter, &item); err != nil {
		slog.Debug("unable to read purchase from the database", "filter", filter, "error", err)
		return nil, err
	}

	return &item, nil
}

// writePurchase inserts the purchase into the database.
func writePurchase(item *Purchase) error {
	item.Version = 0

	result, err := db.InsertOne(purchaseCollection, item)
	if err != nil {
		slog.Error("unable to create purchase", "guildID", item.Item.GuildID, "item", item, "error", err)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		item.ID = id
	}

	purchaseCache.Set(purchaseKey(item), *item)

	return nil
}

// updatePurchase updates the purchase using optimistic locking.
func updatePurchase(item *Purchase) error {
	filter := bson.M{"_id": item.ID}
	addVersionFilter(filter, item.Version)

	update := bson.M{
		"$set": bson.M{
			"guild_id":     item.GuildID,
			"member_id":    item.MemberID,
			"item":         item.Item,
			"status":       item.Status,
			"purchased_on": item.PurchasedOn,
			"expires_on":   item.ExpiresOn,
			"auto_renew":   item.AutoRenew,
			"is_expired":   item.IsExpired,
		},
		"$inc": bson.M{"version": 1},
	}

	result, err := db.UpdateOne(purchaseCollection, filter, update)
	if err != nil {
		slog.Error("unable to update purchase", "guildID", item.GuildID, "item", item, "version", item.Version, "error", err)
		return err
	}
	if result.MatchedCount == 0 {
		return bank.ErrVersionConflict
	}

	item.Version++
	return nil
}

// deletePurchase deletes the purchase from the database.
func deletePurchase(purchase *Purchase) error {
	var filter bson.D
	if !isZeroObjectID(purchase.ID) {
		filter = bson.D{{Key: "_id", Value: purchase.ID}}
	} else {
		filter = bson.D{{Key: "guild_id", Value: purchase.Item.GuildID}, {Key: "member_id", Value: purchase.MemberID}, {Key: "name", Value: purchase.Item.Name}, {Key: "type", Value: purchase.Item.Type}}
	}
	_, err := db.DeleteOne(purchaseCollection, filter)
	if err != nil {
		slog.Error("unable to delete purchase from the database", "guildID", purchase.Item.GuildID, "filter", filter, "error", err)
		return err
	}

	purchaseCache.Delete(purchaseKey(purchase))

	return nil
}

// readMember reads the member from the database.
func readMember(guildID discordid.SnowflakeID, memberID discordid.SnowflakeID) (*Member, error) {
	filter := bson.D{{Key: "guild_id", Value: guildID}, {Key: "member_id", Value: memberID}}
	var member Member
	if err := db.FindOne(memberCollection, filter, &member); err != nil {
		slog.Debug("unable to read shop member from the database", "guildID", guildID, "memberID", memberID, "error", err)
		return nil, err
	}

	return &member, nil
}

// writeMember inserts the member into the database.
func writeMember(member *Member) error {
	member.Version = 0

	result, err := db.InsertOne(memberCollection, member)
	if err != nil {
		slog.Error("unable to create shop member", "guildID", member.GuildID, "memberID", member.MemberID, "error", err)
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		member.ID = id
	}

	return nil
}

// updateMember updates the member using optimistic locking.
func updateMember(member *Member) error {
	filter := bson.M{"guild_id": member.GuildID, "member_id": member.MemberID}
	addVersionFilter(filter, member.Version)

	update := bson.M{
		"$set": bson.M{
			"restrictions": member.Restrictions,
		},
		"$inc": bson.M{"version": 1},
	}

	result, err := db.UpdateOne(memberCollection, filter, update)
	if err != nil {
		slog.Error("unable to update shop member", "guildID", member.GuildID, "memberID", member.MemberID, "version", member.Version, "error", err)
		return err
	}
	if result.MatchedCount == 0 {
		return bank.ErrVersionConflict
	}

	member.Version++
	return nil
}

// deleteMember deletes the shop item from the database.
func deleteMember(member *Member) error {
	var filter bson.D
	if !isZeroObjectID(member.ID) {
		filter = bson.D{{Key: "_id", Value: member.ID}}
	} else {
		filter = bson.D{{Key: "guild_id", Value: member.GuildID}, {Key: "member_id", Value: member.MemberID}}
	}
	_, err := db.DeleteOne(memberCollection, filter)
	if err != nil {
		slog.Error("unable to delete shop member from the database", "guildID", member.GuildID, "memberID", member.MemberID, "filter", filter, "error", err)
		return err
	}

	memberCache.Delete(memberCacheKey{
		guildID:  member.GuildID,
		memberID: member.MemberID,
	})

	return nil
}

// listMembers lists all the members in the given guild.
func listMembers(guildID string) ([]*Member, error) {
	filter := bson.D{{Key: "guild_id", Value: guildID}}
	var members []*Member
	err := db.FindMany(memberCollection, filter, &members, bson.D{}, 0)
	if err != nil {
		slog.Error("unable to read shop members from the database", "guildID", guildID, "filter", filter, "error", err)
		return nil, err
	}
	cacheMembers(members)
	slog.Debug("read shop members from the database", "guildID", guildID, "count", len(members))

	return members, nil
}

func addVersionFilter(filter bson.M, version int) {
	if version == 0 {
		filter["$or"] = bson.A{
			bson.M{"version": version},
			bson.M{"version": bson.M{"$exists": false}},
		}
		return
	}

	filter["version"] = version
}
