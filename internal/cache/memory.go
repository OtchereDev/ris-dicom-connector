package cache

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MemoryCache implements Cache interface using in-memory storage
type MemoryCache struct {
	mu   sync.RWMutex
	data map[string]*cacheItem
	done chan struct{}
}

type cacheItem struct {
	value      []byte
	expiration time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache() *MemoryCache {
	mc := &MemoryCache{
		data: make(map[string]*cacheItem),
		done: make(chan struct{}),
	}

	// Start cleanup goroutine
	go mc.cleanup()

	return mc
}

// Get retrieves a value from cache
func (m *MemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.data[key]
	if !exists {
		return nil, ErrCacheMiss
	}

	if time.Now().After(item.expiration) {
		return nil, ErrCacheMiss
	}

	return item.value, nil
}

// Set stores a value in cache
func (m *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = &cacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}

	return nil
}

// Delete removes a value from cache
func (m *MemoryCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	return nil
}

// Exists checks if a key exists
func (m *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.data[key]
	if !exists {
		return false, nil
	}

	if time.Now().After(item.expiration) {
		return false, nil
	}

	return true, nil
}

// Clear removes all keys matching pattern
func (m *MemoryCache) Clear(ctx context.Context, pattern string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simple pattern matching (only supports * wildcard)
	for key := range m.data {
		if matchPattern(key, pattern) {
			delete(m.data, key)
		}
	}

	return nil
}

// cleanup periodically removes expired items
func (m *MemoryCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.mu.Lock()
			now := time.Now()
			for key, item := range m.data {
				if now.After(item.expiration) {
					delete(m.data, key)
				}
			}
			m.mu.Unlock()
		case <-m.done:
			return
		}
	}
}

// Close closes the memory cache
func (m *MemoryCache) Close() error {
	close(m.done)
	return nil
}

// matchPattern performs simple pattern matching
func matchPattern(s, pattern string) bool {
	// Simple wildcard matching
	if pattern == "*" {
		return true
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(s, prefix)
	}

	return s == pattern
}
