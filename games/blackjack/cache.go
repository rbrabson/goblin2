package blackjack

import (
	"goblin2/internal/cache"
	"goblin2/internal/discordid"
	"sync"
	"time"
)

const (
	blackjackCacheTTL             = 30 * time.Minute
	blackjackCacheCleanupInterval = 5 * time.Minute
)

type configCacheKey struct {
	guildID discordid.SnowflakeID
}

type memberCacheKey struct {
	guildID  discordid.SnowflakeID
	memberID discordid.SnowflakeID
}

var (
	configCache = cache.New[configCacheKey, Config](blackjackCacheTTL, blackjackCacheCleanupInterval)
	memberCache = cache.New[memberCacheKey, Member](blackjackCacheTTL, blackjackCacheCleanupInterval)

	memberLoadMu sync.Mutex
)

// copyConfig returns a copy of the given Config.
func copyConfig(config *Config) *Config {
	if config == nil {
		return nil
	}
	return new(*config)
}

// copyMember returns a copy of the given Member.
func copyMember(member *Member) *Member {
	if member == nil {
		return nil
	}
	return new(*member)
}

// CloseCaches closes the caches.
func CloseCaches() {
	configCache.Destroy()
	memberCache.Destroy()
}
