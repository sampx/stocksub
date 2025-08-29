package provider

import (
	"context"
	"time"

	"stocksub/pkg/core"
)

// Provider 是所有数据提供商（实时、历史、指数等）的基础接口。
// 它定义了所有提供商都必须具备的通用功能，如名称、健康状态和速率限制。
type Provider interface {
	// Name 返回提供商的名称，例如 "tencent" 或 "sina"。
	Name() string

	// IsHealthy 检查提供商的健康状态。
	// 如果提供商能够正常服务，则返回 true。
	IsHealthy() bool

	// GetRateLimit 返回两个连续请求之间的最小允许间隔。
	GetRateLimit() time.Duration
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

// RealtimeStockProvider 定义了获取实时股票数据的提供商接口。
// 它专注于提供股票的当前市场数据。
type RealtimeStockProvider interface {
	Provider

	// FetchStockData 获取指定股票代码列表的实时数据。
	// symbols: 股票代码列表，例如 ["600000", "000001"]。
	FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error)

	// FetchStockDataWithRaw 获取实时股票数据及其原始响应。
	// 这对于调试或需要访问原始API输出的场景很有用。
	FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error)

	// IsSymbolSupported 检查提供商是否支持给定的股票代码。
	IsSymbolSupported(symbol string) bool
}

// RealtimeIndexProvider 定义了获取实时指数数据的提供商接口。
type RealtimeIndexProvider interface {
	Provider

	// FetchIndexData 获取指定指数代码列表的实时数据。
	// indexSymbols: 指数代码列表，例如 ["sh000001", "sz399001"]。
	FetchIndexData(ctx context.Context, indexSymbols []string) ([]core.IndexData, error)

	// IsIndexSupported 检查提供商是否支持给定的指数代码。
	IsIndexSupported(indexSymbol string) bool
}

// --- Decorator Interfaces and Structs ---

// Decorator 装饰器基础接口
type Decorator interface {
	Provider
	GetBaseProvider() Provider
}

// BaseDecorator 装饰器基础实现
type BaseDecorator struct {
	base Provider
}

// NewBaseDecorator 创建基础装饰器
func NewBaseDecorator(base Provider) *BaseDecorator {
	return &BaseDecorator{base: base}
}

// Name 实现 Provider 接口
func (d *BaseDecorator) Name() string {
	return d.base.Name()
}

// GetRateLimit 实现 Provider 接口
func (d *BaseDecorator) GetRateLimit() time.Duration {
	return d.base.GetRateLimit()
}

// IsHealthy 实现 Provider 接口
func (d *BaseDecorator) IsHealthy() bool {
	return d.base.IsHealthy()
}

// GetBaseProvider 实现 Decorator 接口
func (d *BaseDecorator) GetBaseProvider() Provider {
	return d.base
}

// DecoratorType 装饰器类型枚举
type DecoratorType string

const (
	FrequencyControlType DecoratorType = "frequency_control"
	CircuitBreakerType   DecoratorType = "circuit_breaker"
)

// DecoratorConfig 装饰器配置
type DecoratorConfig struct {
	Type         DecoratorType          `yaml:"type" mapstructure:"type"`
	Enabled      bool                   `yaml:"enabled" mapstructure:"enabled"`
	Priority     int                    `yaml:"priority" mapstructure:"priority"`
	ProviderType string                 `yaml:"provider_type" mapstructure:"provider_type"`
	Config       map[string]interface{} `yaml:"config" mapstructure:"config"`
}

// ProviderDecoratorConfig 提供商装饰器完整配置
type ProviderDecoratorConfig struct {
	Realtime   []DecoratorConfig `yaml:"realtime" mapstructure:"realtime"`
	Historical []DecoratorConfig `yaml:"historical" mapstructure:"historical"`
	Index      []DecoratorConfig `yaml:"index" mapstructure:"index"`
	All        []DecoratorConfig `yaml:"all" mapstructure:"all"`
}

// DecoratorFactory 装饰器工厂
type DecoratorFactory struct{}

// NewDecoratorFactory 创建装饰器工厂
func NewDecoratorFactory() *DecoratorFactory {
	return &DecoratorFactory{}
}
