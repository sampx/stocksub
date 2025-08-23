// Package cache 提供了 testkit 的缓存层实现，包括内存缓存、分层缓存和多种缓存淘汰策略。
package cache

import (
	"context"
	"time"
)

// Cache 定义了所有缓存实现都必须遵循的通用接口。
type Cache interface {
	// Get 根据键从缓存中检索一个条目。
	Get(ctx context.Context, key string) (interface{}, error)
	// Set 将一个键值对存入缓存，可以指定一个可选的TTL（生存时间）。
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	// Delete 从缓存中删除一个指定的键。
	Delete(ctx context.Context, key string) error
	// Clear 清空缓存中的所有条目。
	Clear(ctx context.Context) error
	// Stats 返回当前缓存的统计信息。
	Stats() CacheStats
}

// CacheStats 包含了缓存实现的详细统计数据。
type CacheStats struct {
	Size        int64         `json:"size"`         // 当前缓存中的条目数。
	MaxSize     int64         `json:"max_size"`      // 缓存配置的最大容量。
	HitCount    int64         `json:"hit_count"`     // 缓存命中次数。
	MissCount   int64         `json:"miss_count"`    // 缓存未命中次数。
	HitRate     float64       `json:"hit_rate"`      // 缓存命中率 (HitCount / (HitCount + MissCount))。
	TTL         time.Duration `json:"ttl"`          // 缓存的默认TTL。
	LastCleanup time.Time     `json:"last_cleanup"`  // 最后一次执行清理操作的时间。
}

