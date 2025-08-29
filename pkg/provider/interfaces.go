package provider

import (
	"context"
	"time"

	"stocksub/pkg/core"
)

// Provider 数据提供商接口
type Provider interface {
	// Name 提供商名称
	Name() string

	// FetchData 获取股票数据
	FetchData(ctx context.Context, symbols []string) ([]core.StockData, error)

	// IsSymbolSupported 检查是否支持该股票代码
	IsSymbolSupported(symbol string) bool

	// GetRateLimit 获取请求限制信息
	GetRateLimit() time.Duration

	// IsHealthy 检查提供商的健康状态
	IsHealthy() bool
}

// Configurable 可配置接口
// 支持动态配置的提供商可以实现此接口
type Configurable interface {
	// SetRateLimit 设置请求频率限制
	SetRateLimit(limit time.Duration)

	// SetTimeout 设置请求超时时间
	SetTimeout(timeout time.Duration)

	// SetMaxRetries 设置最大重试次数
	SetMaxRetries(retries int)
}

// Closable 可关闭接口
// 需要清理资源的提供商应实现此接口
type Closable interface {
	// Close 关闭提供商，清理资源
	Close() error
}

// HistoricalProvider 历史数据提供商接口
// 用于获取股票的历史K线数据
type HistoricalProvider interface {
	Provider

	// FetchHistoricalData 获取历史数据
	// symbol: 股票代码
	// start: 开始时间
	// end: 结束时间
	// period: 时间周期，如 "1d", "1h", "5m" 等
	FetchHistoricalData(ctx context.Context, symbol string, start, end time.Time, period string) ([]core.HistoricalData, error)

	// GetSupportedPeriods 获取支持的时间周期列表
	GetSupportedPeriods() []string
}

// HistoricalBatchProvider 批量历史数据提供商接口
// 支持批量获取多个股票的历史数据
type HistoricalBatchProvider interface {
	HistoricalProvider

	// FetchHistoricalDataBatch 批量获取历史数据
	// symbols: 股票代码列表
	// start: 开始时间
	// end: 结束时间
	// period: 时间周期
	FetchHistoricalDataBatch(ctx context.Context, symbols []string, start, end time.Time, period string) (map[string][]core.HistoricalData, error)
}

// RealtimeStockProvider 实时股票数据提供商接口
type RealtimeStockProvider interface {
	Provider

	// FetchStockData 获取股票实时数据
	// symbols: 股票代码列表，如 ["600000", "000001"]
	// 返回对应的股票数据列表
	FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error)

	// FetchStockDataWithRaw 获取股票数据和原始响应
	// 返回处理后的数据和原始响应字符串，用于调试和分析
	FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error)
}

// RealtimeIndexProvider 实时指数数据提供商接口
// 用于获取指数的实时行情数据
type RealtimeIndexProvider interface {
	Provider

	// Fetch core.IndexData 获取指数实时数据
	// indexSymbols: 指数代码列表，如 ["000001", "399001"]
	// 返回对应的指数数据列表
	FetchIndexData(ctx context.Context, indexSymbols []string) ([]core.IndexData, error)

	// IsIndexSupported 检查是否支持该指数代码
	IsIndexSupported(indexSymbol string) bool
}
