package main

import (
	"context"
	"fmt"
	"stocksub/pkg/core"
	"stocksub/pkg/provider/decorators"
	"time"
)

// UnreliableStockProvider 不可靠的股票数据提供商，用于演示熔断器
type UnreliableStockProvider struct {
	failureCount int
	maxFailures  int
	shouldFail   bool
}

// NewUnreliableStockProvider 创建不可靠的股票数据提供商
func NewUnreliableStockProvider(maxFailures int) *UnreliableStockProvider {
	return &UnreliableStockProvider{
		maxFailures: maxFailures,
		shouldFail:  true, // 默认开始就失败
	}
}

func (p *UnreliableStockProvider) Name() string {
	return "UnreliableStock"
}

func (p *UnreliableStockProvider) IsHealthy() bool {
	return !p.shouldFail
}

func (p *UnreliableStockProvider) GetRateLimit() time.Duration {
	return 100 * time.Millisecond
}

func (p *UnreliableStockProvider) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	// 模拟失败的逻辑
	if p.shouldFail && p.failureCount < p.maxFailures {
		p.failureCount++
		return nil, fmt.Errorf("模拟的API错误 - 失败次数: %d", p.failureCount)
	}

	// 达到最大失败次数后开始返回成功
	if p.failureCount >= p.maxFailures {
		p.shouldFail = false
	}

	// 返回成功的数据
	result := make([]core.StockData, 0, len(symbols))
	for _, symbol := range symbols {
		result = append(result, core.StockData{
			Symbol:        symbol,
			Name:          fmt.Sprintf("股票%s", symbol),
			Price:         12.34 + float64(len(result)),
			Change:        0.56,
			ChangePercent: 4.75,
			Volume:        1000000,
			Timestamp:     time.Now(),
		})
	}
	return result, nil
}

func (p *UnreliableStockProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	data, err := p.FetchStockData(ctx, symbols)
	if err != nil {
		return nil, "", err
	}
	raw := fmt.Sprintf("Raw response for symbols: %v", symbols)
	return data, raw, nil
}

func (p *UnreliableStockProvider) IsSymbolSupported(symbol string) bool {
	return true
}

// SetFailureMode 设置失败模式
func (p *UnreliableStockProvider) SetFailureMode(shouldFail bool) {
	p.shouldFail = shouldFail
	if !shouldFail {
		p.failureCount = 0
	}
}

// GetFailureCount 获取失败次数
func (p *UnreliableStockProvider) GetFailureCount() int {
	return p.failureCount
}

func main() {
	fmt.Println("=== 熔断器装饰器示例 ===")

	// 创建不可靠的股票数据提供商
	unreliableProvider := NewUnreliableStockProvider(5) // 5次失败后恢复

	// 创建熔断器配置
	config := &decorators.CircuitBreakerConfig{
		Name:        "StockProviderCircuitBreaker",
		MaxRequests: 3,                // 半开状态允许3个请求
		Interval:    10 * time.Second, // 10秒统计窗口
		Timeout:     5 * time.Second,  // 熔断5秒
		ReadyToTrip: 3,                // 3次失败触发熔断
		Enabled:     true,             // 启用熔断器
	}

	// 使用熔断器装饰器包装提供商
	decoratedProvider := decorators.NewCircuitBreakerProvider(unreliableProvider, config)

	fmt.Printf("装饰器名称: %s\n", decoratedProvider.Name())
	fmt.Printf("健康状态: %v\n", decoratedProvider.IsHealthy())
	fmt.Printf("熔断器状态: %s\n", decoratedProvider.GetState())
	fmt.Println()

	symbols := []string{"000001.SZ", "000002.SZ"}
	ctx := context.Background()

	fmt.Println("=== 测试熔断器效果 ===")
	
	// 连续发起多次请求，触发熔断
	for i := 1; i <= 10; i++ {
		fmt.Printf("第%d次请求...\n", i)
		
		data, err := decoratedProvider.FetchStockData(ctx, symbols)
		
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
		} else {
			fmt.Printf("成功获取%d条数据\n", len(data))
			for _, stock := range data {
				fmt.Printf("  %s: ¥%.2f\n", stock.Symbol, stock.Price)
			}
		}

		// 显示当前熔断器状态
		state := decoratedProvider.GetState()
		counts := decoratedProvider.GetCounts()
		fmt.Printf("熔断器状态: %s, 连续失败: %d, 总请求: %d\n", 
			state, counts.ConsecutiveFailures, counts.Requests)
		
		fmt.Println()
		time.Sleep(500 * time.Millisecond) // 稍作等待
	}

	fmt.Println("=== 等待熔断器恢复 ===")
	fmt.Println("等待5秒让熔断器从打开状态恢复...")
	time.Sleep(6 * time.Second)
	
	// 现在提供商应该开始恢复正常
	unreliableProvider.SetFailureMode(false)
	fmt.Println("设置提供商为正常模式")
	
	// 测试恢复后的请求
	for i := 1; i <= 5; i++ {
		fmt.Printf("恢复后第%d次请求...\n", i)
		
		data, err := decoratedProvider.FetchStockData(ctx, symbols)
		
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
		} else {
			fmt.Printf("成功获取%d条数据\n", len(data))
		}

		state := decoratedProvider.GetState()
		counts := decoratedProvider.GetCounts()
		fmt.Printf("熔断器状态: %s, 连续成功: %d, 连续失败: %d\n", 
			state, counts.ConsecutiveSuccesses, counts.ConsecutiveFailures)
		
		fmt.Println()
		time.Sleep(200 * time.Millisecond)
	}

	// 显示完整的熔断器状态信息
	fmt.Println("=== 熔断器详细状态信息 ===")
	status := decoratedProvider.GetStatus()
	for key, value := range status {
		fmt.Printf("%s: %v\n", key, value)
	}

	fmt.Println("\n=== 测试熔断器配置调整 ===")
	
	// 禁用熔断器
	decoratedProvider.SetEnabled(false)
	fmt.Println("禁用熔断器...")
	
	// 重新设置提供商为失败模式
	unreliableProvider.SetFailureMode(true)
	
	// 测试禁用熔断器后的行为
	for i := 1; i <= 3; i++ {
		fmt.Printf("禁用熔断器后第%d次请求...\n", i)
		
		data, err := decoratedProvider.FetchStockData(ctx, symbols)
		
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
		} else {
			fmt.Printf("成功获取%d条数据\n", len(data))
		}
		
		fmt.Printf("健康状态: %v (禁用熔断器时应该直接反映基础提供商状态)\n", decoratedProvider.IsHealthy())
		fmt.Println()
	}

	fmt.Println("\n=== 测试不同的熔断器状态 ===")
	
	// 重新启用熔断器
	decoratedProvider.SetEnabled(true)
	fmt.Println("重新启用熔断器")
	
	// 创建另一个更敏感的熔断器配置
	sensitiveConfig := &decorators.CircuitBreakerConfig{
		Name:        "SensitiveCircuitBreaker",
		MaxRequests: 2,                // 半开状态只允许2个请求
		Interval:    5 * time.Second,  // 5秒统计窗口
		Timeout:     3 * time.Second,  // 熔断3秒
		ReadyToTrip: 2,                // 2次失败就触发熔断
		Enabled:     true,
	}
	
	// 创建另一个不可靠提供商
	anotherUnreliableProvider := NewUnreliableStockProvider(10)
	sensitiveCB := decorators.NewCircuitBreakerProvider(anotherUnreliableProvider, sensitiveConfig)
	
	fmt.Printf("敏感熔断器名称: %s\n", sensitiveCB.Name())
	
	// 快速触发敏感熔断器
	for i := 1; i <= 4; i++ {
		fmt.Printf("敏感熔断器第%d次请求...\n", i)
		
		data, err := sensitiveCB.FetchStockData(ctx, symbols)
		
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
		} else {
			fmt.Printf("成功获取%d条数据\n", len(data))
		}
		
		fmt.Printf("敏感熔断器状态: %s\n", sensitiveCB.GetState())
		fmt.Println()
		
		if sensitiveCB.IsOpen() {
			fmt.Println("敏感熔断器已打开，停止进一步测试")
			break
		}
	}

	fmt.Println("熔断器装饰器示例完成！")
}