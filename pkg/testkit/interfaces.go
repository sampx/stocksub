// 定义了 testkit 框架的核心接口、数据结构和常量。
// 这些类型为所有 testkit 的子包（如 cache, storage, providers）提供了统一的抽象和交互契约。
package testkit

import (
	"context"
	"time"

	"stocksub/pkg/core"
)

// MockProvider 定义了数据提供者的行为，是获取上游（真实或模拟）数据的来源。
type MockProvider interface {
	// FetchData 获取指定股票代码列表的股票数据。
	FetchData(ctx context.Context, symbols []string) ([]core.StockData, error)
	// SetMockMode 启用或禁用数据提供者的Mock模式。
	SetMockMode(enabled bool)
	// SetMockData 为指定的股票代码设置模拟数据。
	SetMockData(symbols []string, data []core.StockData)
	// Close 关闭数据提供者并释放资源。
	Close() error
}

// TestDataManager 是 testkit 框架的顶层接口，为测试用例提供了统一的交互入口。
// 它协调内部的 Provider, Cache 和 Storage，对外提供简洁的数据获取和控制方法。
type TestDataManager interface {
	// GetStockData 获取股票数据，内部会自动处理缓存、Mock和真实数据源的逻辑。
	GetStockData(ctx context.Context, symbols []string) ([]core.StockData, error)
	// SetMockData 为指定的股票代码设置模拟数据，仅在Mock模式下生效。
	SetMockData(symbols []string, data []core.StockData)
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

// Stats 包含了 TestDataManager 的高级统计信息。
type Stats struct {
	CacheSize   int64         `json:"cache_size"`   // 缓存中的条目总数
	TTL         time.Duration `json:"ttl"`          // 缓存的默认生存时间
	Directory   string        `json:"directory"`    // 持久化存储的目录
	MockMode    bool          `json:"mock_mode"`    // Mock模式是否启用
	CacheHits   int64         `json:"cache_hits"`   // 缓存总命中数
	CacheMisses int64         `json:"cache_misses"` // 缓存总未命中数
}

// MockScenario 定义了一个完整的模拟场景，用于高级Mock测试。
type MockScenario struct {
	Name        string                   `yaml:"name"`        // 场景名称，唯一标识
	Description string                   `yaml:"description"` // 场景描述
	Responses   map[string]MockResponse  `yaml:"responses"`   // 针对不同symbol的模拟响应
	Delays      map[string]time.Duration `yaml:"delays"`      // 针对不同symbol的模拟延迟
	Errors      map[string]error         `yaml:"errors"`      // 针对不同symbol的模拟错误
	CallCounts  map[string]int           `yaml:"call_counts"` // 调用次数统计
}

// MockResponse 定义了一个具体的模拟响应。
type MockResponse struct {
	Data      []core.StockData `yaml:"data"`       // 模拟的股票数据
	Error     string           `yaml:"error"`      // 模拟的错误信息
	Delay     time.Duration    `yaml:"delay"`      // 模拟的延迟
	CallCount int              `yaml:"call_count"` // 预期的调用次数
}
