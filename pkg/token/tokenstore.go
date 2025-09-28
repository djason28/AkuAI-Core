package tokenstore

import "sync"

// in-memory token revocation store. For production use Redis or DB.
var (
	mu            sync.RWMutex
	revokedTokens = map[string]struct{}{}
)

func RevokeToken(jti string) {
	if jti == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	revokedTokens[jti] = struct{}{}
}

func IsRevoked(jti string) bool {
	if jti == "" {
		return false
	}
	mu.RLock()
	defer mu.RUnlock()
	_, ok := revokedTokens[jti]
	return ok
}
