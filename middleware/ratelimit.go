package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	tokens     int
	lastRefill time.Time
}

var (
	rlMu        sync.Mutex
	buckets     = map[string]*bucket{}
	window      = 10 * time.Second
	capacity    = 5
	refillPerWd = capacity

	dupMu   sync.Mutex
	lastMsg = map[string]struct {
		text string
		ts   time.Time
	}{}
	dupTTL = 45 * time.Second

	cgMu     sync.Mutex
	userSem  = map[string]chan struct{}{}
	userConc = 2
)

func SetRateLimitConfig(win time.Duration, cap, conc int) {
	rlMu.Lock()
	window = win
	capacity = cap
	refillPerWd = cap
	rlMu.Unlock()
	cgMu.Lock()
	userConc = conc
	cgMu.Unlock()
}

func SetDuplicateTTL(ttl time.Duration) {
	dupMu.Lock()
	dupTTL = ttl
	dupMu.Unlock()
}

func clientIP(c *gin.Context) string {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		host, _, _ := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
		ip = host
	}
	return ip
}

func userKey(c *gin.Context) string {
	uidRaw, _ := c.Get(ContextUserIDKey)
	uid, _ := uidRaw.(string)
	return uid + "@" + clientIP(c)
}

func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := userKey(c)
		now := time.Now()

		rlMu.Lock()
		b := buckets[key]
		if b == nil {
			b = &bucket{tokens: capacity, lastRefill: now}
			buckets[key] = b
		}
		elapsed := now.Sub(b.lastRefill)
		if elapsed > 0 {
			add := int(float64(refillPerWd) * (float64(elapsed) / float64(window)))
			if add > 0 {
				b.tokens += add
				if b.tokens > capacity {
					b.tokens = capacity
				}
				b.lastRefill = now
			}
		}
		if b.tokens <= 0 {
			rlMu.Unlock()
			c.Header("Retry-After", strconv.Itoa(int(window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"msg": "too many requests"})
			return
		}
		b.tokens--
		rlMu.Unlock()

		c.Next()
	}
}

func DuplicateGuard(uid string, text string) bool {
	now := time.Now()
	k := uid
	dupMu.Lock()
	entry, ok := lastMsg[k]
	if ok && entry.text == strings.TrimSpace(text) && now.Sub(entry.ts) < dupTTL {
		dupMu.Unlock()
		return false
	}
	lastMsg[k] = struct {
		text string
		ts   time.Time
	}{text: strings.TrimSpace(text), ts: now}
	dupMu.Unlock()
	return true
}

func AcquireUserSlot(uid string) (release func()) {
	cgMu.Lock()
	sem := userSem[uid]
	if sem == nil {
		sem = make(chan struct{}, userConc)
		userSem[uid] = sem
	}
	cgMu.Unlock()
	sem <- struct{}{}
	return func() { <-sem }
}
