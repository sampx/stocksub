package main

import (
	"context"
	"errors"
	"fmt"
	"stocksub/pkg/core"
	"stocksub/pkg/provider/decorators"
	"time"
)

// MockStockProvider 模拟股票数据提供商
type MockStockProvider struct {
	callCount    int
	failInterval int
}

func NewMockStockProvider(failInterval int) *MockStockProvider {
	return &MockStockProvider{
		callCount:    0,
		failInterval: failInterval, //模拟网络故障,如果 failInterval 设置为 2，则每第 2 次调用会失败
	}
}

func (p *MockStockProvider) Name() string {
	return "MockStockProvider"
}

func (p *MockStockProvider) IsHealthy() bool {
	return true
}

func (p *MockStockProvider) GetRateLimit() time.Duration {
	return 100 * time.Millisecond
}

func (p *MockStockProvider) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	p.callCount++

	// 模拟故障
	if p.failInterval > 0 && p.callCount%p.failInterval == 0 {
		return nil, errors.New("simulated network error")
	}

	// 模拟IP封禁
	if p.callCount%8 == 0 {
		return nil, errors.New("IP banned - too many requests")
	}

	mockData := []core.StockData{
		{Symbol: "000001.SZ", Name: "平安银行", Price: 12.34, Change: 0.56, ChangePercent: 4.75, Volume: 1000000, Timestamp: time.Now()},
		{Symbol: "000002.SZ", Name: "万科A", Price: 23.45, Change: -0.12, ChangePercent: -0.51, Volume: 800000, Timestamp: time.Now()},
	}

	result := make([]core.StockData, 0, len(symbols))
	for _, symbol := range symbols {
		for _, data := range mockData {
			if data.Symbol == symbol {
				newData := data
				newData.Timestamp = time.Now()
				result = append(result, newData)
				break
			}
		}
	}

	return result, nil
}

func (p *MockStockProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	data, err := p.FetchStockData(ctx, symbols)
	if err != nil {
		return nil, "", err
	}
	raw := fmt.Sprintf("Raw response for symbols: %v (call #%d)", symbols, p.callCount)
	return data, raw, nil
}

func (p *MockStockProvider) IsSymbolSupported(symbol string) bool {
	return true
}

func (p *MockStockProvider) GetCallCount() int {
	return p.callCount
}

func demonstrateFrequencyControl() {
	fmt.Println("=== 简化频率控制装饰器演示 ===")

	baseProvider := NewMockStockProvider(0) // 不模拟故障
	config := decorators.DefaultSimplifiedFrequencyControlConfig()
	// config.MinInterval = 200 * time.Millisecond
	// config.MarketTimeAware = true

	decoratedProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, config)

	fmt.Printf("装饰器: %s\n", decoratedProvider.Name())
	fmt.Printf("配置: 最小间隔=%v, 交易时段感知=%v\n", config.MinInterval, config.MarketTimeAware)

	ctx := context.Background()
	symbols := []string{"000001.SZ"}

	fmt.Println("连续请求测试:")
	for i := 1; i <= 3; i++ {
		start := time.Now()
		data, err := decoratedProvider.FetchStockData(ctx, symbols)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("  请求%d失败: %v\n", i, err)
		} else {
			fmt.Printf("  请求%d成功: 耗时=%v, 数据=%d条\n", i, elapsed, len(data))
		}
	}

	status := decoratedProvider.GetStatus()
	fmt.Printf("状态: 交易时段内=%v, IP封禁=%v\n",
		status["in_trading_window"], status["ip_ban_status"])
	fmt.Println()
}

func demonstrateCircuitBreaker() {
	fmt.Println("=== 简化熔断器装饰器演示 ===")

	baseProvider := NewMockStockProvider(2) // 每2次调用失败一次
	config := decorators.DefaultSimplifiedCircuitBreakerConfig()
	config.ReadyToTrip = 2
	config.Interval = 5 * time.Second
	config.Timeout = 3 * time.Second

	decoratedProvider := decorators.NewSimplifiedCircuitBreakerProvider(baseProvider, config)

	fmt.Printf("装饰器: %s\n", decoratedProvider.Name())
	fmt.Printf("配置: 失败阈值=%d, 统计窗口=%v, 熔断超时=%v\n",
		config.ReadyToTrip, config.Interval, config.Timeout)

	ctx := context.Background()
	symbols := []string{"000001.SZ"}

	fmt.Println("熔断器测试:")
	for i := 1; i <= 6; i++ {
		data, err := decoratedProvider.FetchStockData(ctx, symbols)
		state := decoratedProvider.GetState()

		if err != nil {
			fmt.Printf("  请求%d失败: %v (状态=%v)\n", i, err, state)
		} else {
			fmt.Printf("  请求%d成功: 数据=%d条 (状态=%v)\n", i, len(data), state)
		}

		if i == 4 {
			fmt.Println("  等待熔断恢复...")
			time.Sleep(4 * time.Second)
		}
	}

	status := decoratedProvider.GetStatus()
	stats := status["stats"].(map[string]interface{})
	fmt.Printf("统计: 总请求=%v, IP封禁=%v\n",
		stats["total_requests"], stats["ip_ban_count"])
	fmt.Println()
}

func demonstrateCombinedUsage() {
	fmt.Println("=== 组合使用演示 ===")

	baseProvider := NewMockStockProvider(3)

	// 创建频率控制装饰器
	freqConfig := decorators.DefaultSimplifiedFrequencyControlConfig()
	freqConfig.MinInterval = 100 * time.Millisecond
	freqProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, freqConfig)

	// 创建熔断器装饰器
	cbConfig := decorators.DefaultSimplifiedCircuitBreakerConfig()
	cbConfig.ReadyToTrip = 3
	cbProvider := decorators.NewSimplifiedCircuitBreakerProvider(freqProvider, cbConfig)

	fmt.Printf("组合装饰器: %s\n", cbProvider.Name())

	ctx := context.Background()
	symbols := []string{"000001.SZ", "000002.SZ"}

	fmt.Println("组合功能测试:")
	for i := 1; i <= 5; i++ {
		start := time.Now()
		data, err := cbProvider.FetchStockData(ctx, symbols)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("  请求%d失败: %v (耗时=%v)\n", i, err, elapsed)
		} else {
			fmt.Printf("  请求%d成功: 数据=%d条 (耗时=%v)\n", i, len(data), elapsed)
		}
	}

	fmt.Println("组合装饰器状态:")
	status := cbProvider.GetStatus()
	fmt.Printf("  熔断器状态: %v\n", status["state"])
	fmt.Printf("  交易时段内: %v\n", status["in_trading_window"])
	fmt.Println()
}

func demonstrateConfiguration() {
	fmt.Println("=== 配置系统演示 ===")

	// 使用配置系统创建装饰器
	config := decorators.DefaultSimplifiedConfig()
	config.Enabled = true
	config.FrequencyControl.MinInterval = 150 * time.Millisecond
	config.CircuitBreaker.ReadyToTrip = 2

	baseProvider := NewMockStockProvider(0)

	// 直接创建简化装饰器而不是使用通用配置方法
	freqConfig := decorators.DefaultSimplifiedFrequencyControlConfig()
	freqConfig.MinInterval = 150 * time.Millisecond
	cbConfig := decorators.DefaultSimplifiedCircuitBreakerConfig()
	cbConfig.ReadyToTrip = 2

	// 创建装饰器链
	freqProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, freqConfig)
	decoratedProvider := decorators.NewSimplifiedCircuitBreakerProvider(freqProvider, cbConfig)

	fmt.Printf("配置创建的装饰器: %s\n", decoratedProvider.Name())

	ctx := context.Background()
	symbols := []string{"000001.SZ"}

	// 测试配置的装饰器
	data, err := decoratedProvider.FetchStockData(ctx, symbols)
	if err != nil {
		fmt.Printf("配置装饰器请求失败: %v\n", err)
	} else {
		fmt.Printf("配置装饰器请求成功: 获取%d条数据\n", len(data))
	}
	fmt.Println()
}

func main() {
	fmt.Println("=== 简化装饰器综合示例 ===")
	fmt.Println("这个示例演示了简化频率控制和熔断器装饰器的完整功能")
	fmt.Println()

	demonstrateFrequencyControl()
	demonstrateCircuitBreaker()
	demonstrateCombinedUsage()
	demonstrateConfiguration()

	fmt.Println("=== 示例总结 ===")
	fmt.Println("✅ 简化频率控制装饰器:")
	fmt.Println("   - 自动频率控制和交易时段感知")
	fmt.Println("   - IP封禁检测和处理")
	fmt.Println("   - 灵活的配置选项")
	fmt.Println()
	fmt.Println("✅ 简化熔断器装饰器:")
	fmt.Println("   - 熔断保护和自动恢复")
	fmt.Println("   - IP封禁事件统计")
	fmt.Println("   - 交易时段感知")
	fmt.Println()
	fmt.Println("✅ 组合使用:")
	fmt.Println("   - 频率控制 + 熔断器协同工作")
	fmt.Println("   - 配置驱动的创建方式")
	fmt.Println("   - 完整的状态监控")
	fmt.Println()
	fmt.Println("所有简化装饰器示例完成！")
}
