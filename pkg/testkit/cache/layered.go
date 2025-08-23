package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stocksub/pkg/testkit/core"
)

// LayerType 缓存层类型
type LayerType string

const (
	LayerMemory LayerType = "memory" // 内存层
	LayerDisk   LayerType = "disk"   // 磁盘层
	LayerRemote LayerType = "remote" // 远程层（如Redis）
)

// LayerConfig 缓存层配置
type LayerConfig struct {
	Type            LayerType     `yaml:"type"`
	MaxSize         int64         `yaml:"max_size"`
	TTL             time.Duration `yaml:"ttl"`
	Enabled         bool          `yaml:"enabled"`
	Policy          PolicyType    `yaml:"policy"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

// LayeredCacheConfig 分层缓存配置
type LayeredCacheConfig struct {
	Layers         []LayerConfig `yaml:"layers"`
	PromoteEnabled bool          `yaml:"promote_enabled"` // 是否启用数据提升
	WriteThrough   bool          `yaml:"write_through"`   // 是否写穿透
	WriteBack      bool          `yaml:"write_back"`      // 是否写回
}

// LayeredCache 分层缓存实现
type LayeredCache struct {
	mu     sync.RWMutex
	layers []core.Cache
	config LayeredCacheConfig
	stats  LayeredCacheStats
}

// LayeredCacheStats 分层缓存统计
type LayeredCacheStats struct {
	LayerStats   []core.CacheStats `json:"layer_stats"`
	TotalHits    int64        `json:"total_hits"`
	TotalMisses  int64        `json:"total_misses"`
	PromoteCount int64        `json:"promote_count"`
	WriteThrough int64        `json:"write_through"`
	WriteBack    int64        `json:"write_back"`
}

// NewLayeredCache 创建分层缓存
func NewLayeredCache(config LayeredCacheConfig) (*LayeredCache, error) {
	layers := make([]core.Cache, 0, len(config.Layers))

	for i, layerConfig := range config.Layers {
		if !layerConfig.Enabled {
			continue
		}

		layer, err := createCacheLayer(layerConfig, i)
		if err != nil {
			return nil, fmt.Errorf("创建缓存层 %d 失败: %w", i, err)
		}

		layers = append(layers, layer)
	}

	if len(layers) == 0 {
		return nil, fmt.Errorf("至少需要一个启用的缓存层")
	}

	return &LayeredCache{
		layers: layers,
		config: config,
		stats: LayeredCacheStats{
			LayerStats: make([]core.CacheStats, len(layers)),
		},
	}, nil
}

// createCacheLayer 创建单个缓存层
func createCacheLayer(config LayerConfig, layerIndex int) (core.Cache, error) {
	switch config.Type {
	case LayerMemory:
		memConfig := MemoryCacheConfig{
			MaxSize:         config.MaxSize,
			DefaultTTL:      config.TTL,
			CleanupInterval: config.CleanupInterval,
		}

		if config.Policy != "" {
			policyConfig := PolicyConfig{
				Type:    config.Policy,
				MaxSize: config.MaxSize,
				TTL:     config.TTL,
			}
			return NewSmartCache(memConfig, policyConfig), nil
		}

		return NewMemoryCache(memConfig), nil

	case LayerDisk:
		// TODO: 实现磁盘缓存层
		return NewMemoryCache(MemoryCacheConfig{
			MaxSize:         config.MaxSize,
			DefaultTTL:      config.TTL,
			CleanupInterval: config.CleanupInterval,
		}), nil

	case LayerRemote:
		// TODO: 实现远程缓存层（如Redis）
		return NewMemoryCache(MemoryCacheConfig{
			MaxSize:         config.MaxSize,
			DefaultTTL:      config.TTL,
			CleanupInterval: config.CleanupInterval,
		}), nil

	default:
		return nil, fmt.Errorf("不支持的缓存层类型: %s", config.Type)
	}
}

// Get 从分层缓存获取数据
func (lc *LayeredCache) Get(ctx context.Context, key string) (interface{}, error) {
	for i, layer := range lc.layers {
		value, err := layer.Get(ctx, key)
		if err == nil {
			// 缓存命中，检查是否需要数据提升
			if lc.config.PromoteEnabled && i > 0 {
				go lc.promoteToUpperLayers(ctx, key, value, i)
				lc.stats.PromoteCount++
			}
			lc.stats.TotalHits++
			return value, nil
		}

		// 如果不是缓存未命中错误，返回错误
		if err != core.NewTestKitError(core.ErrCacheMiss, "cache miss") {
			return nil, fmt.Errorf("缓存层 %d 错误: %w", i, err)
		}
	}

	lc.stats.TotalMisses++
	return nil, core.NewTestKitError(core.ErrCacheMiss, "cache miss")
}

// Set 向分层缓存设置数据
func (lc *LayeredCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if lc.config.WriteThrough {
		// 写穿透：向所有层写入
		var lastErr error
		for i, layer := range lc.layers {
			if err := layer.Set(ctx, key, value, ttl); err != nil {
				lastErr = fmt.Errorf("缓存层 %d 写入失败: %w", i, err)
			}
		}
		if lastErr == nil {
			lc.stats.WriteThrough++
		}
		return lastErr
	} else {
		// 默认只写入第一层（最快的层）
		if len(lc.layers) > 0 {
			return lc.layers[0].Set(ctx, key, value, ttl)
		}
		return fmt.Errorf("没有可用的缓存层")
	}
}

// Delete 从分层缓存删除数据
func (lc *LayeredCache) Delete(ctx context.Context, key string) error {
	var lastErr error

	// 从所有层删除
	for i, layer := range lc.layers {
		if err := layer.Delete(ctx, key); err != nil {
			lastErr = fmt.Errorf("缓存层 %d 删除失败: %w", i, err)
		}
	}

	return lastErr
}

// Clear 清空所有缓存层
func (lc *LayeredCache) Clear(ctx context.Context) error {
	var lastErr error

	for i, layer := range lc.layers {
		if err := layer.Clear(ctx); err != nil {
			lastErr = fmt.Errorf("缓存层 %d 清空失败: %w", i, err)
		}
	}

	// 重置统计信息
	lc.stats = LayeredCacheStats{
		LayerStats: make([]core.CacheStats, len(lc.layers)),
	}

	return lastErr
}

// Stats 获取分层缓存统计信息
func (lc *LayeredCache) Stats() core.CacheStats {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	// 收集各层统计信息
	totalSize := int64(0)
	totalMaxSize := int64(0)
	totalHitCount := lc.stats.TotalHits
	totalMissCount := lc.stats.TotalMisses

	for i, layer := range lc.layers {
		layerStats := layer.Stats()
		lc.stats.LayerStats[i] = layerStats

		totalSize += layerStats.Size
		totalMaxSize += layerStats.MaxSize
	}

	var hitRate float64
	if total := totalHitCount + totalMissCount; total > 0 {
		hitRate = float64(totalHitCount) / float64(total)
	}

	return core.CacheStats{
		Size:        totalSize,
		MaxSize:     totalMaxSize,
		HitCount:    totalHitCount,
		MissCount:   totalMissCount,
		HitRate:     hitRate,
		TTL:         0, // 分层缓存的TTL取决于各层配置
		LastCleanup: time.Now(),
	}
}

// promoteToUpperLayers 将数据提升到上层缓存
func (lc *LayeredCache) promoteToUpperLayers(ctx context.Context, key string, value interface{}, fromLayer int) {
	// 从命中层的上一层开始，逐层向上提升
	for i := fromLayer - 1; i >= 0; i-- {
		// 使用各层的默认TTL
		if err := lc.layers[i].Set(ctx, key, value, 0); err != nil {
			// 记录错误但不中断提升过程
			continue
		}
	}
}

// GetLayerStats 获取各层统计信息
func (lc *LayeredCache) GetLayerStats() LayeredCacheStats {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	// 更新各层统计信息
	for i, layer := range lc.layers {
		lc.stats.LayerStats[i] = layer.Stats()
	}

	return lc.stats
}

// Close 关闭所有缓存层
func (lc *LayeredCache) Close() error {
	var lastErr error

	for i, layer := range lc.layers {
		if closer, ok := layer.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				lastErr = fmt.Errorf("缓存层 %d 关闭失败: %w", i, err)
			}
		}
	}

	return lastErr
}

// Warm 预热缓存
func (lc *LayeredCache) Warm(ctx context.Context, data map[string]interface{}) error {
	for key, value := range data {
		if err := lc.Set(ctx, key, value, 0); err != nil {
			return fmt.Errorf("预热缓存失败，key=%s: %w", key, err)
		}
	}
	return nil
}

// Flush 刷新缓存（将上层数据写入下层）
func (lc *LayeredCache) Flush(ctx context.Context) error {
	if !lc.config.WriteBack {
		return nil // 只在写回模式下执行刷新
	}

	// TODO: 实现写回逻辑
	// 这需要缓存层支持遍历所有键值对
	lc.stats.WriteBack++
	return nil
}

// DefaultLayeredCacheConfig 默认分层缓存配置
func DefaultLayeredCacheConfig() LayeredCacheConfig {
	return LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:            LayerMemory,
				MaxSize:         1000,
				TTL:             5 * time.Minute,
				Enabled:         true,
				Policy:          PolicyLRU,
				CleanupInterval: 1 * time.Minute,
			},
			{
				Type:            LayerMemory, // 作为二级缓存
				MaxSize:         5000,
				TTL:             30 * time.Minute,
				Enabled:         true,
				Policy:          PolicyLFU,
				CleanupInterval: 5 * time.Minute,
			},
		},
		PromoteEnabled: true,
		WriteThrough:   false,
		WriteBack:      false,
	}
}

var _ core.Cache = (*LayeredCache)(nil)