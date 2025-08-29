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

// SimpleRealtimeStockProvider 简单的实时股票数据提供商实现
type SimpleRealtimeStockProvider struct {
	mockData []core.StockData
}

// NewSimpleRealtimeStockProvider 创建简单的实时股票数据提供商
func NewSimpleRealtimeStockProvider() *SimpleRealtimeStockProvider {
	return &SimpleRealtimeStockProvider{
		mockData: []core.StockData{
			{Symbol: "000001.SZ", Name: "平安银行", Price: 12.34, Change: 0.56, ChangePercent: 4.75, Volume: 1000000, Timestamp: time.Now()},
			{Symbol: "000002.SZ", Name: "万科A", Price: 23.45, Change: -0.12, ChangePercent: -0.51, Volume: 800000, Timestamp: time.Now()},
		},
	}
}

func (p *SimpleRealtimeStockProvider) Name() string {
	return "SimpleRealtimeStock"
}

func (p *SimpleRealtimeStockProvider) IsHealthy() bool {
	return true
}

func (p *SimpleRealtimeStockProvider) GetRateLimit() time.Duration {
	return 100 * time.Millisecond
}

func (p *SimpleRealtimeStockProvider) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	// 简单返回预设的数据
	result := make([]core.StockData, 0, len(symbols))
	for _, symbol := range symbols {
		for _, data := range p.mockData {
			if data.Symbol == symbol {
				// 添加一些随机变化
				newData := data
				newData.Timestamp = time.Now()
				result = append(result, newData)
				break
			}
		}
	}
	return result, nil
}

func (p *SimpleRealtimeStockProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	data, err := p.FetchStockData(ctx, symbols)
	if err != nil {
		return nil, "", err
	}
	raw := fmt.Sprintf("Raw response for symbols: %v", symbols)
	return data, raw, nil
}

func (p *SimpleRealtimeStockProvider) IsSymbolSupported(symbol string) bool {
	return true
}

// SimpleHistoricalProvider 简单的历史数据提供商
type SimpleHistoricalProvider struct{}

func NewSimpleHistoricalProvider() *SimpleHistoricalProvider {
	return &SimpleHistoricalProvider{}
}

func (p *SimpleHistoricalProvider) Name() string {
	return "SimpleHistorical"
}

func (p *SimpleHistoricalProvider) IsHealthy() bool {
	return true
}

func (p *SimpleHistoricalProvider) GetRateLimit() time.Duration {
	return 500 * time.Millisecond
}

func (p *SimpleHistoricalProvider) FetchHistoricalData(ctx context.Context, symbol string, start, end time.Time, period string) ([]core.HistoricalData, error) {
	// 生成一些模拟的历史数据
	result := []core.HistoricalData{
		{
			Symbol:    symbol,
			Timestamp: time.Now().Add(-24 * time.Hour),
			Open:      12.00,
			High:      12.50,
			Low:       11.80,
			Close:     12.34,
			Volume:    1000000,
			Turnover:  150000.0,
			Period:    period,
		},
	}
	return result, nil
}

func (p *SimpleHistoricalProvider) GetSupportedPeriods() []string {
	return []string{"daily", "hourly"}
}

// MockProviderAdapter 适配器，将MockProvider适配为RealtimeStockProvider
type MockProviderAdapter struct {
	*providers.MockProvider
}

// 实现 provider.RealtimeStockProvider 接口
func (m *MockProviderAdapter) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	return m.MockProvider.FetchData(ctx, symbols)
}

func (m *MockProviderAdapter) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	data, err := m.MockProvider.FetchData(ctx, symbols)
	if err != nil {
		return nil, "", err
	}
	raw := fmt.Sprintf("Mock raw response for: %v", symbols)
	return data, raw, nil
}

// 实现 provider.Provider 接口的其他方法
func (m *MockProviderAdapter) Name() string {
	return m.MockProvider.Name()
}

func (m *MockProviderAdapter) IsHealthy() bool {
	return true // MockProvider 总是健康的
}

func (m *MockProviderAdapter) GetRateLimit() time.Duration {
	return m.MockProvider.GetRateLimit()
}

func (m *MockProviderAdapter) IsSymbolSupported(symbol string) bool {
	return m.MockProvider.IsSymbolSupported(symbol)
}

func main() {
	fmt.Println("=== 频率控制装饰器示例 ===")

	// 创建简单股票数据提供商
	simpleProvider := NewSimpleRealtimeStockProvider()
	
	// 创建频率控制配置
	config := &decorators.FrequencyControlConfig{
		MinInterval: 500 * time.Millisecond, // 最小间隔500ms
		MaxRetries:  3,                      // 最大重试3次
		Enabled:     true,                   // 启用频率控制
	}

	// 使用频率控制装饰器包装提供商
	decoratedProvider := decorators.NewFrequencyControlProvider(simpleProvider, config)

	fmt.Printf("装饰器名称: %s\n", decoratedProvider.Name())
	fmt.Printf("频率限制: %v\n", decoratedProvider.GetRateLimit())
	fmt.Printf("健康状态: %v\n", decoratedProvider.IsHealthy())
	fmt.Println()

	// 演示频率控制效果
	symbols := []string{"000001.SZ", "000002.SZ"}
	ctx := context.Background()

	fmt.Println("=== 测试频率控制效果 ===")
	
	// 连续发起多次请求，观察频率控制效果
	for i := 1; i <= 5; i++ {
		start := time.Now()
		
		fmt.Printf("第%d次请求开始...\n", i)
		data, err := decoratedProvider.FetchStockData(ctx, symbols)
		elapsed := time.Since(start)
		
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
		} else {
			fmt.Printf("成功获取%d条数据，耗时: %v\n", len(data), elapsed)
			for _, stock := range data {
				fmt.Printf("  %s: ¥%.2f (变化: %.2f, %.2f%%)\n", 
					stock.Symbol, stock.Price, stock.Change, stock.ChangePercent)
			}
		}
		fmt.Println()
	}

	// 演示配置调整
	fmt.Println("=== 动态调整配置 ===")
	fmt.Println("调整最小间隔为1秒...")
	decoratedProvider.SetMinInterval(1 * time.Second)
	
	fmt.Println("禁用频率控制...")
	decoratedProvider.SetEnabled(false)
	
	start := time.Now()
	stockData, err := decoratedProvider.FetchStockData(ctx, symbols)
	elapsed := time.Since(start)
	
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
	} else {
		fmt.Printf("禁用频率控制后，请求耗时: %v (应该很快), 获取%d条数据\n", elapsed, len(stockData))
	}

	// 显示状态信息
	fmt.Println("\n=== 装饰器状态信息 ===")
	status := decoratedProvider.GetStatus()
	for key, value := range status {
		fmt.Printf("%s: %v\n", key, value)
	}

	fmt.Println("\n=== 测试历史数据提供商的频率控制 ===")
	
	// 创建简单历史数据提供商
	simpleHistoricalProvider := NewSimpleHistoricalProvider()
	
	// 使用频率控制装饰历史数据提供商
	historicalDecorated := decorators.NewFrequencyControlForHistoricalProvider(
		simpleHistoricalProvider, 
		&decorators.FrequencyControlConfig{
			MinInterval: 1 * time.Second, // 历史数据使用更长的间隔
			MaxRetries:  3,
			Enabled:     true,
		},
	)

	fmt.Printf("历史数据装饰器名称: %s\n", historicalDecorated.Name())
	fmt.Printf("健康状态: %v\n", historicalDecorated.IsHealthy())

	// 测试历史数据获取
	start = time.Now()
	endTime := time.Now()
	startTime := endTime.Add(-30 * 24 * time.Hour) // 获取30天的历史数据
	
	// 需要类型转换才能调用 FetchHistoricalData
	if histProvider, ok := historicalDecorated.(provider.HistoricalProvider); ok {
		historicalData, err := histProvider.FetchHistoricalData(ctx, "000001.SZ", startTime, endTime, "daily")
		elapsed = time.Since(start)
		
		if err != nil {
			fmt.Printf("获取历史数据失败: %v\n", err)
		} else {
			fmt.Printf("成功获取%d条历史数据，耗时: %v\n", len(historicalData), elapsed)
			if len(historicalData) > 0 {
				fmt.Printf("最新数据: %s - 开盘:%.2f 最高:%.2f 最低:%.2f 收盘:%.2f\n",
					historicalData[0].Timestamp.Format("2006-01-02"),
					historicalData[0].Open, historicalData[0].High, 
					historicalData[0].Low, historicalData[0].Close)
			}
		}
	} else {
		fmt.Println("类型转换失败：无法调用历史数据方法")
	}
	
	fmt.Println("\n=== 使用通用Mock Provider ===")
	
	// 创建通用Mock Provider
	mockProvider := providers.NewMockProvider(providers.DefaultMockProviderConfig())
	
	// 设置mock数据
	mockProvider.SetMockData([]string{"600000.SH", "000001.SZ"}, []core.StockData{
		{Symbol: "600000.SH", Name: "浦发银行", Price: 8.90, Change: 0.15, ChangePercent: 1.71, Volume: 2000000, Timestamp: time.Now()},
		{Symbol: "000001.SZ", Name: "平安银行", Price: 12.34, Change: 0.56, ChangePercent: 4.75, Volume: 1500000, Timestamp: time.Now()},
	})
	
	// 包装Mock Provider（需要实现RealtimeStockProvider接口的适配器）
	mockAdapter := &MockProviderAdapter{mockProvider}
	mockDecoratedProvider := decorators.NewFrequencyControlProvider(mockAdapter, config)
	
	fmt.Printf("Mock装饰器名称: %s\n", mockDecoratedProvider.Name())
	
	testSymbols := []string{"600000.SH", "000001.SZ"}
	mockData, err := mockDecoratedProvider.FetchStockData(ctx, testSymbols)
	if err != nil {
		fmt.Printf("Mock Provider请求失败: %v\n", err)
	} else {
		fmt.Printf("Mock Provider成功获取%d条数据\n", len(mockData))
		for _, stock := range mockData {
			fmt.Printf("  %s (%s): ¥%.2f (变化: %.2f, %.2f%%)\n", 
				stock.Symbol, stock.Name, stock.Price, stock.Change, stock.ChangePercent)
		}
	}
	
	fmt.Println("\n频率控制装饰器示例完成！")
}