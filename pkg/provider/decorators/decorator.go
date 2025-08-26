package decorators

import (
	"context"
	"stocksub/pkg/provider/core"
	"stocksub/pkg/subscriber"
	"time"
)

// Decorator 装饰器基础接口
// 所有装饰器都应该实现此接口
type Decorator interface {
	core.Provider
	
	// GetBaseProvider 获取被装饰的基础 Provider
	GetBaseProvider() core.Provider
}

// RealtimeStockDecorator 实时股票装饰器接口
// 装饰 RealtimeStockProvider
type RealtimeStockDecorator interface {
	core.RealtimeStockProvider
	Decorator
}

// RealtimeIndexDecorator 实时指数装饰器接口  
// 装饰 RealtimeIndexProvider
type RealtimeIndexDecorator interface {
	core.RealtimeIndexProvider
	Decorator
}

// BaseDecorator 装饰器基础实现
// 提供通用的装饰器功能
type BaseDecorator struct {
	base core.Provider
}

// NewBaseDecorator 创建基础装饰器
func NewBaseDecorator(base core.Provider) *BaseDecorator {
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
func (d *BaseDecorator) GetBaseProvider() core.Provider {
	return d.base
}

// RealtimeStockBaseDecorator 实时股票装饰器基础实现
type RealtimeStockBaseDecorator struct {
	*BaseDecorator
	stockProvider core.RealtimeStockProvider
}

// NewRealtimeStockBaseDecorator 创建实时股票基础装饰器
func NewRealtimeStockBaseDecorator(stockProvider core.RealtimeStockProvider) *RealtimeStockBaseDecorator {
	return &RealtimeStockBaseDecorator{
		BaseDecorator: NewBaseDecorator(stockProvider),
		stockProvider: stockProvider,
	}
}

// FetchStockData 实现 RealtimeStockProvider 接口
func (d *RealtimeStockBaseDecorator) FetchStockData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	return d.stockProvider.FetchStockData(ctx, symbols)
}

// FetchStockDataWithRaw 实现 RealtimeStockProvider 接口  
func (d *RealtimeStockBaseDecorator) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]subscriber.StockData, string, error) {
	return d.stockProvider.FetchStockDataWithRaw(ctx, symbols)
}

// IsSymbolSupported 实现 RealtimeStockProvider 接口
func (d *RealtimeStockBaseDecorator) IsSymbolSupported(symbol string) bool {
	return d.stockProvider.IsSymbolSupported(symbol)
}

// DecoratorChain 装饰器链
// 用于组合多个装饰器
type DecoratorChain struct {
	decorators []func(core.Provider) core.Provider
}

// NewDecoratorChain 创建装饰器链
func NewDecoratorChain() *DecoratorChain {
	return &DecoratorChain{
		decorators: make([]func(core.Provider) core.Provider, 0),
	}
}

// AddDecorator 添加装饰器到链中
func (dc *DecoratorChain) AddDecorator(decorator func(core.Provider) core.Provider) *DecoratorChain {
	dc.decorators = append(dc.decorators, decorator)
	return dc
}

// Apply 应用装饰器链到指定的 Provider
func (dc *DecoratorChain) Apply(base core.Provider) core.Provider {
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
