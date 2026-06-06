package cache

import (
	"sync"
	"time"
)

// entry is a single entry in the cache.
type entry[V any] struct {
	value     V
	expiresAt time.Time
	ttl       time.Duration
}

// TimedMap is a thread-safe map with sliding expiration.
type TimedMap[K comparable, V any] struct {
	mu              sync.RWMutex
	items           map[K]entry[V]
	defaultTTL      time.Duration
	cleanupInterval time.Duration

	stopCh    chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// New creates a new timed cache with sliding expiration.
//
// Any successful access (Get, GetOrSet, Range) refreshes the entry's TTL.
//
// cleanupInterval controls how often expired entries are purged. If
// cleanupInterval is less than or equal to zero, the defaultTTL is used. If both are
// less than or equal to zero, a one-minute cleanup interval is used.
//
// Close should be called when the cache is no longer needed to stop the
// background cleanup goroutine.
func New[K comparable, V any](defaultTTL time.Duration, cleanupInterval time.Duration) *TimedMap[K, V] {
	if cleanupInterval <= 0 {
		cleanupInterval = defaultTTL
	}
	if cleanupInterval <= 0 {
		cleanupInterval = time.Minute
	}

	tm := &TimedMap[K, V]{
		items:           make(map[K]entry[V]),
		defaultTTL:      defaultTTL,
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
	}

	tm.wg.Add(1)
	go tm.cleanupLoop()

	return tm
}

// Set inserts or replaces a value using the default TTL.
func (tm *TimedMap[K, V]) Set(key K, value V) {
	tm.SetWithTTL(key, value, tm.defaultTTL)
}

// SetWithTTL inserts or replaces a value using a custom TTL.
//
// If ttl is less than or equal to zero, the value is not cached.
func (tm *TimedMap[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	if ttl <= 0 {
		return
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.items[key] = entry[V]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
		ttl:       ttl,
	}
}

// Get retrieves a value and refreshes its expiration time.
func (tm *TimedMap[K, V]) Get(key K) (V, bool) {
	var zero V

	now := time.Now()

	tm.mu.Lock()
	defer tm.mu.Unlock()

	item, ok := tm.items[key]
	if !ok {
		return zero, false
	}

	if now.After(item.expiresAt) {
		delete(tm.items, key)
		return zero, false
	}

	item.expiresAt = now.Add(item.ttl)
	tm.items[key] = item

	return item.value, true
}

// GetOrSet returns an existing value if present and not expired.
//
// If the key is missing or expired, factory() is called,
// the result is stored and returned.
//
// Returns:
//
//	value, true  -> existing value found
//	value, false -> new value created
//
// If the default TTL is less than or equal to zero, factory() is still called
// for missing or expired keys, but the result is not cached.
func (tm *TimedMap[K, V]) GetOrSet(key K, factory func() V) (V, bool) {
	now := time.Now()

	tm.mu.Lock()
	if item, ok := tm.items[key]; ok {
		if now.Before(item.expiresAt) {
			item.expiresAt = now.Add(item.ttl)
			tm.items[key] = item
			tm.mu.Unlock()

			return item.value, true
		}

		delete(tm.items, key)
	}
	tm.mu.Unlock()

	value := factory()

	if tm.defaultTTL <= 0 {
		return value, false
	}

	now = time.Now()

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if item, ok := tm.items[key]; ok {
		if now.Before(item.expiresAt) {
			item.expiresAt = now.Add(item.ttl)
			tm.items[key] = item

			return item.value, true
		}

		delete(tm.items, key)
	}

	tm.items[key] = entry[V]{
		value:     value,
		expiresAt: now.Add(tm.defaultTTL),
		ttl:       tm.defaultTTL,
	}

	return value, false
}

// Delete removes a key from the cache.
func (tm *TimedMap[K, V]) Delete(key K) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	delete(tm.items, key)
}

// Clear removes all entries from the cache without stopping the background
// cleanup goroutine.
func (tm *TimedMap[K, V]) Clear() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.items = make(map[K]entry[V])
}

// Destroy stops the background cleanup goroutine and removes all entries from
// the cache. After Destroy is called, the TimedMap should not be used again.
func (tm *TimedMap[K, V]) Destroy() {
	tm.Close()
	tm.Clear()
}

// Len returns the current number of entries.
// Expired entries that haven't yet been cleaned up are included.
func (tm *TimedMap[K, V]) Len() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return len(tm.items)
}

// Range iterates over all non-expired entries.
//
// Each non-expired entry has its expiration refreshed before callbacks are run.
//
// Returning false from fn stops iteration.
func (tm *TimedMap[K, V]) Range(fn func(K, V) bool) {
	type pair struct {
		key   K
		value V
	}

	now := time.Now()

	tm.mu.Lock()

	items := make([]pair, 0, len(tm.items))

	for k, item := range tm.items {
		if now.After(item.expiresAt) {
			delete(tm.items, k)
			continue
		}

		item.expiresAt = now.Add(item.ttl)
		tm.items[k] = item

		items = append(items, pair{
			key:   k,
			value: item.value,
		})
	}

	tm.mu.Unlock()

	for _, item := range items {
		if !fn(item.key, item.value) {
			return
		}
	}
}

// Close stops the background cleanup goroutine.
//
// Close is safe to call multiple times.
func (tm *TimedMap[K, V]) Close() {
	tm.closeOnce.Do(func() {
		close(tm.stopCh)
	})
	tm.wg.Wait()
}

// cleanupLoop runs in a background goroutine and periodically purges expired
// entries from the cache.
func (tm *TimedMap[K, V]) cleanupLoop() {
	defer tm.wg.Done()

	ticker := time.NewTicker(tm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.cleanupExpired()
		case <-tm.stopCh:
			return
		}
	}
}

// cleanupExpired removes expired entries from the cache.
func (tm *TimedMap[K, V]) cleanupExpired() {
	now := time.Now()

	tm.mu.Lock()
	defer tm.mu.Unlock()

	for key, item := range tm.items {
		if now.After(item.expiresAt) {
			delete(tm.items, key)
		}
	}
}
