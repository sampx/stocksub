package decorators

import (
	"context"
	"stocksub/pkg/core"
	"stocksub/pkg/provider"
	"time"
)

// Decorator 装饰器基础接口
// 所有装饰器都应该实现此接口
type Decorator interface {
	provider.Provider

	// GetBaseProvider 获取被装饰的基础 Provider
	GetBaseProvider() provider.Provider
}

// RealtimeStockDecorator 实时股票装饰器接口
// 装饰 RealtimeStockProvider
type RealtimeStockDecorator interface {
	provider.RealtimeStockProvider
	Decorator
}

// RealtimeIndexDecorator 实时指数装饰器接口
// 装饰 RealtimeIndexProvider
type RealtimeIndexDecorator interface {
	provider.RealtimeIndexProvider
	Decorator
}

// BaseDecorator 装饰器基础实现
// 提供通用的装饰器功能
type BaseDecorator struct {
	base provider.Provider
}

// NewBaseDecorator 创建基础装饰器
func NewBaseDecorator(base provider.Provider) *BaseDecorator {
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
func (d *BaseDecorator) GetBaseProvider() provider.Provider {
	return d.base
}

// RealtimeStockBaseDecorator 实时股票装饰器基础实现
type RealtimeStockBaseDecorator struct {
	*BaseDecorator
	stockProvider provider.RealtimeStockProvider
}

// NewRealtimeStockBaseDecorator 创建实时股票基础装饰器
func NewRealtimeStockBaseDecorator(stockProvider provider.RealtimeStockProvider) *RealtimeStockBaseDecorator {
	return &RealtimeStockBaseDecorator{
		BaseDecorator: NewBaseDecorator(stockProvider),
		stockProvider: stockProvider,
	}
}

// FetchStockData 实现 RealtimeStockProvider 接口
func (d *RealtimeStockBaseDecorator) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	return d.stockProvider.FetchStockData(ctx, symbols)
}

// FetchData 实现 Provider 接口
func (d *RealtimeStockBaseDecorator) FetchData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	return d.stockProvider.FetchStockData(ctx, symbols)
}

// FetchStockDataWithRaw 实现 RealtimeStockProvider 接口
func (d *RealtimeStockBaseDecorator) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	return d.stockProvider.FetchStockDataWithRaw(ctx, symbols)
}

// IsSymbolSupported 实现 RealtimeStockProvider 接口
func (d *RealtimeStockBaseDecorator) IsSymbolSupported(symbol string) bool {
	return d.stockProvider.IsSymbolSupported(symbol)
}

// DecoratorChain 装饰器链
// 用于组合多个装饰器
type DecoratorChain struct {
	decorators []func(provider.Provider) provider.Provider
}

// NewDecoratorChain 创建装饰器链
func NewDecoratorChain() *DecoratorChain {
	return &DecoratorChain{
		decorators: make([]func(provider.Provider) provider.Provider, 0),
	}
}

// AddDecorator 添加装饰器到链中
func (dc *DecoratorChain) AddDecorator(decorator func(provider.Provider) provider.Provider) *DecoratorChain {
	dc.decorators = append(dc.decorators, decorator)
	return dc
}

// Apply 应用装饰器链到指定的 Provider
func (dc *DecoratorChain) Apply(base provider.Provider) provider.Provider {
	provider := base
	for _, decorator := range dc.decorators {
		provider = decorator(provider)
	}
	return provider
}

// DecoratorFactory 装饰器工厂
// 用于创建各种类型的装饰器
type DecoratorFactory struct{}

// NewDecoratorFactory 创建装饰器工厂
func NewDecoratorFactory() *DecoratorFactory {
	return &DecoratorFactory{}
}

// ApplyDefaultDecorators 应用默认装饰器配置
func ApplyDefaultDecorators(provider provider.RealtimeStockProvider) (provider.RealtimeStockProvider, error) {
	// 应用频率控制装饰器
	frequencyProvider := NewFrequencyControlProvider(provider, &FrequencyControlConfig{
		MinInterval: 200 * time.Millisecond,
		MaxRetries:  3,
		Enabled:     true,
	})

	// 应用熔断器装饰器
	circuitProvider := NewCircuitBreakerProvider(frequencyProvider, &CircuitBreakerConfig{
		Name:        "default-circuit-breaker",
		MaxRequests: 3,
		Interval:    30 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: 5,
		Enabled:     true,
	})

	return circuitProvider, nil
}
