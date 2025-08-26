package core

import (
	"context"
	"stocksub/pkg/subscriber"
	"time"
)

// LegacyProviderAdapter 旧版 Provider 适配器
// 将旧版的 subscriber.Provider 适配为新的 core.RealtimeStockProvider 接口
type LegacyProviderAdapter struct {
	legacyProvider subscriber.Provider
}

// NewLegacyProviderAdapter 创建新的旧版Provider适配器
func NewLegacyProviderAdapter(provider subscriber.Provider) *LegacyProviderAdapter {
	return &LegacyProviderAdapter{
		legacyProvider: provider,
	}
}

// Name 返回提供商名称
func (a *LegacyProviderAdapter) Name() string {
	return a.legacyProvider.Name()
}

// GetRateLimit 获取请求频率限制
func (a *LegacyProviderAdapter) GetRateLimit() time.Duration {
	return a.legacyProvider.GetRateLimit()
}

// IsHealthy 检查提供商健康状态
// 旧版 Provider 没有健康检查，默认返回 true
func (a *LegacyProviderAdapter) IsHealthy() bool {
	// 简单的健康检查：检查提供商是否为 nil
	return a.legacyProvider != nil
}

// FetchStockData 获取股票实时数据
func (a *LegacyProviderAdapter) FetchStockData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	return a.legacyProvider.FetchData(ctx, symbols)
}

// FetchStockDataWithRaw 获取股票数据和原始响应
// 如果旧版 Provider 支持原始数据，则调用对应方法，否则只返回处理后的数据
func (a *LegacyProviderAdapter) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]subscriber.StockData, string, error) {
	// 检查是否支持原始数据接口
	if rawProvider, ok := a.legacyProvider.(interface {
		FetchDataWithRaw(ctx context.Context, symbols []string) ([]subscriber.StockData, string, error)
	}); ok {
		return rawProvider.FetchDataWithRaw(ctx, symbols)
	}

	// 不支持原始数据，只返回处理后的数据
	data, err := a.legacyProvider.FetchData(ctx, symbols)
	return data, "", err
}

// IsSymbolSupported 检查是否支持该股票代码
func (a *LegacyProviderAdapter) IsSymbolSupported(symbol string) bool {
	return a.legacyProvider.IsSymbolSupported(symbol)
}

// GetLegacyProvider 获取原始的旧版 Provider
// 用于需要直接访问旧版 Provider 的场景
func (a *LegacyProviderAdapter) GetLegacyProvider() subscriber.Provider {
	return a.legacyProvider
}

// NewProviderAdapter 智能适配器创建函数
// 根据输入的 Provider 类型，自动选择合适的适配器
func NewProviderAdapter(provider interface{}) RealtimeStockProvider {
	// 如果已经是新接口，直接返回
	if newProvider, ok := provider.(RealtimeStockProvider); ok {
		return newProvider
	}

	// 如果是旧版接口，使用适配器
	if legacyProvider, ok := provider.(subscriber.Provider); ok {
		return NewLegacyProviderAdapter(legacyProvider)
	}

	// 不支持的类型，返回 nil
	return nil
}
