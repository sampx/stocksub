package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stocksub/pkg/testkit/core"
)

// RemoteCacheConfig 远程缓存配置
type RemoteCacheConfig struct {
	Address        string        `yaml:"address"`         // 远程服务器地址
	MaxSize        int64         `yaml:"max_size"`        // 最大缓存条目数（客户端限制）
	DefaultTTL     time.Duration `yaml:"default_ttl"`     // 默认生存时间
	ConnectTimeout time.Duration `yaml:"connect_timeout"` // 连接超时
	RequestTimeout time.Duration `yaml:"request_timeout"` // 请求超时
	MaxConnections int           `yaml:"max_connections"` // 最大连接数
	PoolSize       int           `yaml:"pool_size"`       // 连接池大小
}

// RemoteCache 远程缓存接口
// 为不同的远程缓存实现（Redis、Memcached等）提供统一接口
type RemoteCache interface {
	core.Cache

	// Connect 连接到远程缓存服务器
	Connect(ctx context.Context) error

	// IsConnected 检查是否已连接
	IsConnected() bool

	// Ping 检查连接状态
	Ping(ctx context.Context) error

	// GetStats 获取远程缓存统计信息
	GetStats(ctx context.Context) (map[string]interface{}, error)
}

// remoteCacheBase 远程缓存基础实现
type remoteCacheBase struct {
	mu          sync.RWMutex
	config      RemoteCacheConfig
	stats       core.CacheStats
	isConnected bool
	client      interface{} // 具体的客户端实现
}

// newRemoteCacheBase 创建远程缓存基础实例
func newRemoteCacheBase(config RemoteCacheConfig) *remoteCacheBase {
	return &remoteCacheBase{
		config: config,
		stats: core.CacheStats{
			MaxSize: config.MaxSize,
			TTL:     config.DefaultTTL,
		},
	}
}

// Get 从远程缓存获取数据（基础实现，需要具体实现重写）
func (rc *remoteCacheBase) Get(ctx context.Context, key string) (interface{}, error) {
	return nil, fmt.Errorf("Get method not implemented")
}

// Set 向远程缓存设置数据（基础实现，需要具体实现重写）
func (rc *remoteCacheBase) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return fmt.Errorf("Set method not implemented")
}

// Delete 从远程缓存删除数据（基础实现，需要具体实现重写）
func (rc *remoteCacheBase) Delete(ctx context.Context, key string) error {
	return fmt.Errorf("Delete method not implemented")
}

// Clear 清空远程缓存（基础实现，需要具体实现重写）
func (rc *remoteCacheBase) Clear(ctx context.Context) error {
	return fmt.Errorf("Clear method not implemented")
}

// Stats 获取缓存统计信息
func (rc *remoteCacheBase) Stats() core.CacheStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	stats := rc.stats
	stats.LastCleanup = time.Now()

	// 计算命中率
	total := stats.HitCount + stats.MissCount
	if total > 0 {
		stats.HitRate = float64(stats.HitCount) / float64(total)
	}

	return stats
}

// IsConnected 检查是否已连接
func (rc *remoteCacheBase) IsConnected() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.isConnected
}

// setConnected 设置连接状态
func (rc *remoteCacheBase) setConnected(connected bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.isConnected = connected
}

// updateStats 更新统计信息
func (rc *remoteCacheBase) updateStats(hit bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if hit {
		rc.stats.HitCount++
	} else {
		rc.stats.MissCount++
	}
}

// MockRemoteCache 模拟远程缓存实现（用于测试和开发）
type MockRemoteCache struct {
	*remoteCacheBase
	data map[string]mockRemoteEntry
}

type mockRemoteEntry struct {
	value      interface{}
	expireTime time.Time
}

// NewMockRemoteCache 创建模拟远程缓存
func NewMockRemoteCache(config RemoteCacheConfig) *MockRemoteCache {
	return &MockRemoteCache{
		remoteCacheBase: newRemoteCacheBase(config),
		data:            make(map[string]mockRemoteEntry),
	}
}

// Connect 模拟连接
func (m *MockRemoteCache) Connect(ctx context.Context) error {
	m.setConnected(true)
	return nil
}

// Ping 模拟Ping操作
func (m *MockRemoteCache) Ping(ctx context.Context) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected")
	}
	return nil
}

// Get 从模拟缓存获取数据
func (m *MockRemoteCache) Get(ctx context.Context, key string) (interface{}, error) {
	m.mu.RLock()
	entry, exists := m.data[key]
	m.mu.RUnlock()

	if !exists {
		m.updateStats(false)
		return nil, core.NewTestKitError(core.ErrCacheMiss, "cache miss")
	}

	// 检查是否过期
	if time.Now().After(entry.expireTime) {
		m.mu.Lock()
		delete(m.data, key)
		m.stats.Size--
		m.mu.Unlock()
		m.updateStats(false)
		return nil, core.NewTestKitError(core.ErrCacheMiss, "cache expired")
	}

	m.updateStats(true)
	return entry.value, nil
}

// Set 向模拟缓存设置数据
func (m *MockRemoteCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = m.config.DefaultTTL
	}

	expireTime := time.Now().Add(ttl)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否超过最大大小
	if int64(len(m.data)) >= m.config.MaxSize && m.config.MaxSize > 0 {
		// 简单策略：删除最旧的条目
		var oldestKey string
		var oldestTime time.Time
		for k, e := range m.data {
			if oldestTime.IsZero() || e.expireTime.Before(oldestTime) {
				oldestKey = k
				oldestTime = e.expireTime
			}
		}
		if oldestKey != "" {
			delete(m.data, oldestKey)
			m.stats.Size--
		}
	}

	// 添加新条目
	if _, exists := m.data[key]; !exists {
		m.stats.Size++
	}

	m.data[key] = mockRemoteEntry{
		value:      value,
		expireTime: expireTime,
	}

	return nil
}

// Delete 从模拟缓存删除数据
func (m *MockRemoteCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.data[key]; exists {
		delete(m.data, key)
		m.stats.Size--
	}

	return nil
}

// Clear 清空模拟缓存
func (m *MockRemoteCache) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]mockRemoteEntry)
	m.stats.Size = 0
	m.stats.HitCount = 0
	m.stats.MissCount = 0

	return nil
}

// GetStats 获取模拟缓存统计信息
func (m *MockRemoteCache) GetStats(ctx context.Context) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.Stats()
	return map[string]interface{}{
		"connected":    m.isConnected,
		"items_count":  len(m.data),
		"hit_count":    stats.HitCount,
		"miss_count":   stats.MissCount,
		"hit_rate":     stats.HitRate,
		"max_size":     stats.MaxSize,
		"current_size": stats.Size,
	}, nil
}

// Close 关闭模拟缓存
func (m *MockRemoteCache) Close() error {
	m.setConnected(false)
	return nil
}

var _ RemoteCache = (*MockRemoteCache)(nil)
