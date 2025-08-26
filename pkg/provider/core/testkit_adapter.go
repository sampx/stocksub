package core

import (
	"context"
	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/core"
	"time"
)

// TestKitProviderAdapter testkit Provider 适配器
// 将 testkit/core.Provider 适配为新的 core.RealtimeStockProvider 接口
type TestKitProviderAdapter struct {
	testkitProvider core.Provider
}

// NewTestKitProviderAdapter 创建 testkit Provider 适配器
func NewTestKitProviderAdapter(provider core.Provider) *TestKitProviderAdapter {
	return &TestKitProviderAdapter{
		testkitProvider: provider,
	}
}

// Name 返回提供商名称
func (a *TestKitProviderAdapter) Name() string {
	// testkit Provider 没有 Name 方法，返回固定名称
	return "testkit_provider"
}

// GetRateLimit 获取请求频率限制
func (a *TestKitProviderAdapter) GetRateLimit() time.Duration {
	// testkit Provider 没有频率限制概念，返回默认值
	return 200 * time.Millisecond
}

// IsHealthy 检查提供商健康状态
func (a *TestKitProviderAdapter) IsHealthy() bool {
	// 简单的健康检查：检查提供商是否为 nil
	return a.testkitProvider != nil
}

// FetchStockData 获取股票实时数据
func (a *TestKitProviderAdapter) FetchStockData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	return a.testkitProvider.FetchData(ctx, symbols)
}

// FetchStockDataWithRaw 获取股票数据和原始响应
// testkit Provider 不支持原始数据，只返回处理后的数据
func (a *TestKitProviderAdapter) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]subscriber.StockData, string, error) {
	data, err := a.testkitProvider.FetchData(ctx, symbols)
	return data, "", err
}

// IsSymbolSupported 检查是否支持该股票代码
func (a *TestKitProviderAdapter) IsSymbolSupported(symbol string) bool {
	// testkit Provider 通常支持所有股票代码（用于测试）
	return true
}

// SetMockMode 设置Mock模式（委托给 testkit Provider）
func (a *TestKitProviderAdapter) SetMockMode(enabled bool) {
	a.testkitProvider.SetMockMode(enabled)
}

// SetMockData 设置Mock数据（委托给 testkit Provider）
func (a *TestKitProviderAdapter) SetMockData(symbols []string, data []subscriber.StockData) {
	a.testkitProvider.SetMockData(symbols, data)
}

// Close 关闭提供商
func (a *TestKitProviderAdapter) Close() error {
	return a.testkitProvider.Close()
}

// GetTestKitProvider 获取原始的 testkit Provider
// 用于需要直接访问 testkit Provider 特有功能的场景
func (a *TestKitProviderAdapter) GetTestKitProvider() core.Provider {
	return a.testkitProvider
}

// SmartProviderAdapter 智能提供商适配器
// 能够自动检测多种提供商类型并适配到统一接口
type SmartProviderAdapter struct {
	provider interface{}
	adapted  RealtimeStockProvider
}

// NewSmartProviderAdapter 创建智能提供商适配器
func NewSmartProviderAdapter(provider interface{}) *SmartProviderAdapter {
	adapter := &SmartProviderAdapter{
		provider: provider,
	}
	
	// 自动适配
	adapter.adapted = adapter.autoAdapt()
	
	return adapter
}

// autoAdapt 自动适配提供商
func (a *SmartProviderAdapter) autoAdapt() RealtimeStockProvider {
	// 如果已经是新接口，直接返回
	if newProvider, ok := a.provider.(RealtimeStockProvider); ok {
		return newProvider
	}

	// 如果是旧版 subscriber.Provider 接口
	if legacyProvider, ok := a.provider.(subscriber.Provider); ok {
		return NewLegacyProviderAdapter(legacyProvider)
	}

	// 如果是 testkit core.Provider 接口
	if testkitProvider, ok := a.provider.(core.Provider); ok {
		return NewTestKitProviderAdapter(testkitProvider)
	}

	// 不支持的类型，返回 nil
	return nil
}

// Name 返回提供商名称
func (a *SmartProviderAdapter) Name() string {
	if a.adapted != nil {
		return a.adapted.Name()
	}
	return "unknown_provider"
}

// GetRateLimit 获取请求频率限制
func (a *SmartProviderAdapter) GetRateLimit() time.Duration {
	if a.adapted != nil {
		return a.adapted.GetRateLimit()
	}
	return 200 * time.Millisecond
}

// IsHealthy 检查提供商健康状态
func (a *SmartProviderAdapter) IsHealthy() bool {
	if a.adapted != nil {
		return a.adapted.IsHealthy()
	}
	return false
}

// FetchStockData 获取股票实时数据
func (a *SmartProviderAdapter) FetchStockData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	if a.adapted != nil {
		return a.adapted.FetchStockData(ctx, symbols)
	}
	return nil, ErrProviderNotSupported
}

// FetchStockDataWithRaw 获取股票数据和原始响应
func (a *SmartProviderAdapter) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]subscriber.StockData, string, error) {
	if a.adapted != nil {
		return a.adapted.FetchStockDataWithRaw(ctx, symbols)
	}
	return nil, "", ErrProviderNotSupported
}

// IsSymbolSupported 检查是否支持该股票代码
func (a *SmartProviderAdapter) IsSymbolSupported(symbol string) bool {
	if a.adapted != nil {
		return a.adapted.IsSymbolSupported(symbol)
	}
	return false
}

// GetOriginalProvider 获取原始的提供商
func (a *SmartProviderAdapter) GetOriginalProvider() interface{} {
	return a.provider
}

// GetAdaptedProvider 获取适配后的提供商
func (a *SmartProviderAdapter) GetAdaptedProvider() RealtimeStockProvider {
	return a.adapted
}

// IsSupported 检查是否支持该提供商类型
func (a *SmartProviderAdapter) IsSupported() bool {
	return a.adapted != nil
}

// 确保适配器实现了所需的接口
var _ RealtimeStockProvider = (*TestKitProviderAdapter)(nil)
var _ RealtimeStockProvider = (*SmartProviderAdapter)(nil)
