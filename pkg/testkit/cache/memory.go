package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"stocksub/pkg/testkit/core"
)

// MemoryCache 线程安全的内存缓存实现
type MemoryCache struct {
	mu         sync.RWMutex
	entries    map[string]*core.CacheEntry
	maxSize    int64
	hitCount   int64
	missCount  int64
	defaultTTL time.Duration

	// 清理相关
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
	lastCleanup   time.Time
}

// NewMemoryCache 创建新的内存缓存
func NewMemoryCache(config MemoryCacheConfig) *MemoryCache {
	cache := &MemoryCache{
		entries:     make(map[string]*core.CacheEntry),
		maxSize:     config.MaxSize,
		defaultTTL:  config.DefaultTTL,
		stopCleanup: make(chan struct{}),
		lastCleanup: time.Now(),
	}

	// 启动清理协程
	if config.CleanupInterval > 0 {
		cache.cleanupTicker = time.NewTicker(config.CleanupInterval)
		go cache.startCleanup()
	}

	return cache
}

// MemoryCacheConfig 内存缓存配置
type MemoryCacheConfig struct {
	MaxSize         int64         // 最大条目数量
	DefaultTTL      time.Duration // 默认TTL
	CleanupInterval time.Duration // 清理间隔
}

// Get 获取缓存值
func (mc *MemoryCache) Get(ctx context.Context, key string) (interface{}, error) {
	mc.mu.RLock()
	entry, exists := mc.entries[key]
	mc.mu.RUnlock()

	if !exists {
		atomic.AddInt64(&mc.missCount, 1)
		return nil, core.NewTestKitError(core.ErrCacheMiss, "cache miss")
	}

	// 检查过期
	if entry.ExpireTime.Before(time.Now()) {
		mc.mu.Lock()
		delete(mc.entries, key)
		mc.mu.Unlock()
		atomic.AddInt64(&mc.missCount, 1)
		return nil, core.NewTestKitError(core.ErrCacheMiss, "cache miss")
	}

	// 更新访问信息
	entry.AccessTime = time.Now()
	atomic.AddInt64(&entry.HitCount, 1)
	atomic.AddInt64(&mc.hitCount, 1)

	return entry.Value, nil
}

// Set 设置缓存值
func (mc *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = mc.defaultTTL
	}

	now := time.Now()
	entry := &core.CacheEntry{
		Value:      value,
		ExpireTime: now.Add(ttl),
		AccessTime: now,
		CreateTime: now,
		HitCount:   0,
		Size:       estimateSize(value),
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 检查是否需要淘汰
	if int64(len(mc.entries)) >= mc.maxSize {
		mc.evictOldest()
	}

	mc.entries[key] = entry
	return nil
}

// Delete 删除缓存值
func (mc *MemoryCache) Delete(ctx context.Context, key string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.entries, key)
	return nil
}

// Clear 清空缓存
func (mc *MemoryCache) Clear(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.entries = make(map[string]*core.CacheEntry)
	atomic.StoreInt64(&mc.hitCount, 0)
	atomic.StoreInt64(&mc.missCount, 0)
	return nil
}

// Stats 获取缓存统计信息
func (mc *MemoryCache) Stats() core.CacheStats {
	mc.mu.RLock()
	size := int64(len(mc.entries))
	mc.mu.RUnlock()

	hitCount := atomic.LoadInt64(&mc.hitCount)
	missCount := atomic.LoadInt64(&mc.missCount)

	var hitRate float64
	if total := hitCount + missCount; total > 0 {
		hitRate = float64(hitCount) / float64(total)
	}

	return core.CacheStats{
		Size:        size,
		MaxSize:     mc.maxSize,
		HitCount:    hitCount,
		MissCount:   missCount,
		HitRate:     hitRate,
		TTL:         mc.defaultTTL,
		LastCleanup: mc.lastCleanup,
	}
}

// Close 关闭缓存
func (mc *MemoryCache) Close() error {
	if mc.cleanupTicker != nil {
		mc.cleanupTicker.Stop()
	}
	close(mc.stopCleanup)
	return nil
}

// startCleanup 启动清理协程
func (mc *MemoryCache) startCleanup() {
	for {
		select {
		case <-mc.cleanupTicker.C:
			mc.cleanup()
		case <-mc.stopCleanup:
			return
		}
	}
}

// cleanup 清理过期条目
func (mc *MemoryCache) cleanup() {
	now := time.Now()
	expiredKeys := make([]string, 0)

	mc.mu.RLock()
	for key, entry := range mc.entries {
		if entry.ExpireTime.Before(now) {
			expiredKeys = append(expiredKeys, key)
		}
	}
	mc.mu.RUnlock()

	if len(expiredKeys) > 0 {
		mc.mu.Lock()
		for _, key := range expiredKeys {
			delete(mc.entries, key)
		}
		mc.lastCleanup = now
		mc.mu.Unlock()
	}
}

// evictOldest 淘汰创建时间最早的条目（基于创建时间的淘汰策略）
func (mc *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range mc.entries {
		if oldestKey == "" || entry.CreateTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreateTime
		}
	}

	if oldestKey != "" {
		delete(mc.entries, oldestKey)
	}
}

// estimateSize 估算值的大小（简单实现）
func estimateSize(value interface{}) int64 {
	// 简单的大小估算，可以根据需要改进
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	default:
		return 64 // 默认大小
	}
}

var _ core.Cache = (*MemoryCache)(nil)
