package cache

import (
	"encoding/hex"
	"hash/fnv"
	"log"
	"sync"
	"time"
)

type ResponseStatus string

const (
	StatusCompleted ResponseStatus = "completed"
	StatusCanceled  ResponseStatus = "canceled"
	StatusError     ResponseStatus = "error"
	StatusPending   ResponseStatus = "pending"
)

type CachedResponse struct {
	Text     string         `json:"text"`
	Status   ResponseStatus `json:"status"`
	CachedAt time.Time      `json:"cached_at"`
}

type Item struct {
	V   any
	Exp int64
}

type Cache struct {
	mu    sync.RWMutex
	items map[string]Item
}

var (
	defaultCache *Cache
	once         sync.Once
)

func Default() *Cache {
	once.Do(func() {
		defaultCache = &Cache{items: make(map[string]Item)}
		go defaultCache.janitor(60 * time.Second)
	})
	return defaultCache
}

func (c *Cache) Get(key string) (any, bool) {
	if c == nil {
		return nil, false
	}
	now := time.Now().Unix()
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if it.Exp != 0 && it.Exp < now {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}
	return it.V, true
}

func (c *Cache) Set(key string, v any, ttl time.Duration) {
	if c == nil {
		return
	}
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).Unix()
	}
	c.mu.Lock()
	c.items[key] = Item{V: v, Exp: exp}
	c.mu.Unlock()
}

func (c *Cache) Delete(key string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

func (c *Cache) janitor(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		now := time.Now().Unix()
		c.mu.Lock()
		for k, it := range c.items {
			if it.Exp != 0 && it.Exp < now {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

func KeyFromStrings(parts ...string) string {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(p))
	}
	return string(h.Sum(nil))
}

func (c *Cache) SetChatResponse(key string, text string, status ResponseStatus, ttl time.Duration) {
	if status == StatusCompleted && text != "" && text != "Maaf, belum ada jawaban." {
		response := CachedResponse{
			Text:     text,
			Status:   status,
			CachedAt: time.Now(),
		}
		c.Set(key, response, ttl)
		log.Printf("[cache] Cache SAVED: key=%s, status=%s, text_length=%d, ttl=%v",
			shortenKey(key), status, len(text), ttl)
	} else {
		log.Printf("[cache] Cache SKIPPED: key=%s, status=%s, text_length=%d (not caching incomplete/error responses)",
			shortenKey(key), status, len(text))
	}
}

func (c *Cache) GetChatResponse(key string) (string, bool) {
	text, found, _ := c.GetChatResponseWithInfo(key)
	return text, found
}

func (c *Cache) GetChatResponseWithInfo(key string) (string, bool, *CachedResponse) {
	v, ok := c.Get(key)
	if !ok {
		return "", false, nil
	}

	switch resp := v.(type) {
	case string:
		if resp != "" && resp != "Maaf, belum ada jawaban." {
			log.Printf("[cache] Cache HIT (legacy format): key=%s, text_length=%d", shortenKey(key), len(resp))
			return resp, true, &CachedResponse{
				Text:     resp,
				Status:   StatusCompleted,
				CachedAt: time.Now(),
			}
		}
		return "", false, nil
	case CachedResponse:
		if resp.Status == StatusCompleted && resp.Text != "" && resp.Text != "Maaf, belum ada jawaban." {
			log.Printf("[cache] Cache HIT: key=%s, status=%s, text_length=%d, cached_at=%s",
				shortenKey(key), resp.Status, len(resp.Text), resp.CachedAt.Format("15:04:05"))
			return resp.Text, true, &resp
		}
		return "", false, nil
	default:
		return "", false, nil
	}
}

func shortenKey(key string) string {
	hexKey := hex.EncodeToString([]byte(key))
	if len(hexKey) <= 16 {
		return hexKey
	}
	return hexKey[:8] + "..." + hexKey[len(hexKey)-8:]
}

func (c *Cache) InvalidateChatResponse(key string) {
	if _, exists := c.Get(key); exists {
		log.Printf("[cache] Cache INVALIDATED: key=%s (canceled/failed request)", shortenKey(key))
	}
	c.Delete(key)
}
