package cache

import (
	"context"
	"time"
)

// Cache defines the cache interface
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context, pattern string) error
}

// CacheKey generates a cache key
func CacheKey(tenantID, studyUID, seriesUID, instanceUID, suffix string) string {
	if instanceUID != "" {
		return tenantID + ":" + studyUID + ":" + seriesUID + ":" + instanceUID + ":" + suffix
	}
	if seriesUID != "" {
		return tenantID + ":" + studyUID + ":" + seriesUID + ":" + suffix
	}
	return tenantID + ":" + studyUID + ":" + suffix
}
