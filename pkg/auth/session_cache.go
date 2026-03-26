package auth

import (
	"crypto/ed25519"
	"sync"
	"time"
)

// CachedSession holds a user's decrypted keys in memory.
type CachedSession struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
	ExpiresAt time.Time
}

// SessionCache is a thread-safe in-memory cache for user sessions.
type SessionCache struct {
	mu       sync.RWMutex
	sessions map[string]*CachedSession // userID → session
}

// NewSessionCache creates a new session cache.
func NewSessionCache() *SessionCache {
	return &SessionCache{
		sessions: make(map[string]*CachedSession),
	}
}

// Put stores a session for a user.
func (sc *SessionCache) Put(userID string, priv ed25519.PrivateKey, pub ed25519.PublicKey, ttl time.Duration) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.sessions[userID] = &CachedSession{
		PrivateKey: priv,
		PublicKey:  pub,
		ExpiresAt:  time.Now().Add(ttl),
	}
}

// Get retrieves a session, returning nil if not found or expired.
func (sc *SessionCache) Get(userID string) *CachedSession {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	s, ok := sc.sessions[userID]
	if !ok || time.Now().After(s.ExpiresAt) {
		return nil
	}
	return s
}

// Evict removes a specific user's session.
func (sc *SessionCache) Evict(userID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.sessions, userID)
}

// EvictExpired removes all expired sessions.
func (sc *SessionCache) EvictExpired() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	now := time.Now()
	for id, s := range sc.sessions {
		if now.After(s.ExpiresAt) {
			delete(sc.sessions, id)
		}
	}
}

// ActiveUserKeys returns the public keys of all users with active sessions.
func (sc *SessionCache) ActiveUserKeys() []ed25519.PublicKey {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	now := time.Now()
	var keys []ed25519.PublicKey
	for _, s := range sc.sessions {
		if now.Before(s.ExpiresAt) {
			keys = append(keys, s.PublicKey)
		}
	}
	return keys
}
