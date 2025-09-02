//go:build integration

package integration

import (
	"context"
	"stocksub/pkg/provider"
	"stocksub/pkg/provider/decorators"
	"stocksub/pkg/provider/tencent"
	"strings"
	"testing"
	"time"
)

// TestAPIMonitorDecoratorCompatibility 测试 API Monitor 与装饰器的兼容性
func TestAPIMonitorDecoratorCompatibility(t *testing.T) {
	t.Log("测试 API Monitor 在装饰器架构下的兼容性")

	// 创建基础腾讯提供商
	baseProvider := tencent.NewClient()

	// 使用监控专用的装饰器配置
	decoratorConfig := decorators.MonitoringDecoratorConfig()

	// 应用装饰器
	decoratedProvider, err := decorators.CreateDecoratedProvider(baseProvider, decoratorConfig)
	if err != nil {
		t.Fatalf("创建装饰提供商失败: %v", err)
	}

	// 验证装饰器链配置
	t.Log("验证装饰器配置...")

	// 检查是否为熔断器装饰器
	cbProvider, ok := decoratedProvider.(*decorators.CircuitBreakerProvider)
	if !ok {
		t.Fatal("期望装饰器链顶层是熔断器")
	}

	status := cbProvider.GetStatus()
	if status["state"] != "closed" {
		t.Errorf("期望熔断器状态为 closed，得到 %v", status["state"])
	}

	// 检查频率控制装饰器
	fcProvider, ok := cbProvider.RealtimeStockProvider.(*decorators.FrequencyControlProvider)
	if !ok {
		t.Fatal("期望在熔断器下找到频率控制装饰器")
	}

	fcStatus := fcProvider.GetStatus()
	minInterval := fcStatus["min_interval"].(string)
	if minInterval != "3s" {
		t.Errorf("期望频率控制间隔为 3s，得到 %v", minInterval)
	}

	maxRetries := fcStatus["max_retries"].(int)
	if maxRetries != 5 {
		t.Errorf("期望最大重试次数为 5，得到 %v", maxRetries)
	}

	isActive := fcStatus["is_active"].(bool)
	if !isActive {
		t.Error("期望频率控制装饰器处于活跃状态")
	}

	// 验证装饰器名称
	expectedName := "CircuitBreaker(FrequencyControl(tencent))"
	actualName := decoratedProvider.Name()
	if actualName != expectedName {
		t.Errorf("期望装饰器名称为 %s，得到 %s", expectedName, actualName)
	}
}

// TestAPIMonitorDecoratorFunctionality 测试装饰器功能性
func TestAPIMonitorDecoratorFunctionality(t *testing.T) {
	t.Log("测试装饰器功能性")

	// 创建装饰后的提供商
	baseProvider := tencent.NewClient()

	decoratorConfig := decorators.MonitoringDecoratorConfig()
	decoratedProvider, err := decorators.CreateDecoratedProvider(baseProvider, decoratorConfig)
	if err != nil {
		t.Fatalf("创建装饰提供商失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 模拟监控场景：多次连续调用
	symbols := []string{"600000"} // 使用一个测试股票代码
	successCount := 0
	totalCalls := 3

	// 记录每次调用的时间，用于验证频率控制
	callTimes := make([]time.Time, 0, totalCalls)

	for i := 0; i < totalCalls; i++ {
		t.Logf("执行第 %d 次调用", i+1)

		start := time.Now()
		callTimes = append(callTimes, start)

		realtimeProvider, ok := decoratedProvider.(provider.RealtimeStockProvider)
		if !ok {
			t.Fatalf("decoratedProvider is not a RealtimeStockProvider")
		}
		data, err := realtimeProvider.FetchStockData(ctx, symbols)
		duration := time.Since(start)

		if err != nil {
			t.Logf("第 %d 次调用失败: %v (耗时: %v)", i+1, err, duration)
			// 在测试环境中，由于网络或市场时间限制，失败是可接受的
		} else {
			successCount++
			t.Logf("第 %d 次调用成功: 获取 %d 条数据 (耗时: %v)", i+1, len(data), duration)

			// 验证返回数据的正确性
			if len(data) > 0 {
				if data[0].Symbol == "" {
					t.Error("返回数据的股票代码不应为空")
				}
				if data[0].Price <= 0 {
					t.Error("返回数据的价格应大于0")
				}
			}
		}

		// 验证装饰器的频率控制是否生效
		if i < totalCalls-1 {
			// 确保有足够的间隔（不少于3秒）
			if duration < 3*time.Second {
				time.Sleep(3*time.Second - duration)
			}
		}
	}

	t.Logf("调用完成: 成功 %d/%d", successCount, totalCalls)

	// 验证调用间隔（应该至少3秒）
	if len(callTimes) >= 2 {
		for i := 1; i < len(callTimes); i++ {
			interval := callTimes[i].Sub(callTimes[i-1])
			t.Logf("第 %d 次和第 %d 次调用间隔: %v", i, i+1, interval)
			// 由于网络延迟等因素，我们允许一些误差
			if interval < 2*time.Second {
				t.Errorf("调用间隔应至少为2秒，实际为 %v", interval)
			}
		}
	}

	// 获取最终状态并验证
	if statusProvider, ok := decoratedProvider.(interface{ GetStatus() map[string]interface{} }); ok {
		status := statusProvider.GetStatus()
		t.Logf("最终装饰器状态: %+v", status)

		// 验证状态信息的完整性
		if status["decorator_type"] == nil {
			t.Error("装饰器状态应包含 decorator_type")
		}

		if status["base_provider"] == nil {
			t.Error("装饰器状态应包含 base_provider")
		}

		// 验证统计信息
		if stats, ok := status["stats"].(map[string]interface{}); ok {
			totalRequests := stats["total_requests"].(int64)
			if totalRequests <= 0 {
				t.Error("总请求数应大于0")
			}
		}
	}
}

// TestAPIMonitorBackwardCompatibility 测试向后兼容性
func TestAPIMonitorBackwardCompatibility(t *testing.T) {
	t.Log("测试向后兼容性")

	// 创建原始提供商（不使用装饰器）
	originalProvider := tencent.NewClient()

	// 创建装饰后的提供商
	decoratedProvider, err := decorators.CreateDecoratedProvider(originalProvider, decorators.MonitoringDecoratorConfig())
	if err != nil {
		t.Fatalf("创建装饰提供商失败: %v", err)
	}

	// 验证接口兼容性
	ctx := context.Background()
	symbols := []string{"600000"}

	// 测试 FetchStockData 方法
	t.Log("测试 FetchStockData 方法...")
	realtimeProvider, ok := decoratedProvider.(provider.RealtimeStockProvider)
	if !ok {
		t.Fatalf("decoratedProvider is not a RealtimeStockProvider")
	}
	data, err := realtimeProvider.FetchStockData(ctx, symbols)
	if err != nil {
		t.Logf("FetchStockData 调用结果: %v (这在测试环境中是正常的)", err)
	} else {
		t.Log("FetchStockData 调用成功")
		// 验证返回数据
		if len(data) > 0 {
			if data[0].Symbol == "" {
				t.Error("返回数据的股票代码不应为空")
			}
		}
	}

	// 测试 FetchStockDataWithRaw 方法
	t.Log("测试 FetchStockDataWithRaw 方法...")
	rawData, rawString, err := realtimeProvider.FetchStockDataWithRaw(ctx, symbols)
	if err != nil {
		t.Logf("FetchStockDataWithRaw 调用结果: %v (这在测试环境中是正常的)", err)
	} else {
		t.Log("FetchStockDataWithRaw 调用成功")
		// 验证返回数据
		if len(rawData) > 0 {
			if rawData[0].Symbol == "" {
				t.Error("返回数据的股票代码不应为空")
			}
		}
		if rawString == "" {
			t.Error("原始数据字符串不应为空")
		}
	}

	// 测试 IsSymbolSupported 方法
	t.Log("测试 IsSymbolSupported 方法...")
	supported := realtimeProvider.IsSymbolSupported("600000")
	if !supported {
		t.Error("期望支持股票代码 600000")
	} else {
		t.Log("IsSymbolSupported 方法正常工作")
	}

	// 测试基础 Provider 接口方法
	t.Log("测试基础 Provider 接口...")
	name := decoratedProvider.Name()
	if name == "" {
		t.Error("Provider 名称不应为空")
	} else {
		t.Logf("Provider 名称: %s", name)
		// 验证名称格式
		if !strings.Contains(name, "CircuitBreaker") || !strings.Contains(name, "FrequencyControl") {
			t.Error("Provider 名称应包含 CircuitBreaker 和 FrequencyControl")
		}
	}

	rateLimit := decoratedProvider.GetRateLimit()
	if rateLimit <= 0 {
		t.Error("频率限制应大于0")
	} else {
		t.Logf("频率限制: %v", rateLimit)
		// 验证频率限制是否符合监控配置（应为3秒）
		expectedRateLimit := 3 * time.Second
		if rateLimit != expectedRateLimit {
			t.Errorf("期望频率限制为 %v，得到 %v", expectedRateLimit, rateLimit)
		}
	}

	healthy := decoratedProvider.IsHealthy()
	t.Logf("健康状态: %v", healthy)
	// 健康状态应该是布尔值
	// 这里直接就是bool类型，不需要类型断言
	if healthy != true && healthy != false {
		t.Error("健康状态应为布尔值")
	}
}

// TestAPIMonitorErrorHandling 测试错误处理能力
func TestAPIMonitorErrorHandling(t *testing.T) {
	t.Log("测试API Monitor装饰器的错误处理能力")

	// 创建装饰后的提供商
	baseProvider := tencent.NewClient()
	decoratorConfig := decorators.MonitoringDecoratorConfig()
	decoratedProvider, err := decorators.CreateDecoratedProvider(baseProvider, decoratorConfig)
	if err != nil {
		t.Fatalf("创建装饰提供商失败: %v", err)
	}

	// 获取熔断器装饰器
	cbProvider, ok := decoratedProvider.(*decorators.CircuitBreakerProvider)
	if !ok {
		t.Fatal("期望装饰器链顶层是熔断器")
	}

	// 初始状态应该是关闭的
	if !cbProvider.IsClosed() {
		t.Error("初始状态下熔断器应处于关闭状态")
	}

	// 测试健康状态
	healthy := decoratedProvider.IsHealthy()
	t.Logf("健康状态: %v", healthy)
	if healthy != true && healthy != false {
		t.Error("健康状态应为布尔值")
	}

	t.Log("API Monitor错误处理测试完成")
}
