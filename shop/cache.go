package shop

import (
	"goblin2/discordid"
	"goblin2/internal/cache"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	shopCacheTTL             = 30 * time.Minute
	shopCacheCleanupInterval = 5 * time.Minute
)

// configCacheKey is the key for the shop config cache
type configCacheKey struct {
	guildID discordid.SnowflakeID
}

// itemCacheKey is the key for the shop item cache
type itemCacheKey struct {
	guildID  discordid.SnowflakeID
	name     string
	itemType string
}

// purchaseCacheKey is the key for the shop purchase cache
type purchaseCacheKey struct {
	guildID   discordid.SnowflakeID
	memberID  discordid.SnowflakeID
	itemName  string
	itemType  string
	isExpired bool
}

// memberCacheKey is the key for the shop member cache
type memberCacheKey struct {
	guildID  discordid.SnowflakeID
	memberID discordid.SnowflakeID
}

var (
	configCache   = cache.New[configCacheKey, Config](shopCacheTTL, shopCacheCleanupInterval)
	itemCache     = cache.New[itemCacheKey, Item](shopCacheTTL, shopCacheCleanupInterval)
	purchaseCache = cache.New[purchaseCacheKey, Purchase](shopCacheTTL, shopCacheCleanupInterval)
	memberCache   = cache.New[memberCacheKey, Member](shopCacheTTL, shopCacheCleanupInterval)

	configMu   sync.RWMutex
	itemMu     sync.RWMutex
	purchaseMu sync.RWMutex
	memberMu   sync.RWMutex
)

// copyConfig returns a copy of the config.
func copyConfig(config *Config) *Config {
	if config == nil {
		return nil
	}
	return new(*config)
}

// copyItem returns a copy of the item.
func copyItem(item *Item) *Item {
	if item == nil {
		return nil
	}
	return new(*item)
}

// copyPurchase returns a copy of the purchase.
func copyPurchase(purchase *Purchase) *Purchase {
	if purchase == nil {
		return nil
	}

	copied := new(*purchase)
	if purchase.Item != nil {
		copied.Item = copyItem(purchase.Item)
	}
	return copied
}

// copyMember returns a copy of the member.
func copyMember(member *Member) *Member {
	if member == nil {
		return nil
	}

	copied := new(*member)
	copied.Restrictions = append([]string(nil), member.Restrictions...)
	return copied
}

// itemKey returns the cache key for the given item.
func itemKey(item *Item) itemCacheKey {
	return itemCacheKey{
		guildID:  item.GuildID,
		name:     item.Name,
		itemType: item.Type,
	}
}

// purchaseKey returns the cache key for the given purchase.
func purchaseKey(purchase *Purchase) purchaseCacheKey {
	return purchaseCacheKey{
		guildID:   purchase.GuildID,
		memberID:  purchase.MemberID,
		itemName:  purchase.Item.Name,
		itemType:  purchase.Item.Type,
		isExpired: purchase.IsExpired,
	}
}

// cacheShopItems caches the given items.
func cacheShopItems(items []*Item) {
	for _, item := range items {
		itemCache.Set(itemKey(item), *item)
	}
}

// cachePurchases caches the given purchases.
func cachePurchases(purchases []*Purchase) {
	for _, purchase := range purchases {
		purchaseCache.Set(purchaseKey(purchase), *purchase)
	}
}

// cacheMembers caches the given members.
func cacheMembers(members []*Member) {
	for _, member := range members {
		key := memberCacheKey{
			guildID:  member.GuildID,
			memberID: member.MemberID,
		}
		memberCache.Set(key, *member)
	}
}

// isZeroObjectID returns true if the given ID is zero or nil.
func isZeroObjectID(id bson.ObjectID) bool {
	return id.IsZero() || id == bson.NilObjectID
}

// CloseCaches closes the caches.
func CloseCaches() {
	configCache.Destroy()
	itemCache.Destroy()
	purchaseCache.Destroy()
	memberCache.Destroy()
}
