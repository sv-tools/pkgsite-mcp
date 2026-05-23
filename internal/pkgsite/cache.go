package pkgsite

import (
	"sync"
	"time"
)

// cache is a concurrency-safe, size-bounded TTL cache of raw response bodies,
// keyed by request URL.
type cache struct {
	mu      sync.Mutex
	ttl     time.Duration
	maxSize int
	entries map[string]cacheEntry
}

type cacheEntry struct {
	body    []byte
	expires time.Time
}

func newCache(ttl time.Duration, maxSize int) *cache {
	return &cache{ttl: ttl, maxSize: maxSize, entries: make(map[string]cacheEntry)}
}

// get returns the cached body for key if present and unexpired.
func (c *cache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return e.body, true
}

// set stores body under key, evicting to stay within maxSize.
func (c *cache) set(key string, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxSize {
		c.evictLocked()
	}
	c.entries[key] = cacheEntry{body: body, expires: time.Now().Add(c.ttl)}
}

// evictLocked drops all expired entries, or the soonest-to-expire entry if none
// have expired. The caller must hold c.mu.
func (c *cache) evictLocked() {
	now := time.Now()
	var oldestKey string
	var oldest time.Time
	removed := false
	for k, e := range c.entries {
		if now.After(e.expires) {
			delete(c.entries, k)
			removed = true
			continue
		}
		if oldestKey == "" || e.expires.Before(oldest) {
			oldestKey, oldest = k, e.expires
		}
	}
	if !removed && oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}
