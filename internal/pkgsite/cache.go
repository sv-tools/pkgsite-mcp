package pkgsite

import (
	"container/list"
	"sync"
	"time"
)

// cache is a concurrency-safe LRU cache of raw response bodies, keyed by request
// URL. It is bounded by both an entry count and a total byte budget, and each
// entry carries a TTL. When a bound is exceeded the least-recently-used entries
// are evicted; expired entries are dropped lazily when next accessed.
type cache struct {
	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	maxBytes   int64
	curBytes   int64
	ll         *list.List               // front = most recently used
	items      map[string]*list.Element // key -> element holding *cacheEntry
}

type cacheEntry struct {
	key     string
	body    []byte
	expires time.Time
}

func newCache(ttl time.Duration, maxEntries int, maxBytes int64) *cache {
	return &cache{
		ttl:        ttl,
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
		ll:         list.New(),
		items:      make(map[string]*list.Element),
	}
}

// get returns the cached body for key if present and unexpired, promoting it to
// most-recently-used.
func (c *cache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	e := el.Value.(*cacheEntry)
	if time.Now().After(e.expires) {
		c.removeLocked(el)
		return nil, false
	}
	c.ll.MoveToFront(el)
	return e.body, true
}

// set stores body under key, then evicts least-recently-used entries until both
// bounds are satisfied. A body that alone exceeds the byte budget is not cached,
// since storing it would evict everything else and still not fit.
func (c *cache) set(key string, body []byte) {
	size := int64(len(body))
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.maxBytes > 0 && size > c.maxBytes {
		if el, ok := c.items[key]; ok {
			c.removeLocked(el)
		}
		return
	}

	expires := time.Now().Add(c.ttl)
	if el, ok := c.items[key]; ok {
		e := el.Value.(*cacheEntry)
		c.curBytes += size - int64(len(e.body))
		e.body, e.expires = body, expires
		c.ll.MoveToFront(el)
	} else {
		el := c.ll.PushFront(&cacheEntry{key: key, body: body, expires: expires})
		c.items[key] = el
		c.curBytes += size
	}
	c.evictLocked()
}

// evictLocked removes least-recently-used entries until both the entry-count and
// byte bounds hold. A non-positive bound is treated as unlimited. The caller must
// hold c.mu.
func (c *cache) evictLocked() {
	for c.ll.Len() > 0 &&
		((c.maxEntries > 0 && c.ll.Len() > c.maxEntries) ||
			(c.maxBytes > 0 && c.curBytes > c.maxBytes)) {
		c.removeLocked(c.ll.Back())
	}
}

// removeLocked drops el from both the list and the index and updates the byte
// total. The caller must hold c.mu.
func (c *cache) removeLocked(el *list.Element) {
	e := el.Value.(*cacheEntry)
	c.ll.Remove(el)
	delete(c.items, e.key)
	c.curBytes -= int64(len(e.body))
}
