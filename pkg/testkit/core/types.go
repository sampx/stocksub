// Package core 定义了 testkit 框架的核心接口、数据结构和常量。
// 这些类型为所有 testkit 的子包（如 cache, storage, providers）提供了统一的抽象和交互契约。
package core

import (
	"context"
	"time"

	"stocksub/pkg/subscriber"
)

// Storage 定义了持久化存储的行为。
// 任何希望在 testkit 中作为存储后端（如CSV、数据库）的组件都必须实现此接口。
type Storage interface {
	// Save 保存一条数据记录到存储后端。
	Save(ctx context.Context, data interface{}) error
	// Load 根据查询条件从存储后端加载数据。
	Load(ctx context.Context, query Query) ([]interface{}, error)
	// Delete 根据查询条件从存储后端删除数据。
	Delete(ctx context.Context, query Query) error
	// Close 关闭存储连接并释放所有资源。
	Close() error
}

// Provider 定义了数据提供者的行为，是获取上游（真实或模拟）数据的来源。
type Provider interface {
	// FetchData 获取指定股票代码列表的股票数据。
	FetchData(ctx context.Context, symbols []string) ([]subscriber.StockData, error)
	// SetMockMode 启用或禁用数据提供者的Mock模式。
	SetMockMode(enabled bool)
	// SetMockData 为指定的股票代码设置模拟数据。
	SetMockData(symbols []string, data []subscriber.StockData)
	// Close 关闭数据提供者并释放资源。
	Close() error
}

// TestDataManager 是 testkit 框架的顶层接口，为测试用例提供了统一的交互入口。
// 它协调内部的 Provider, Cache 和 Storage，对外提供简洁的数据获取和控制方法。
type TestDataManager interface {
	// GetStockData 获取股票数据，内部会自动处理缓存、Mock和真实数据源的逻辑。
	GetStockData(ctx context.Context, symbols []string) ([]subscriber.StockData, error)
	// SetMockData 为指定的股票代码设置模拟数据，仅在Mock模式下生效。
	SetMockData(symbols []string, data []subscriber.StockData)
	// EnableCache 全局启用或禁用缓存功能。
	EnableCache(enabled bool)
	// EnableMock 全局启用或禁用Mock模式。启用后，GetStockData将从MockProvider获取数据。
	EnableMock(enabled bool)
	// Reset 清空所有缓存和内部状态，用于在测试之间隔离环境。
	Reset() error
	// GetStats 获取当前管理器的运行统计信息。
	GetStats() Stats
	// Close 关闭管理器及其所有底层组件（Provider, Cache, Storage）。
	Close() error
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
	MaxSize     int64         `json:"max_size"`      // 缓存最大容量
	HitCount    int64         `json:"hit_count"`     // 命中次数
	MissCount   int64         `json:"miss_count"`    // 未命中次数
	HitRate     float64       `json:"hit_rate"`      // 命中率
	TTL         time.Duration `json:"ttl"`          // 默认的生存时间
	LastCleanup time.Time     `json:"last_cleanup"`  // 最后一次清理过期条目的时间
}

// Stats 包含了 TestDataManager 的高级统计信息。
type Stats struct {
	CacheSize   int64         `json:"cache_size"`   // 缓存中的条目总数
	TTL         time.Duration `json:"ttl"`         // 缓存的默认生存时间
	Directory   string        `json:"directory"`   // 持久化存储的目录
	MockMode    bool          `json:"mock_mode"`    // Mock模式是否启用
	CacheHits   int64         `json:"cache_hits"`   // 缓存总命中数
	CacheMisses int64         `json:"cache_misses"` // 缓存总未命中数
}

// Query 定义了在存储层进行数据查询的条件。
type Query struct {
	Symbols   []string  `json:"symbols"`    // 目标股票代码
	StartTime time.Time `json:"start_time"`  // 查询的开始时间
	EndTime   time.Time `json:"end_time"`    // 查询的结束时间
	Fields    []string  `json:"fields"`     // 需要返回的字段
	Limit     int       `json:"limit"`      // 返回记录的最大数量
	Offset    int       `json:"offset"`     // 返回记录的偏移量
}

// Record 代表一条通用的、可被存储的数据记录。
type Record struct {
	Type      string      `json:"type"`      // 数据类型 (e.g., "stock_data", "performance_metric")
	Symbol    string      `json:"symbol"`    // 关联的股票代码
	Timestamp time.Time   `json:"timestamp"`  // 记录生成的时间戳
	Date      string      `json:"date"`      // 记录生成的日期 (YYYY-MM-DD)
	Fields    []string    `json:"fields"`    // 用于CSV存储的字段数组
	Data      interface{} `json:"data"`      // 原始数据对象
}

// MockScenario 定义了一个完整的模拟场景，用于高级Mock测试。
type MockScenario struct {
	Name        string                  `yaml:"name"`        // 场景名称，唯一标识
	Description string                  `yaml:"description"` // 场景描述
	Responses   map[string]MockResponse `yaml:"responses"`   // 针对不同symbol的模拟响应
	Delays      map[string]time.Duration `yaml:"delays"`      // 针对不同symbol的模拟延迟
	Errors      map[string]error        `yaml:"errors"`      // 针对不同symbol的模拟错误
	CallCounts  map[string]int          `yaml:"call_counts"` // 调用次数统计
}

// MockResponse 定义了一个具体的模拟响应。
type MockResponse struct {
	Data      []subscriber.StockData `yaml:"data"`      // 模拟的股票数据
	Error     string                 `yaml:"error"`     // 模拟的错误信息
	Delay     time.Duration          `yaml:"delay"`     // 模拟的延迟
	CallCount int                    `yaml:"call_count"` // 预期的调用次数
}

// ResourceManager 定义了对可复用资源（如缓冲区、CSV写入器）的管理接口。
type ResourceManager interface {
	AcquireCSVWriter() interface{}
	ReleaseCSVWriter(writer interface{})
	AcquireBuffer() interface{}
	ReleaseBuffer(buffer interface{})
	RegisterCleanup(fn func())
	Cleanup()
}

// Serializer 定义了对象和字节流之间相互转换的接口。
type Serializer interface {
	// Serialize 将任意对象序列化为字节数组。
	Serialize(data interface{}) ([]byte, error)
	// Deserialize 将字节数组反序列化为目标对象。
	Deserialize(data []byte, target interface{}) error
	// MimeType 返回此序列化器对应的MIME类型。
	MimeType() string
}

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
