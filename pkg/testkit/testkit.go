// Package testkit 提供了用于支持测试的工具集，包括数据模拟、缓存、持久化存储等功能。
// 它是项目测试框架的核心，旨在提供一个与生产环境隔离、可预测且高效的测试数据环境。
package testkit

import (
	"context"
	"fmt"
	
	"sync"
	"time"

	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/cache"
	"stocksub/pkg/testkit/config"
	"stocksub/pkg/testkit/core"
	"stocksub/pkg/testkit/providers"
	"stocksub/pkg/testkit/storage"
)

// testDataManager 是 core.TestDataManager 接口的默认实现。
// 它整合了缓存、存储和数据提供者，为测试提供统一的数据访问入口。
type testDataManager struct {
	config       *config.Config
	cache        core.Cache
	storage      core.Storage
	provider     core.Provider
	cacheEnabled bool
	sessionID    string
	stats        *enhancedStats
	mu           sync.RWMutex
}

// enhancedStats 包含了 testDataManager 内部的详细统计信息。
type enhancedStats struct {
	cacheHits     int64
	cacheMisses   int64
	apiCalls      int64
	storageWrites int64
	storageReads  int64
	mockCalls     int64
	lastActivity  time.Time
	mutex         sync.RWMutex
}

// NewTestDataManager 是 testkit 的主要入口点，用于创建一个新的测试数据管理器实例。
// 它会根据传入的配置自动初始化多层缓存、持久化存储和数据提供者。
//
// 参数:
//   - cfg: 指向 config.Config 的指针，用于配置管理器的所有行为，
//          包括缓存策略、存储类型和目录等。
//
// 返回:
//   - 一个实现了 core.TestDataManager 接口的实例。
func NewTestDataManager(cfg *config.Config) core.TestDataManager {
	// 创建缓存层
	var cacheLayer core.Cache
	if cfg.Cache.Type == "layered" {
		layeredConfig := cache.DefaultLayeredCacheConfig()
		layeredCache, err := cache.NewLayeredCache(layeredConfig)
		if err != nil {
			// 回退到简单内存缓存
			memConfig := cache.MemoryCacheConfig{
				MaxSize:         cfg.Cache.MaxSize,
				DefaultTTL:      cfg.Cache.TTL,
				CleanupInterval: 5 * time.Minute,
			}
			cacheLayer = cache.NewMemoryCache(memConfig)
		} else {
			cacheLayer = layeredCache
		}
	} else {
		memConfig := cache.MemoryCacheConfig{
			MaxSize:         cfg.Cache.MaxSize,
			DefaultTTL:      cfg.Cache.TTL,
			CleanupInterval: 5 * time.Minute,
		}
		cacheLayer = cache.NewMemoryCache(memConfig)
	}

	// 创建存储层
	var storageLayer core.Storage
	if cfg.Storage.Type == "csv" {
		csvConfig := storage.DefaultCSVStorageConfig()
		csvConfig.Directory = cfg.Storage.Directory
		csvStorage, err := storage.NewCSVStorage(csvConfig)
		if err != nil {
			// 回退到内存存储
			memStorageConfig := storage.DefaultMemoryStorageConfig()
			storageLayer = storage.NewMemoryStorage(memStorageConfig)
		} else {
			storageLayer = csvStorage
		}
	} else {
		memStorageConfig := storage.DefaultMemoryStorageConfig()
		storageLayer = storage.NewMemoryStorage(memStorageConfig)
	}

	// 创建Provider层
	providerFactory := providers.NewProviderFactory(cacheLayer)
	cachedProviderConfig := providers.DefaultCachedProviderConfig()
	providerLayer := providerFactory.CreateCachedProvider(cachedProviderConfig)

	return &testDataManager{
		config:       cfg,
		cache:        cacheLayer,
		storage:      storageLayer,
		provider:     providerLayer,
		cacheEnabled: true,
		sessionID:    generateSessionID(),
		stats:        &enhancedStats{lastActivity: time.Now()},
	}
}

// GetStockData 实现了 core.TestDataManager 接口的 GetStockData 方法。
func (tdm *testDataManager) GetStockData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	startTime := time.Now()
	defer func() {
		tdm.stats.mutex.Lock()
		tdm.stats.lastActivity = time.Now()
		tdm.stats.mutex.Unlock()
	}()

	// 1. 检查顶层缓存
	cacheKey := tdm.generateCacheKey(symbols)
	if tdm.cacheEnabled {
		if cachedData, err := tdm.cache.Get(ctx, cacheKey); err == nil {
			tdm.updateCacheHit()
			fmt.Printf("🎯 TestDataManager 缓存命中，股票: %v\n", symbols)
			return cachedData.([]subscriber.StockData), nil
		}
	}

	// 2. 缓存未命中，通过Provider获取
	tdm.updateCacheMiss()
	fmt.Printf("📡 通过Provider获取数据，股票: %v\n", symbols)
	data, err := tdm.provider.FetchData(ctx, symbols)
	if err != nil {
		return nil, core.WrapError(core.ErrProviderError, "获取数据失败", err)
	}

	// 3. 获取成功后，异步更新缓存和存储
	go func() {
		// 更新顶层缓存
		if tdm.cacheEnabled {
			if err := tdm.cache.Set(ctx, cacheKey, data, tdm.config.Cache.TTL); err != nil {
				fmt.Printf("⚠️ 顶层缓存存储失败: %v\n", err)
			}
		}
		// 保存到存储层
		if err := tdm.saveToStorage(context.Background(), data); err != nil {
			fmt.Printf("⚠️ 存储数据失败: %v\n", err)
		}
	}()

	tdm.stats.mutex.Lock()
	tdm.stats.apiCalls++
	tdm.stats.mutex.Unlock()

	fmt.Printf("✅ 数据获取完成，耗时: %v\n", time.Since(startTime))
	return data, nil
}

// SetMockData 实现了 core.TestDataManager 接口的 SetMockData 方法。
func (tdm *testDataManager) SetMockData(symbols []string, data []subscriber.StockData) {
	tdm.provider.SetMockData(symbols, data)
}

// EnableCache 实现了 core.TestDataManager 接口的 EnableCache 方法。
func (tdm *testDataManager) EnableCache(enabled bool) {
	tdm.mu.Lock()
	defer tdm.mu.Unlock()

	tdm.cacheEnabled = enabled
	if !enabled {
		// 清空缓存
		tdm.cache.Clear(context.Background())
	}
}

// EnableMock 实现了 core.TestDataManager 接口的 EnableMock 方法。
func (tdm *testDataManager) EnableMock(enabled bool) {
	tdm.provider.SetMockMode(enabled)
}

// Reset 实现了 core.TestDataManager 接口的 Reset 方法。
func (tdm *testDataManager) Reset() error {
	// 清空缓存
	if err := tdm.cache.Clear(context.Background()); err != nil {
		return fmt.Errorf("清空缓存失败: %w", err)
	}

	// 重置统计信息
	tdm.stats.mutex.Lock()
	tdm.stats.cacheHits = 0
	tdm.stats.cacheMisses = 0
	tdm.stats.apiCalls = 0
	tdm.stats.storageWrites = 0
	tdm.stats.storageReads = 0
	tdm.stats.mockCalls = 0
	tdm.stats.lastActivity = time.Now()
	tdm.stats.mutex.Unlock()

	fmt.Printf("🔄 TestDataManager已重置\n")
	return nil
}

// GetStats 实现了 core.TestDataManager 接口的 GetStats 方法。
func (tdm *testDataManager) GetStats() core.Stats {
	tdm.stats.mutex.RLock()
	defer tdm.stats.mutex.RUnlock()

	// 获取缓存统计
	cacheStats := tdm.cache.Stats()

	return core.Stats{
		CacheSize:   cacheStats.Size,
		TTL:         tdm.config.Cache.TTL,
		Directory:   tdm.config.Storage.Directory,
		
		CacheHits:   tdm.stats.cacheHits + cacheStats.HitCount,
		CacheMisses: tdm.stats.cacheMisses + cacheStats.MissCount,
	}
}

// Close 实现了 core.TestDataManager 接口的 Close 方法。
func (tdm *testDataManager) Close() error {
	var errs []error

	// 关闭Provider
	if err := tdm.provider.Close(); err != nil {
		errs = append(errs, fmt.Errorf("关闭Provider失败: %w", err))
	}

	// 关闭存储
	if err := tdm.storage.Close(); err != nil {
		errs = append(errs, fmt.Errorf("关闭存储失败: %w", err))
	}

	// 关闭缓存
	if closer, ok := tdm.cache.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭缓存失败: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("关闭过程中发生错误: %v", errs)
	}

	fmt.Printf("🔒 TestDataManager已关闭\n")
	return nil
}

// --- 私有辅助方法 ---

func (tdm *testDataManager) generateCacheKey(symbols []string) string {
	// 使用简单的字符串连接作为缓存键
	return fmt.Sprintf("stocks:%v", symbols)
}

func (tdm *testDataManager) updateCacheHit() {
	tdm.stats.mutex.Lock()
	tdm.stats.cacheHits++
	tdm.stats.mutex.Unlock()
}

func (tdm *testDataManager) updateCacheMiss() {
	tdm.stats.mutex.Lock()
	tdm.stats.cacheMisses++
	tdm.stats.mutex.Unlock()
}

func (tdm *testDataManager) saveToStorage(ctx context.Context, data []subscriber.StockData) error {
	for _, stockData := range data {
		if err := tdm.storage.Save(ctx, stockData); err != nil {
			return fmt.Errorf("保存股票数据失败 %s: %w", stockData.Symbol, err)
		}
	}

	tdm.stats.mutex.Lock()
	tdm.stats.storageWrites += int64(len(data))
	tdm.stats.mutex.Unlock()

	return nil
}

// GetAdvancedStats 获取详细统计信息 (内部使用)。
func (tdm *testDataManager) GetAdvancedStats() map[string]interface{} {
	tdm.stats.mutex.RLock()
	defer tdm.stats.mutex.RUnlock()

	cacheStats := tdm.cache.Stats()

	return map[string]interface{}{
		"session_id":        tdm.sessionID,
		"cache_enabled":     tdm.cacheEnabled,
		"cache_stats":       cacheStats,
		"api_calls":         tdm.stats.apiCalls,
		"storage_writes":    tdm.stats.storageWrites,
		"storage_reads":     tdm.stats.storageReads,
		"mock_calls":        tdm.stats.mockCalls,
		"last_activity":     tdm.stats.lastActivity,
	}
}

// GetMockProvider 获取底层的Mock Provider（内部测试使用）。
func (tdm *testDataManager) GetMockProvider() interface{} {
	if cachedProvider, ok := tdm.provider.(*providers.CachedProvider); ok {
		return cachedProvider.GetMockProvider()
	}
	return nil
}

func generateSessionID() string {
	return fmt.Sprintf("enhanced_%d", time.Now().Unix())
}
