package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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

// LayerFactory 缓存层工厂接口
type LayerFactory interface {
	// CreateLayer 根据配置创建缓存层
	CreateLayer(config LayerConfig, layerIndex int) (core.Cache, error)
	// LayerType 返回支持的缓存层类型
	LayerType() LayerType
}

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
	mu          sync.RWMutex
	layers      []core.Cache
	config      LayeredCacheConfig
	stats       LayeredCacheStats
	factories   map[LayerType]LayerFactory // 缓存层工厂注册表
	promoteChan chan promoteRequest        // 数据提升请求通道
	closed      bool                       // 缓存是否已关闭
}

// promoteRequest 数据提升请求
type promoteRequest struct {
	ctx        context.Context
	key        string
	value      interface{}
	fromLayer  int
	errChan    chan error
}

// LayeredCacheStats 分层缓存统计
type LayeredCacheStats struct {
	LayerStats   []core.CacheStats `json:"layer_stats"`
	TotalHits    int64             `json:"total_hits"`
	TotalMisses  int64             `json:"total_misses"`
	PromoteCount int64             `json:"promote_count"`
	WriteThrough int64             `json:"write_through"`
	WriteBack    int64             `json:"write_back"`
}

// NewLayeredCache 创建分层缓存
func NewLayeredCache(config LayeredCacheConfig) (*LayeredCache, error) {
	return NewLayeredCacheWithFactories(config, nil)
}

// NewLayeredCacheWithFactories 使用指定的工厂创建分层缓存
func NewLayeredCacheWithFactories(config LayeredCacheConfig, customFactories map[LayerType]LayerFactory) (*LayeredCache, error) {
	layers := make([]core.Cache, 0, len(config.Layers))
	
	// 初始化默认工厂注册表
	factories := make(map[LayerType]LayerFactory)
	if customFactories != nil {
		for layerType, factory := range customFactories {
			factories[layerType] = factory
		}
	}
	
	// 注册默认工厂
	registerDefaultFactories(factories)

	for i, layerConfig := range config.Layers {
		if !layerConfig.Enabled {
			continue
		}

		layer, err := createCacheLayer(layerConfig, i, factories)
		if err != nil {
			return nil, fmt.Errorf("创建缓存层 %d 失败: %w", i, err)
		}

		layers = append(layers, layer)
	}

	if len(layers) == 0 {
		return nil, fmt.Errorf("至少需要一个启用的缓存层")
	}

	lc := &LayeredCache{
		layers:    layers,
		config:    config,
		factories: factories,
		stats: LayeredCacheStats{
			LayerStats: make([]core.CacheStats, len(layers)),
		},
		promoteChan: make(chan promoteRequest, 100), // 缓冲通道避免阻塞
	}
	
	// 启动数据提升工作协程
	if config.PromoteEnabled {
		go lc.promoteWorker()
	}

	return lc, nil
}

// createCacheLayer 创建单个缓存层
func createCacheLayer(config LayerConfig, layerIndex int, factories map[LayerType]LayerFactory) (core.Cache, error) {
	// 为调试和监控目的，可以根据层索引进行特殊处理
	// 例如：为不同层设置不同的标识或配置

	factory, exists := factories[config.Type]
	if !exists {
		return nil, fmt.Errorf("不支持的缓存层类型: %s (层索引: %d)", config.Type, layerIndex)
	}

	return factory.CreateLayer(config, layerIndex)
}

// registerDefaultFactories 注册默认的缓存层工厂
func registerDefaultFactories(factories map[LayerType]LayerFactory) {
	if _, exists := factories[LayerMemory]; !exists {
		factories[LayerMemory] = &memoryLayerFactory{}
	}
	if _, exists := factories[LayerDisk]; !exists {
		factories[LayerDisk] = &diskLayerFactory{}
	}
	if _, exists := factories[LayerRemote]; !exists {
		factories[LayerRemote] = &remoteLayerFactory{}
	}
}

// Get 从分层缓存获取数据
func (lc *LayeredCache) Get(ctx context.Context, key string) (interface{}, error) {
	lc.mu.RLock()
	if lc.closed {
		lc.mu.RUnlock()
		return nil, fmt.Errorf("缓存已关闭")
	}
	lc.mu.RUnlock()
	
	for i, layer := range lc.layers {
		value, err := layer.Get(ctx, key)
		if err == nil {
			// 缓存命中，检查是否需要数据提升
			if lc.config.PromoteEnabled && i > 0 {
				lc.asyncPromoteToUpperLayers(ctx, key, value, i)
			}
			atomic.AddInt64(&lc.stats.TotalHits, 1)
			return value, nil
		}

		// 如果不是缓存未命中错误，返回错误
		// 使用错误码比较而不是实例比较
		var testKitErr *core.TestKitError
		if errors.As(err, &testKitErr) {
			if testKitErr.Code != core.ErrCacheMiss {
				return nil, fmt.Errorf("缓存层 %d (%s) 错误: %w", i, lc.getLayerType(i), err)
			}
		} else {
			// 如果不是TestKitError，直接返回
			return nil, fmt.Errorf("缓存层 %d (%s) 错误: %w", i, lc.getLayerType(i), err)
		}
	}

	atomic.AddInt64(&lc.stats.TotalMisses, 1)
	return nil, core.NewTestKitError(core.ErrCacheMiss, "cache miss")
}

// getLayerType 获取缓存层类型
func (lc *LayeredCache) getLayerType(index int) string {
	if index < 0 || index >= len(lc.layers) {
		return "unknown"
	}
	
	// 通过反射或其他方式获取实际类型
	// 这里简化处理，返回基本类型信息
	switch lc.layers[index].(type) {
	case *MemoryCache, *SmartCache:
		return "memory"
	case *DiskCache:
		return "disk"
	case interface{ IsConnected() bool }: // 检查是否是RemoteCache
		return "remote"
	default:
		return "unknown"
	}
}

// asyncPromoteToUpperLayers 异步将数据提升到上层缓存
func (lc *LayeredCache) asyncPromoteToUpperLayers(ctx context.Context, key string, value interface{}, fromLayer int) {
	if !lc.config.PromoteEnabled {
		return
	}

	// 使用缓冲通道避免阻塞
	select {
	case lc.promoteChan <- promoteRequest{
		ctx:       ctx,
		key:       key,
		value:     value,
		fromLayer: fromLayer,
		errChan:   nil, // 不需要错误返回
	}:
		atomic.AddInt64(&lc.stats.PromoteCount, 1)
	default:
		// 通道满时丢弃提升请求，避免阻塞
	}
}

// promoteWorker 数据提升工作协程
func (lc *LayeredCache) promoteWorker() {
	for req := range lc.promoteChan {
		lc.promoteToUpperLayers(req.ctx, req.key, req.value, req.fromLayer)
		if req.errChan != nil {
			req.errChan <- nil
		}
	}
}

// Set 向分层缓存设置数据
func (lc *LayeredCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	lc.mu.RLock()
	if lc.closed {
		lc.mu.RUnlock()
		return fmt.Errorf("缓存已关闭")
	}
	lc.mu.RUnlock()
	
	if lc.config.WriteThrough {
		// 写穿透：向所有层写入
		var lastErr error
		for i, layer := range lc.layers {
			if err := layer.Set(ctx, key, value, ttl); err != nil {
				layerType := lc.getLayerType(i)
				lastErr = fmt.Errorf("缓存层 %d (%s) 写入失败: %w", i, layerType, err)
			}
		}
		if lastErr == nil {
			atomic.AddInt64(&lc.stats.WriteThrough, 1)
		}
		return lastErr
	} else {
		// 默认只写入第一层（最快的层）
		if len(lc.layers) > 0 {
			if err := lc.layers[0].Set(ctx, key, value, ttl); err != nil {
				return fmt.Errorf("第一层缓存 (%s) 写入失败: %w", lc.getLayerType(0), err)
			}
			return nil
		}
		return fmt.Errorf("没有可用的缓存层")
	}
}

// Delete 从分层缓存删除数据
func (lc *LayeredCache) Delete(ctx context.Context, key string) error {
	lc.mu.RLock()
	if lc.closed {
		lc.mu.RUnlock()
		return fmt.Errorf("缓存已关闭")
	}
	lc.mu.RUnlock()
	
	var lastErr error

	// 从所有层删除
	for i, layer := range lc.layers {
		if err := layer.Delete(ctx, key); err != nil {
			layerType := lc.getLayerType(i)
			lastErr = fmt.Errorf("缓存层 %d (%s) 删除失败: %w", i, layerType, err)
		}
	}

	return lastErr
}

// Clear 清空所有缓存层
func (lc *LayeredCache) Clear(ctx context.Context) error {
	lc.mu.RLock()
	if lc.closed {
		lc.mu.RUnlock()
		return fmt.Errorf("缓存已关闭")
	}
	lc.mu.RUnlock()
	
	var lastErr error

	for i, layer := range lc.layers {
		if err := layer.Clear(ctx); err != nil {
			layerType := lc.getLayerType(i)
			lastErr = fmt.Errorf("缓存层 %d (%s) 清空失败: %w", i, layerType, err)
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
	
	// 即使缓存已关闭，也允许获取统计信息

	// 收集各层统计信息
	totalSize := int64(0)
	totalMaxSize := int64(0)
	totalHitCount := atomic.LoadInt64(&lc.stats.TotalHits)
	totalMissCount := atomic.LoadInt64(&lc.stats.TotalMisses)

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
	
	// 即使缓存已关闭，也允许获取统计信息

	// 更新各层统计信息
	for i, layer := range lc.layers {
		lc.stats.LayerStats[i] = layer.Stats()
	}

	return lc.stats
}

// Close 关闭所有缓存层
func (lc *LayeredCache) Close() error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	if lc.closed {
		return nil // 已经关闭
	}

	var lastErr error

	// 关闭数据提升通道和工作协程
	if lc.promoteChan != nil {
		close(lc.promoteChan)
	}

	for i, layer := range lc.layers {
		if closer, ok := layer.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				lastErr = fmt.Errorf("缓存层 %d 关闭失败: %w", i, err)
			}
		}
	}

	lc.closed = true
	return lastErr
}

// BatchGet 批量从分层缓存获取数据（可选扩展功能）
func (lc *LayeredCache) BatchGet(ctx context.Context, keys []string) (map[string]any, error) {
	lc.mu.RLock()
	if lc.closed {
		lc.mu.RUnlock()
		return nil, fmt.Errorf("缓存已关闭")
	}
	lc.mu.RUnlock()
	
	result := make(map[string]any)
	remainingKeys := make([]string, len(keys))
	copy(remainingKeys, keys)

	// 逐层查询，直到所有键都找到或所有层都查询完毕
	for i, layer := range lc.layers {
		if len(remainingKeys) == 0 {
			break
		}

		// 检查当前层是否支持批量操作
		if batchGetter, ok := layer.(core.BatchGetter); ok {
			batchResult, err := batchGetter.BatchGet(ctx, remainingKeys)
			if err != nil {
				return nil, fmt.Errorf("缓存层 %d (%s) 批量获取失败: %w", i, lc.getLayerType(i), err)
			}

			// 收集结果并更新剩余键
			for key, value := range batchResult {
				result[key] = value
				// 从剩余键中移除已找到的键
				remainingKeys = removeKey(remainingKeys, key)
				
				// 检查是否需要数据提升
				if lc.config.PromoteEnabled && i > 0 {
					lc.asyncPromoteToUpperLayers(ctx, key, value, i)
				}
			}
		} else {
			// 当前层不支持批量操作，回退到单键操作
			for _, key := range remainingKeys {
				value, err := layer.Get(ctx, key)
				if err == nil {
					result[key] = value
					// 从剩余键中移除已找到的键
					remainingKeys = removeKey(remainingKeys, key)
					
					// 检查是否需要数据提升
					if lc.config.PromoteEnabled && i > 0 {
						lc.asyncPromoteToUpperLayers(ctx, key, value, i)
					}
				}
			}
		}
	}

	// 更新统计信息
	atomic.AddInt64(&lc.stats.TotalHits, int64(len(result)))
	atomic.AddInt64(&lc.stats.TotalMisses, int64(len(remainingKeys)))

	return result, nil
}

// BatchSet 批量向分层缓存设置数据
func (lc *LayeredCache) BatchSet(ctx context.Context, items map[string]any, ttl time.Duration) error {
	lc.mu.RLock()
	if lc.closed {
		lc.mu.RUnlock()
		return fmt.Errorf("缓存已关闭")
	}
	lc.mu.RUnlock()
	
	if lc.config.WriteThrough {
		// 写穿透：向所有层写入
		var lastErr error
		for i, layer := range lc.layers {
			if batchSetter, ok := layer.(core.BatchSetter); ok {
				if err := batchSetter.BatchSet(ctx, items, ttl); err != nil {
					layerType := lc.getLayerType(i)
					lastErr = fmt.Errorf("缓存层 %d (%s) 批量设置失败: %w", i, layerType, err)
				}
			} else {
				// 当前层不支持批量操作，回退到单键操作
				for key, value := range items {
					if err := layer.Set(ctx, key, value, ttl); err != nil {
						layerType := lc.getLayerType(i)
						lastErr = fmt.Errorf("缓存层 %d (%s) 设置失败: %w", i, layerType, err)
					}
				}
			}
		}
		if lastErr == nil {
			atomic.AddInt64(&lc.stats.WriteThrough, 1)
		}
		return lastErr
	} else {
		// 默认只写入第一层（最快的层）
		if len(lc.layers) > 0 {
			layer := lc.layers[0]
			if batchSetter, ok := layer.(core.BatchSetter); ok {
				if err := batchSetter.BatchSet(ctx, items, ttl); err != nil {
					return fmt.Errorf("第一层缓存 (%s) 批量设置失败: %w", lc.getLayerType(0), err)
				}
			} else {
				// 回退到单键操作
				for key, value := range items {
					if err := layer.Set(ctx, key, value, ttl); err != nil {
						return fmt.Errorf("第一层缓存 (%s) 设置失败: %w", lc.getLayerType(0), err)
					}
				}
			}
			return nil
		}
		return fmt.Errorf("没有可用的缓存层")
	}
}

// removeKey 从字符串切片中移除指定的键
func removeKey(keys []string, keyToRemove string) []string {
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if key != keyToRemove {
			result = append(result, key)
		}
	}
	return result
}

// Warm 预热缓存
func (lc *LayeredCache) Warm(ctx context.Context, data map[string]interface{}) error {
	lc.mu.RLock()
	if lc.closed {
		lc.mu.RUnlock()
		return fmt.Errorf("缓存已关闭")
	}
	lc.mu.RUnlock()
	
	for key, value := range data {
		if err := lc.Set(ctx, key, value, 0); err != nil {
			return fmt.Errorf("预热缓存失败，key=%s: %w", key, err)
		}
	}
	return nil
}

// Flush 刷新缓存（将上层数据写入下层）
func (lc *LayeredCache) Flush(ctx context.Context) error {
	lc.mu.RLock()
	if lc.closed {
		lc.mu.RUnlock()
		return fmt.Errorf("缓存已关闭")
	}
	lc.mu.RUnlock()
	
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

// 默认缓存层工厂实现

type memoryLayerFactory struct{}

func (f *memoryLayerFactory) LayerType() LayerType {
	return LayerMemory
}

func (f *memoryLayerFactory) CreateLayer(config LayerConfig, layerIndex int) (core.Cache, error) {
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
}

type diskLayerFactory struct{}

func (f *diskLayerFactory) LayerType() LayerType {
	return LayerDisk
}

func (f *diskLayerFactory) CreateLayer(config LayerConfig, layerIndex int) (core.Cache, error) {
	diskConfig := DiskCacheConfig{
		BaseDir:         "./cache_data", // 默认缓存目录
		MaxSize:         config.MaxSize,
		DefaultTTL:      config.TTL,
		CleanupInterval: config.CleanupInterval,
		FilePrefix:      fmt.Sprintf("layer_%d", layerIndex),
	}
	return NewDiskCache(diskConfig)
}

type remoteLayerFactory struct{}

func (f *remoteLayerFactory) LayerType() LayerType {
	return LayerRemote
}

func (f *remoteLayerFactory) CreateLayer(config LayerConfig, layerIndex int) (core.Cache, error) {
	remoteConfig := RemoteCacheConfig{
		Address:         "localhost:6379", // 默认Redis地址
		MaxSize:         config.MaxSize,
		DefaultTTL:      config.TTL,
		ConnectTimeout:  5 * time.Second,
		RequestTimeout:  2 * time.Second,
		MaxConnections:  10,
		PoolSize:        5,
	}
	
	// 使用模拟实现，实际项目中应该根据配置选择具体的远程缓存类型
	return NewMockRemoteCache(remoteConfig), nil
}
