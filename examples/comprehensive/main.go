package main

import (
	"context"
	"fmt"
	"stocksub/pkg/core"
	"stocksub/pkg/provider"
	"stocksub/pkg/provider/decorators"
	"stocksub/pkg/testkit/providers"
	"time"
)

// ComprehensiveExample 综合示例演示如何结合使用提供商和装饰器
type ComprehensiveExample struct {
	providerManager *ProviderManager
}

// ProviderManager 简化的提供商管理器
type ProviderManager struct {
	realtimeProviders map[string]provider.RealtimeStockProvider
	historicalProviders map[string]provider.HistoricalProvider
	indexProviders map[string]provider.RealtimeIndexProvider
}

// NewProviderManager 创建新的提供商管理器
func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		realtimeProviders: make(map[string]provider.RealtimeStockProvider),
		historicalProviders: make(map[string]provider.HistoricalProvider),
		indexProviders: make(map[string]provider.RealtimeIndexProvider),
	}
}

// RegisterRealtimeProvider 注册实时股票提供商
func (pm *ProviderManager) RegisterRealtimeProvider(name string, p provider.RealtimeStockProvider) {
	pm.realtimeProviders[name] = p
}

// RegisterHistoricalProvider 注册历史数据提供商
func (pm *ProviderManager) RegisterHistoricalProvider(name string, p provider.HistoricalProvider) {
	pm.historicalProviders[name] = p
}

// RegisterIndexProvider 注册指数提供商
func (pm *ProviderManager) RegisterIndexProvider(name string, p provider.RealtimeIndexProvider) {
	pm.indexProviders[name] = p
}

// GetRealtimeProvider 获取实时股票提供商
func (pm *ProviderManager) GetRealtimeProvider(name string) (provider.RealtimeStockProvider, bool) {
	p, ok := pm.realtimeProviders[name]
	return p, ok
}

// GetHistoricalProvider 获取历史数据提供商
func (pm *ProviderManager) GetHistoricalProvider(name string) (provider.HistoricalProvider, bool) {
	p, ok := pm.historicalProviders[name]
	return p, ok
}

// GetIndexProvider 获取指数提供商
func (pm *ProviderManager) GetIndexProvider(name string) (provider.RealtimeIndexProvider, bool) {
	p, ok := pm.indexProviders[name]
	return p, ok
}

// NewComprehensiveExample 创建综合示例
func NewComprehensiveExample() *ComprehensiveExample {
	return &ComprehensiveExample{
		providerManager: NewProviderManager(),
	}
}

// MockRealtimeStockProviderAdapter 适配器，将MockProvider适配为RealtimeStockProvider
type MockRealtimeStockProviderAdapter struct {
	*providers.MockProvider
}

func (m *MockRealtimeStockProviderAdapter) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	return m.MockProvider.FetchData(ctx, symbols)
}

func (m *MockRealtimeStockProviderAdapter) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	data, err := m.MockProvider.FetchData(ctx, symbols)
	if err != nil {
		return nil, "", err
	}
	raw := fmt.Sprintf("Mock raw response for: %v", symbols)
	return data, raw, nil
}

func (m *MockRealtimeStockProviderAdapter) IsSymbolSupported(symbol string) bool {
	return true
}

// MockIndexProvider 简单的指数提供商实现
type MockIndexProvider struct{}

func (m *MockIndexProvider) Name() string {
	return "MockIndex"
}

func (m *MockIndexProvider) IsHealthy() bool {
	return true
}

func (m *MockIndexProvider) GetRateLimit() time.Duration {
	return 500 * time.Millisecond
}

func (m *MockIndexProvider) FetchIndexData(ctx context.Context, indexSymbols []string) ([]core.IndexData, error) {
	result := make([]core.IndexData, 0, len(indexSymbols))
	
	for _, symbol := range indexSymbols {
		var name string
		switch symbol {
		case "sh000001":
			name = "上证指数"
		case "sz399001":
			name = "深证成指"
		default:
			name = fmt.Sprintf("指数%s", symbol)
		}
		
		result = append(result, core.IndexData{
			Symbol:        symbol,
			Name:          name,
			Value:         3000.0 + float64(len(result))*100,
			Change:        15.5 + float64(len(result))*2,
			ChangePercent: 0.5 + float64(len(result))*0.1,
			Volume:        int64(100000000 + len(result)*10000000),
			Turnover:      1000000000.0 + float64(len(result))*100000000,
		})
	}
	
	return result, nil
}

func (m *MockIndexProvider) IsIndexSupported(indexSymbol string) bool {
	return true
}

// SetupProviders 设置提供商
func (ce *ComprehensiveExample) SetupProviders() {
	fmt.Println("=== 设置提供商 ===")
	
	// 1. 创建基础提供商
	mockProvider := providers.NewMockProvider(providers.DefaultMockProviderConfig())
	mockProvider.SetMockData([]string{"000001.SZ", "000002.SZ"}, []core.StockData{
		{Symbol: "000001.SZ", Name: "平安银行", Price: 12.34, Change: 0.56, ChangePercent: 4.75, Volume: 1000000, Timestamp: time.Now()},
		{Symbol: "000002.SZ", Name: "万科A", Price: 23.45, Change: -0.12, ChangePercent: -0.51, Volume: 800000, Timestamp: time.Now()},
	})
	
	// 创建适配器
	mockRealtimeProvider := &MockRealtimeStockProviderAdapter{MockProvider: mockProvider}
	
	// 2. 为实时提供商创建装饰器链（使用测试配置禁用限流器）
	realtimeDecorators := decorators.TestDecoratorConfig()
	
	decoratedRealtimeProvider, err := decorators.CreateDecoratedProvider(mockRealtimeProvider, realtimeDecorators)
	if err != nil {
		fmt.Printf("创建实时提供商装饰器失败: %v\n", err)
		return
	}
	
	// 3. 注册提供商
	ce.providerManager.RegisterRealtimeProvider("mock", decoratedRealtimeProvider.(provider.RealtimeStockProvider))
	
	// 4. 创建历史数据提供商
	historicalProvider := &MockHistoricalProvider{}
	historicalDecorators := decorators.TestDecoratorConfig()
	
	decoratedHistoricalProvider, err := decorators.CreateDecoratedProvider(historicalProvider, historicalDecorators)
	if err != nil {
		fmt.Printf("创建历史提供商装饰器失败: %v\n", err)
		return
	}
	
	ce.providerManager.RegisterHistoricalProvider("mock", decoratedHistoricalProvider.(provider.HistoricalProvider))
	
	// 5. 创建指数提供商
	indexProvider := &MockIndexProvider{}
	indexDecorators := decorators.TestDecoratorConfig()
	
	decoratedIndexProvider, err := decorators.CreateDecoratedProvider(indexProvider, indexDecorators)
	if err != nil {
		fmt.Printf("创建指数提供商装饰器失败: %v\n", err)
		return
	}
	
	ce.providerManager.RegisterIndexProvider("mock", decoratedIndexProvider.(provider.RealtimeIndexProvider))
	
	fmt.Println("提供商设置完成")
}

// DemonstrateRealtimeData 演示实时数据获取
func (ce *ComprehensiveExample) DemonstrateRealtimeData() {
	fmt.Println("\n=== 演示实时数据获取 ===")
	
	provider, ok := ce.providerManager.GetRealtimeProvider("mock")
	if !ok {
		fmt.Println("未找到实时提供商")
		return
	}
	
	ctx := context.Background()
	symbols := []string{"000001.SZ", "000002.SZ"}
	
	fmt.Printf("提供商名称: %s\n", provider.Name())
	fmt.Printf("健康状态: %v\n", provider.IsHealthy())
	fmt.Printf("频率限制: %v\n", provider.GetRateLimit())
	
	// 获取数据
	data, err := provider.FetchStockData(ctx, symbols)
	if err != nil {
		fmt.Printf("获取实时数据失败: %v\n", err)
		return
	}
	
	fmt.Printf("成功获取%d条实时数据:\n", len(data))
	for _, stock := range data {
		fmt.Printf("  %s (%s): ¥%.2f (变化: %.2f, %.2f%%)\n", 
			stock.Symbol, stock.Name, stock.Price, stock.Change, stock.ChangePercent)
	}
}

// DemonstrateHistoricalData 演示历史数据获取
func (ce *ComprehensiveExample) DemonstrateHistoricalData() {
	fmt.Println("\n=== 演示历史数据获取 ===")
	
	provider, ok := ce.providerManager.GetHistoricalProvider("mock")
	if !ok {
		fmt.Println("未找到历史数据提供商")
		return
	}
	
	ctx := context.Background()
	symbol := "000001.SZ"
	endTime := time.Now()
	startTime := endTime.Add(-7 * 24 * time.Hour) // 7天前
	
	fmt.Printf("提供商名称: %s\n", provider.Name())
	fmt.Printf("健康状态: %v\n", provider.IsHealthy())
	fmt.Printf("频率限制: %v\n", provider.GetRateLimit())
	
	// 获取历史数据
	data, err := provider.FetchHistoricalData(ctx, symbol, startTime, endTime, "daily")
	if err != nil {
		fmt.Printf("获取历史数据失败: %v\n", err)
		return
	}
	
	fmt.Printf("成功获取%d条历史数据:\n", len(data))
	for _, hist := range data {
		fmt.Printf("  %s: 开盘%.2f 最高%.2f 最低%.2f 收盘%.2f 成交量%d\n",
			hist.Timestamp.Format("2006-01-02"), hist.Open, hist.High, hist.Low, hist.Close, hist.Volume)
	}
}

// DemonstrateIndexData 演示指数数据获取
func (ce *ComprehensiveExample) DemonstrateIndexData() {
	fmt.Println("\n=== 演示指数数据获取 ===")
	
	provider, ok := ce.providerManager.GetIndexProvider("mock")
	if !ok {
		fmt.Println("未找到指数提供商")
		return
	}
	
	ctx := context.Background()
	symbols := []string{"sh000001", "sz399001"}
	
	fmt.Printf("提供商名称: %s\n", provider.Name())
	fmt.Printf("健康状态: %v\n", provider.IsHealthy())
	fmt.Printf("频率限制: %v\n", provider.GetRateLimit())
	
	// 获取指数数据
	data, err := provider.FetchIndexData(ctx, symbols)
	if err != nil {
		fmt.Printf("获取指数数据失败: %v\n", err)
		return
	}
	
	fmt.Printf("成功获取%d条指数数据:\n", len(data))
	for _, index := range data {
		fmt.Printf("  %s (%s): %.2f (变化: %.2f, %.2f%%)\n",
			index.Symbol, index.Name, index.Value, index.Change, index.ChangePercent)
	}
}

// DemonstrateDecoratorChain 演示装饰器链效果
func (ce *ComprehensiveExample) DemonstrateDecoratorChain() {
	fmt.Println("\n=== 演示装饰器链效果 ===")
	
	provider, ok := ce.providerManager.GetRealtimeProvider("mock")
	if !ok {
		fmt.Println("未找到实时提供商")
		return
	}
	
	// 检查装饰器链状态
	fmt.Printf("装饰器链: %s\n", provider.Name())
	
	// 检查是否是熔断器装饰器
	if cbProvider, ok := provider.(interface{ GetState() string }); ok {
		fmt.Printf("熔断器状态: %s\n", cbProvider.GetState())
	}
	
	// 检查是否是频率控制装饰器
	if fcProvider, ok := provider.(interface{ GetStatus() map[string]interface{} }); ok {
		status := fcProvider.GetStatus()
		fmt.Printf("频率控制状态: %+v\n", status)
	}
}

// MockHistoricalProvider 简单的历史数据提供商实现
type MockHistoricalProvider struct{}

func (m *MockHistoricalProvider) Name() string {
	return "MockHistorical"
}

func (m *MockHistoricalProvider) IsHealthy() bool {
	return true
}

func (m *MockHistoricalProvider) GetRateLimit() time.Duration {
	return 1 * time.Second
}

func (m *MockHistoricalProvider) FetchHistoricalData(ctx context.Context, symbol string, start, end time.Time, period string) ([]core.HistoricalData, error) {
	// 生成一些模拟的历史数据
	result := make([]core.HistoricalData, 0)
	current := start
	
	for i := 0; i < 7 && current.Before(end); i++ {
		price := 10.0 + float64(i)
		result = append(result, core.HistoricalData{
			Symbol:    symbol,
			Timestamp: current,
			Open:      price,
			High:      price + 0.5,
			Low:       price - 0.3,
			Close:     price + 0.2,
			Volume:    int64(1000000 + i*100000),
			Turnover:  0,
			Period:    period,
		})
		current = current.Add(24 * time.Hour)
	}
	
	return result, nil
}

func (m *MockHistoricalProvider) GetSupportedPeriods() []string {
	return []string{"daily", "weekly", "monthly"}
}

func main() {
	fmt.Println("=== 综合示例：提供商与装饰器的结合使用 ===")
	
	// 创建综合示例
	example := NewComprehensiveExample()
	
	// 设置提供商
	example.SetupProviders()
	
	// 演示实时数据获取
	example.DemonstrateRealtimeData()
	
	// 演示历史数据获取
	example.DemonstrateHistoricalData()
	
	// 演示指数数据获取
	example.DemonstrateIndexData()
	
	// 演示装饰器链效果
	example.DemonstrateDecoratorChain()
	
	fmt.Println("\n=== 综合示例完成 ===")
	fmt.Println("\n这个示例展示了:")
	fmt.Println("1. 如何设置不同类型的提供商（实时、历史、指数）")
	fmt.Println("2. 如何为不同类型的提供商应用不同的装饰器")
	fmt.Println("3. 如何使用装饰器链来增强提供商功能")
	fmt.Println("4. 如何统一管理多种类型的提供商")
	fmt.Println("5. 如何在实际场景中结合使用提供商和装饰器")
}