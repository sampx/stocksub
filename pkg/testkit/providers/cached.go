package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/core"
)

// CachedProvider 缓存包装的Provider
type CachedProvider struct {
	realProvider  core.Provider
	mockProvider  *MockProvider
	cache         core.Cache
	mu            sync.RWMutex
	config        CachedProviderConfig
	stats         CachedProviderStats
	mockMode      bool
}

// CachedProviderConfig 缓存Provider配置
type CachedProviderConfig struct {
	CacheTTL         time.Duration `yaml:"cache_ttl"`         // 缓存TTL
	EnableMockFallback bool        `yaml:"enable_mock_fallback"` // 是否启用Mock回退
	MaxRetries       int           `yaml:"max_retries"`       // 最大重试次数
	RetryDelay       time.Duration `yaml:"retry_delay"`       // 重试延迟
	TimeoutDuration  time.Duration `yaml:"timeout_duration"`  // 请求超时
}

// CachedProviderStats 缓存Provider统计
type CachedProviderStats struct {
	TotalRequests    int64         `json:"total_requests"`
	CacheHits        int64         `json:"cache_hits"`
	CacheMisses      int64         `json:"cache_misses"`
	RealProviderCalls int64        `json:"real_provider_calls"`
	MockProviderCalls int64        `json:"mock_provider_calls"`
	FailedRequests   int64         `json:"failed_requests"`
	AverageLatency   time.Duration `json:"average_latency"`
	LastRequest      time.Time     `json:"last_request"`
}

// NewCachedProvider 创建缓存Provider
func NewCachedProvider(realProvider core.Provider, cache core.Cache, config CachedProviderConfig) *CachedProvider {
	// 创建Mock Provider
	mockConfig := DefaultMockProviderConfig()
	mockProvider := NewMockProvider(mockConfig)
	
	return &CachedProvider{
		realProvider: realProvider,
		mockProvider: mockProvider,
		cache:        cache,
		config:       config,
		stats:        CachedProviderStats{},
		mockMode:     false,
	}
}

// FetchData 获取股票数据
func (cp *CachedProvider) FetchData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	startTime := time.Now()
	cp.stats.TotalRequests++
	cp.stats.LastRequest = startTime
	
	// 如果是Mock模式，直接使用Mock Provider
	if cp.mockMode {
		cp.stats.MockProviderCalls++
		return cp.mockProvider.FetchData(ctx, symbols)
	}
	
	// 尝试从缓存获取
	cacheKey := cp.generateCacheKey(symbols)
	if cached, err := cp.cache.Get(ctx, cacheKey); err == nil {
		cp.stats.CacheHits++
		cp.updateLatency(time.Since(startTime))
		return cached.([]subscriber.StockData), nil
	}
	
	cp.stats.CacheMisses++
	
	// 从真实Provider获取数据
	data, err := cp.fetchFromRealProvider(ctx, symbols)
	if err != nil {
		cp.stats.FailedRequests++
		
		// 如果启用了Mock回退，使用Mock数据
		if cp.config.EnableMockFallback {
			cp.stats.MockProviderCalls++
			mockData, mockErr := cp.mockProvider.FetchData(ctx, symbols)
			if mockErr == nil {
				cp.updateLatency(time.Since(startTime))
				return mockData, nil
			}
		}
		
		return nil, fmt.Errorf("获取数据失败: %w", err)
	}
	
	// 存入缓存
	if err := cp.cache.Set(ctx, cacheKey, data, cp.config.CacheTTL); err != nil {
		// 缓存失败不影响数据返回，只记录错误
		fmt.Printf("缓存写入失败: %v\n", err)
	}
	
	cp.stats.RealProviderCalls++
	cp.updateLatency(time.Since(startTime))
	
	return data, nil
}

// SetMockMode 设置Mock模式
func (cp *CachedProvider) SetMockMode(enabled bool) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	cp.mockMode = enabled
	if cp.mockProvider != nil {
		cp.mockProvider.SetMockMode(enabled)
	}
}

// SetMockData 设置Mock数据
func (cp *CachedProvider) SetMockData(symbols []string, data []subscriber.StockData) {
	if cp.mockProvider != nil {
		cp.mockProvider.SetMockData(symbols, data)
	}
}

// Close 关闭Provider
func (cp *CachedProvider) Close() error {
	var errs []error
	
	if cp.realProvider != nil {
		if err := cp.realProvider.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭真实Provider失败: %w", err))
		}
	}
	
	if cp.mockProvider != nil {
		if err := cp.mockProvider.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭Mock Provider失败: %w", err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("关闭Provider时发生错误: %v", errs)
	}
	
	return nil
}

// GetStats 获取统计信息
func (cp *CachedProvider) GetStats() CachedProviderStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	
	return cp.stats
}

// GetMockProvider 获取Mock Provider
func (cp *CachedProvider) GetMockProvider() *MockProvider {
	return cp.mockProvider
}

// fetchFromRealProvider 从真实Provider获取数据（带重试）
func (cp *CachedProvider) fetchFromRealProvider(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	var lastErr error
	
	for attempt := 0; attempt <= cp.config.MaxRetries; attempt++ {
		// 创建超时上下文
		timeoutCtx, cancel := context.WithTimeout(ctx, cp.config.TimeoutDuration)
		
		data, err := cp.realProvider.FetchData(timeoutCtx, symbols)
		cancel()
		
		if err == nil {
			return data, nil
		}
		
		lastErr = err
		
		// 如果不是最后一次尝试，等待后重试
		if attempt < cp.config.MaxRetries {
			time.Sleep(cp.config.RetryDelay)
		}
	}
	
	return nil, fmt.Errorf("重试 %d 次后仍失败: %w", cp.config.MaxRetries, lastErr)
}

// generateCacheKey 生成缓存键
func (cp *CachedProvider) generateCacheKey(symbols []string) string {
	// 简单的键生成策略
	key := "stock_data:"
	for i, symbol := range symbols {
		if i > 0 {
			key += ","
		}
		key += symbol
	}
	return key
}

// updateLatency 更新平均延迟
func (cp *CachedProvider) updateLatency(duration time.Duration) {
	if cp.stats.AverageLatency == 0 {
		cp.stats.AverageLatency = duration
	} else {
		// 简单的移动平均
		cp.stats.AverageLatency = (cp.stats.AverageLatency + duration) / 2
	}
}

// TencentProviderWrapper 腾讯Provider包装器
type TencentProviderWrapper struct {
	client *tencent.Provider
	mu     sync.Mutex
}

// FetchData 获取股票数据
func (tpw *TencentProviderWrapper) FetchData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	tpw.mu.Lock()
	defer tpw.mu.Unlock()
	
	// 如果客户端未初始化，进行初始化
	if tpw.client == nil {
		tpw.client = tencent.NewProvider()
	}
	
	// 调用腾讯API
	return tpw.client.FetchData(ctx, symbols)
}

// SetMockMode 设置Mock模式（腾讯Provider不支持）
func (tpw *TencentProviderWrapper) SetMockMode(enabled bool) {
	// 腾讯Provider不支持Mock模式
}

// SetMockData 设置Mock数据（腾讯Provider不支持）
func (tpw *TencentProviderWrapper) SetMockData(symbols []string, data []subscriber.StockData) {
	// 腾讯Provider不支持Mock数据设置
}

// Close 关闭Provider
func (tpw *TencentProviderWrapper) Close() error {
	// 腾讯Provider没有需要关闭的资源
	return nil
}

// ProviderFactory Provider工厂
type ProviderFactory struct {
	cache core.Cache
}

// NewProviderFactory 创建Provider工厂
func NewProviderFactory(cache core.Cache) *ProviderFactory {
	return &ProviderFactory{
		cache: cache,
	}
}

// CreateMockProvider 创建Mock Provider
func (pf *ProviderFactory) CreateMockProvider(config MockProviderConfig) *MockProvider {
	return NewMockProvider(config)
}

// CreateCachedProvider 创建缓存Provider
func (pf *ProviderFactory) CreateCachedProvider(config CachedProviderConfig) *CachedProvider {
	return NewCachedProvider(pf.CreateRealProvider(), pf.cache, config)
}

// CreateRealProvider 创建真实Provider
func (pf *ProviderFactory) CreateRealProvider() core.Provider {
	return &TencentProviderWrapper{}
}

// ProviderManager Provider管理器
type ProviderManager struct {
	providers map[string]core.Provider
	mu        sync.RWMutex
	factory   *ProviderFactory
}

// NewProviderManager 创建Provider管理器
func NewProviderManager(cache core.Cache) *ProviderManager {
	return &ProviderManager{
		providers: make(map[string]core.Provider),
		factory:   NewProviderFactory(cache),
	}
}

// RegisterProvider 注册Provider
func (pm *ProviderManager) RegisterProvider(name string, provider core.Provider) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.providers[name] = provider
}

// GetProvider 获取Provider
func (pm *ProviderManager) GetProvider(name string) (core.Provider, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	provider, exists := pm.providers[name]
	return provider, exists
}

// RemoveProvider 移除Provider
func (pm *ProviderManager) RemoveProvider(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if provider, exists := pm.providers[name]; exists {
		provider.Close()
		delete(pm.providers, name)
	}
}

// CloseAll 关闭所有Provider
func (pm *ProviderManager) CloseAll() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	var errs []error
	for name, provider := range pm.providers {
		if err := provider.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭Provider %s 失败: %w", name, err))
		}
	}
	
	pm.providers = make(map[string]core.Provider)
	
	if len(errs) > 0 {
		return fmt.Errorf("关闭Provider时发生错误: %v", errs)
	}
	
	return nil
}

// DefaultCachedProviderConfig 默认缓存Provider配置
func DefaultCachedProviderConfig() CachedProviderConfig {
	return CachedProviderConfig{
		CacheTTL:           5 * time.Minute,
		EnableMockFallback: true,
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		TimeoutDuration:    10 * time.Second,
	}
}
