package main

import (
	"context"
	"fmt"
	"stocksub/pkg/core"
	"stocksub/pkg/provider"
	"stocksub/pkg/provider/decorators"
	"time"
)

// TestableStockProvider 可测试的股票数据提供商
type TestableStockProvider struct {
	name         string
	shouldFail   bool
	failureCount int
	delay        time.Duration
}

// NewTestableStockProvider 创建可测试的股票数据提供商
func NewTestableStockProvider(name string) *TestableStockProvider {
	return &TestableStockProvider{
		name:         name,
		shouldFail:   false,
		failureCount: 0,
		delay:        50 * time.Millisecond,
	}
}

func (p *TestableStockProvider) Name() string {
	return p.name
}

func (p *TestableStockProvider) IsHealthy() bool {
	return !p.shouldFail
}

func (p *TestableStockProvider) GetRateLimit() time.Duration {
	return 100 * time.Millisecond
}

func (p *TestableStockProvider) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	// 模拟延迟
	if p.delay > 0 {
		time.Sleep(p.delay)
	}

	// 模拟失败
	if p.shouldFail {
		p.failureCount++
		return nil, fmt.Errorf("模拟API错误 - 提供商: %s, 失败次数: %d", p.name, p.failureCount)
	}

	// 返回成功数据
	result := make([]core.StockData, 0, len(symbols))
	for i, symbol := range symbols {
		result = append(result, core.StockData{
			Symbol:        symbol,
			Name:          fmt.Sprintf("股票%s", symbol),
			Price:         10.0 + float64(i),
			Change:        0.5,
			ChangePercent: 5.0,
			Volume:        1000000,
			Timestamp:     time.Now(),
		})
	}
	return result, nil
}

func (p *TestableStockProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	data, err := p.FetchStockData(ctx, symbols)
	if err != nil {
		return nil, "", err
	}
	raw := fmt.Sprintf("Raw response from %s for symbols: %v", p.name, symbols)
	return data, raw, nil
}

func (p *TestableStockProvider) IsSymbolSupported(symbol string) bool {
	return true
}

// SetFailureMode 设置失败模式
func (p *TestableStockProvider) SetFailureMode(shouldFail bool) {
	p.shouldFail = shouldFail
	if !shouldFail {
		p.failureCount = 0
	}
}

// SetDelay 设置延迟
func (p *TestableStockProvider) SetDelay(delay time.Duration) {
	p.delay = delay
}

func main() {
	fmt.Println("=== 装饰器链组合示例 ===")

	// 创建基础提供商
	baseProvider := NewTestableStockProvider("BaseStockProvider")

	// 方式1: 手动组合装饰器链
	fmt.Println("\n=== 手动组合装饰器链 ===")
	
	// 频率控制配置
	fcConfig := &decorators.FrequencyControlConfig{
		MinInterval: 300 * time.Millisecond,
		MaxRetries:  2,
		Enabled:     true,
	}

	// 熔断器配置
	cbConfig := &decorators.CircuitBreakerConfig{
		Name:        "ChainedCircuitBreaker",
		MaxRequests: 3,
		Interval:    5 * time.Second,
		Timeout:     3 * time.Second,
		ReadyToTrip: 2,
		Enabled:     true,
	}

	// 先应用频率控制，再应用熔断器
	frequencyControlled := decorators.NewFrequencyControlProvider(baseProvider, fcConfig)
	fullyDecorated := decorators.NewCircuitBreakerProvider(frequencyControlled, cbConfig)

	fmt.Printf("装饰器链: %s\n", fullyDecorated.Name())
	fmt.Printf("健康状态: %v\n", fullyDecorated.IsHealthy())

	symbols := []string{"000001.SZ", "000002.SZ"}
	ctx := context.Background()

	// 测试正常情况
	fmt.Println("\n--- 测试正常请求 ---")
	for i := 1; i <= 3; i++ {
		start := time.Now()
		
		data, err := fullyDecorated.FetchStockData(ctx, symbols)
		elapsed := time.Since(start)
		
		if err != nil {
			fmt.Printf("第%d次请求失败: %v, 耗时: %v\n", i, err, elapsed)
		} else {
			fmt.Printf("第%d次请求成功: 获取%d条数据, 耗时: %v\n", i, len(data), elapsed)
		}
	}

	// 测试失败情况
	fmt.Println("\n--- 测试装饰器链的错误处理 ---")
	baseProvider.SetFailureMode(true)
	
	for i := 1; i <= 5; i++ {
		data, err := fullyDecorated.FetchStockData(ctx, symbols)
		
		if err != nil {
			fmt.Printf("失败测试第%d次: %v\n", i, err)
		} else {
			fmt.Printf("失败测试第%d次: 成功获取%d条数据\n", i, len(data))
		}

		// 显示熔断器状态
		fmt.Printf("  熔断器状态: %s\n", fullyDecorated.GetState())
		
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("\n=== 使用配置驱动的装饰器链 ===")

	// 方式2: 使用配置驱动的装饰器链
	baseProvider2 := NewTestableStockProvider("ConfigDrivenProvider")
	baseProvider2.SetFailureMode(false) // 确保基础提供商正常

	// 创建装饰器配置
	decoratorConfig := provider.ProviderDecoratorConfig{
		Realtime: []provider.DecoratorConfig{
			{
				Type:         provider.FrequencyControlType,
				Enabled:      true,
				Priority:     1, // 优先级1，最先应用
				ProviderType: "realtime",
				Config: map[string]interface{}{
					"min_interval_ms": 200,
					"max_retries":     3,
					"enabled":         true,
				},
			},
			{
				Type:         provider.CircuitBreakerType,
				Enabled:      true,
				Priority:     2, // 优先级2，后应用
				ProviderType: "realtime",
				Config: map[string]interface{}{
					"name":          "ConfigDrivenCB",
					"max_requests":  2,
					"interval":      "10s",
					"timeout":       "5s",
					"ready_to_trip": 3,
					"enabled":       true,
				},
			},
		},
	}

	// 使用配置创建装饰器链
	configDecorated, err := decorators.CreateDecoratedProvider(baseProvider2, decoratorConfig)
	if err != nil {
		fmt.Printf("创建配置驱动装饰器失败: %v\n", err)
		return
	}

	fmt.Printf("配置驱动装饰器链: %s\n", configDecorated.Name())
	
	// 测试配置驱动的装饰器链
	fmt.Println("\n--- 测试配置驱动装饰器链 ---")
	
	if realtimeProvider, ok := configDecorated.(provider.RealtimeStockProvider); ok {
		for i := 1; i <= 4; i++ {
			start := time.Now()
			
			data, err := realtimeProvider.FetchStockData(ctx, symbols)
			elapsed := time.Since(start)
			
			if err != nil {
				fmt.Printf("配置链第%d次请求失败: %v, 耗时: %v\n", i, err, elapsed)
			} else {
				fmt.Printf("配置链第%d次请求成功: 获取%d条数据, 耗时: %v\n", i, len(data), elapsed)
			}
			
			time.Sleep(100 * time.Millisecond)
		}
	}

	fmt.Println("\n=== 测试不同装饰器组合顺序 ===")

	// 测试不同的装饰器组合顺序
	baseProvider3 := NewTestableStockProvider("OrderTestProvider")
	
	// 顺序1: 频率控制 -> 熔断器
	fc1 := decorators.NewFrequencyControlProvider(baseProvider3, fcConfig)
	order1 := decorators.NewCircuitBreakerProvider(fc1, cbConfig)
	
	// 顺序2: 熔断器 -> 频率控制 (需要重新创建配置，避免名称冲突)
	cbConfig2 := &decorators.CircuitBreakerConfig{
		Name:        "OuterCircuitBreaker",
		MaxRequests: 3,
		Interval:    5 * time.Second,
		Timeout:     3 * time.Second,
		ReadyToTrip: 2,
		Enabled:     true,
	}
	
	baseProvider4 := NewTestableStockProvider("OrderTestProvider2")
	cb2 := decorators.NewCircuitBreakerProvider(baseProvider4, cbConfig2)
	order2 := decorators.NewFrequencyControlProvider(cb2, fcConfig)

	fmt.Printf("顺序1 (频率控制->熔断器): %s\n", order1.Name())
	fmt.Printf("顺序2 (熔断器->频率控制): %s\n", order2.Name())

	// 比较两种顺序的性能
	fmt.Println("\n--- 性能比较测试 ---")
	
	testOrders := []struct {
		name     string
		provider provider.RealtimeStockProvider
	}{
		{"频率控制->熔断器", order1},
		{"熔断器->频率控制", order2},
	}

	for _, test := range testOrders {
		fmt.Printf("\n测试 %s:\n", test.name)
		
		totalTime := time.Duration(0)
		successCount := 0
		
		for i := 1; i <= 3; i++ {
			start := time.Now()
			
			data, err := test.provider.FetchStockData(ctx, symbols)
			elapsed := time.Since(start)
			totalTime += elapsed
			
			if err != nil {
				fmt.Printf("  第%d次: 失败 - %v, 耗时: %v\n", i, err, elapsed)
			} else {
				fmt.Printf("  第%d次: 成功 - %d条数据, 耗时: %v\n", i, len(data), elapsed)
				successCount++
			}
		}
		
		avgTime := totalTime / 3
		fmt.Printf("  平均耗时: %v, 成功率: %d/3\n", avgTime, successCount)
	}

	fmt.Println("\n=== 复杂场景测试 ===")

	// 创建一个更复杂的测试场景
	complexProvider := NewTestableStockProvider("ComplexScenarioProvider")
	complexProvider.SetDelay(100 * time.Millisecond) // 设置一定延迟

	// 创建复杂的装饰器链配置
	complexConfig := provider.ProviderDecoratorConfig{
		All: []provider.DecoratorConfig{
			{
				Type:         provider.CircuitBreakerType,
				Enabled:      true,
				Priority:     3, // 最外层的熔断器
				ProviderType: "all",
				Config: map[string]interface{}{
					"name":          "GlobalCircuitBreaker",
					"max_requests":  5,
					"interval":      "30s",
					"timeout":       "10s",
					"ready_to_trip": 5,
					"enabled":       true,
				},
			},
		},
		Realtime: []provider.DecoratorConfig{
			{
				Type:         provider.FrequencyControlType,
				Enabled:      true,
				Priority:     1, // 最内层的频率控制
				ProviderType: "realtime",
				Config: map[string]interface{}{
					"min_interval_ms": 150,
					"max_retries":     2,
					"enabled":         true,
				},
			},
			{
				Type:         provider.CircuitBreakerType,
				Enabled:      true,
				Priority:     2, // 中间层的熔断器
				ProviderType: "realtime",
				Config: map[string]interface{}{
					"name":          "RealtimeCircuitBreaker",
					"max_requests":  3,
					"interval":      "15s",
					"timeout":       "5s",
					"ready_to_trip": 3,
					"enabled":       true,
				},
			},
		},
	}

	complexDecorated, err := decorators.CreateDecoratedProvider(complexProvider, complexConfig)
	if err != nil {
		fmt.Printf("创建复杂装饰器链失败: %v\n", err)
		return
	}

	fmt.Printf("复杂装饰器链: %s\n", complexDecorated.Name())

	// 测试复杂场景
	if realtimeProvider, ok := complexDecorated.(provider.RealtimeStockProvider); ok {
		fmt.Println("\n--- 复杂场景正常测试 ---")
		
		for i := 1; i <= 3; i++ {
			start := time.Now()
			
			data, err := realtimeProvider.FetchStockData(ctx, symbols)
			elapsed := time.Since(start)
			
			if err != nil {
				fmt.Printf("复杂链第%d次请求失败: %v, 耗时: %v\n", i, err, elapsed)
			} else {
				fmt.Printf("复杂链第%d次请求成功: 获取%d条数据, 耗时: %v\n", i, len(data), elapsed)
			}
		}
	}

	fmt.Println("\n装饰器链组合示例完成！")
	fmt.Println("\n=== 总结 ===")
	fmt.Println("1. 手动组合：直接创建装饰器实例并嵌套")
	fmt.Println("2. 配置驱动：使用配置文件定义装饰器链")
	fmt.Println("3. 顺序影响：不同的装饰器顺序会影响行为和性能")
	fmt.Println("4. 复杂组合：可以创建多层次的装饰器结构")
}