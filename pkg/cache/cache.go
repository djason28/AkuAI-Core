package cache

import (
	"container/list"
	"hash/fnv"
	"sync"
	"time"
)

// Item represents a cached value with expiration time.
type Item struct {
	V   any
	Exp int64 // unix seconds; 0 = no expiry
}

// Cache is a simple in-memory TTL cache safe for concurrent use.
type Cache struct {
	mu       sync.RWMutex
	items    map[string]*entry
	order    *list.List       // MRU at front, LRU at back
	maxItems int              // 0 = unlimited
}

type entry struct {
	key  string
	item Item
	elem *list.Element
}

var (
	defaultCache *Cache
	once         sync.Once
	defaultMax   = 500
)

// Default returns a process-wide cache instance.
func Default() *Cache {
	once.Do(func() {
		defaultCache = &Cache{items: make(map[string]*entry), order: list.New(), maxItems: defaultMax}
		// start janitor
		go defaultCache.janitor(60 * time.Second)
	})
	return defaultCache
}

// Get returns value and whether it exists and not expired.
func (c *Cache) Get(key string) (any, bool) {
	if c == nil {
		return nil, false
	}
	now := time.Now().Unix()
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if e.item.Exp != 0 && e.item.Exp < now {
		// lazy delete
		c.mu.Lock()
		c.removeNoLock(key)
		c.mu.Unlock()
		return nil, false
	}
	// move to front (MRU)
	c.mu.Lock()
	if e.elem != nil {
		c.order.MoveToFront(e.elem)
	}
	c.mu.Unlock()
	return e.item.V, true
}

// Set sets a value with TTL. ttl<=0 means no expiry.
func (c *Cache) Set(key string, v any, ttl time.Duration) {
	if c == nil {
		return
	}
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).Unix()
	}
	c.mu.Lock()
	if e, ok := c.items[key]; ok {
		e.item = Item{V: v, Exp: exp}
		if e.elem != nil {
			c.order.MoveToFront(e.elem)
		}
	} else {
		e := &entry{key: key, item: Item{V: v, Exp: exp}}
		e.elem = c.order.PushFront(e)
		c.items[key] = e
		// enforce capacity
		if c.maxItems > 0 && c.order.Len() > c.maxItems {
			c.evictLRUNoLock()
		}
	}
	c.mu.Unlock()
}

// Delete removes a key.
func (c *Cache) Delete(key string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.removeNoLock(key)
	c.mu.Unlock()
}

// janitor periodically removes expired items.
func (c *Cache) janitor(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		now := time.Now().Unix()
		c.mu.Lock()
		for k, e := range c.items {
			if e.item.Exp != 0 && e.item.Exp < now {
				c.removeNoLock(k)
			}
		}
		c.mu.Unlock()
	}
}

// KeyFromStrings creates a compact stable key from parts.
func KeyFromStrings(parts ...string) string {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(p))
	}
	return string(h.Sum(nil))
}

// SetMaxItems updates capacity for the default cache. Safe to call at startup.
func SetMaxItems(n int) {
	if n <= 0 {
		n = 0 // unlimited
	}
	c := Default()
	c.mu.Lock()
	c.maxItems = n
	// Trim if needed
	for c.maxItems > 0 && c.order.Len() > c.maxItems {
		c.evictLRUNoLock()
	}
	c.mu.Unlock()
}

// removeNoLock removes key from map/list; caller must hold c.mu.
func (c *Cache) removeNoLock(key string) {
	if e, ok := c.items[key]; ok {
		if e.elem != nil {
			c.order.Remove(e.elem)
		}
		delete(c.items, key)
	}
}

// evictLRUNoLock removes one LRU entry; caller must hold c.mu.
func (c *Cache) evictLRUNoLock() {
	back := c.order.Back()
	if back == nil {
		return
	}
	if e, ok := back.Value.(*entry); ok {
		c.order.Remove(back)
		delete(c.items, e.key)
	} else {
		// fallback safety: remove the element regardless
		c.order.Remove(back)
	}
}
