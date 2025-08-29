package cache

import (
	"context"
	"time"
)

// Cache 定义了缓存行为的接口。
// testkit中的所有缓存实现（如MemoryCache, LayeredCache）都遵循此接口。
type Cache interface {
	// Get 从缓存中获取一个值。
	Get(ctx context.Context, key string) (interface{}, error)
	// Set 向缓存中设置一个值，可以指定TTL（生存时间）。
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	// Delete 从缓存中删除一个值。
	Delete(ctx context.Context, key string) error
	// Clear 清空所有缓存条目。
	Clear(ctx context.Context) error
	// Stats 获取缓存的统计信息。
	Stats() CacheStats
}

// CacheEntry 代表缓存中的一个条目。
type CacheEntry struct {
	Value      interface{} // 缓存的值
	ExpireTime time.Time   // 过期时间
	AccessTime time.Time   // 最后访问时间
	CreateTime time.Time   // 创建时间
	HitCount   int64       // 命中次数
	Size       int64       // 条目大小（字节）
}

// CacheStats 包含了缓存的详细统计信息。
type CacheStats struct {
	Size        int64         `json:"size"`         // 当前缓存中的条目数
	MaxSize     int64         `json:"max_size"`     // 缓存最大容量
	HitCount    int64         `json:"hit_count"`    // 命中次数
	MissCount   int64         `json:"miss_count"`   // 未命中次数
	HitRate     float64       `json:"hit_rate"`     // 命中率
	TTL         time.Duration `json:"ttl"`          // 默认的生存时间
	LastCleanup time.Time     `json:"last_cleanup"` // 最后一次清理过期条目的时间
}

// BatchGetter 批量获取接口
type BatchGetter interface {
	// BatchGet 批量从缓存中获取多个值
	BatchGet(ctx context.Context, keys []string) (map[string]any, error)
}

// BatchSetter 批量设置接口
type BatchSetter interface {
	// BatchSet 批量向缓存中设置多个值
	BatchSet(ctx context.Context, items map[string]any, ttl time.Duration) error
}
