package blackjack

import (
	"goblin2/discordid"
	"goblin2/internal/cache"
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
)

func copyConfig(config *Config) *Config {
	if config == nil {
		return nil
	}
	return new(*config)
}

func copyMember(member *Member) *Member {
	if member == nil {
		return nil
	}
	return new(*member)
}

func CloseCaches() {
	configCache.Destroy()
	memberCache.Destroy()
}
