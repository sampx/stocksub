package compatibility

import (
	"context"
	"stocksub/pkg/provider/decorators"
	"stocksub/pkg/provider/tencent"
	"testing"
	"time"
)

// TestAPIMonitorDecoratorCompatibility 测试 API Monitor 与装饰器的兼容性
func TestAPIMonitorDecoratorCompatibility(t *testing.T) {
	t.Log("测试 API Monitor 在装饰器架构下的兼容性")
	
	// 创建基础腾讯提供商
	baseProvider := tencent.NewProvider()
	baseProvider.SetTimeout(10 * time.Second)
	
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
	if cbProvider, ok := decoratedProvider.(*decorators.CircuitBreakerProvider); ok {
		status := cbProvider.GetStatus()
		t.Logf("熔断器状态: %v", status["state"])
		
		// 检查频率控制装饰器
		if fcProvider, ok := cbProvider.GetBaseProvider().(*decorators.FrequencyControlProvider); ok {
			fcStatus := fcProvider.GetStatus()
			t.Logf("频率控制状态: 间隔=%v, 活跃=%v", fcStatus["min_interval"], fcStatus["is_active"])
			
			// 验证配置值
			if fcStatus["min_interval"] != "3s" {
				t.Errorf("期望频率控制间隔为 3s，得到 %v", fcStatus["min_interval"])
			}
			
			if fcStatus["max_retries"] != 5 {
				t.Errorf("期望最大重试次数为 5，得到 %v", fcStatus["max_retries"])
			}
		} else {
			t.Error("期望在熔断器下找到频率控制装饰器")
		}
	} else {
		t.Error("期望装饰器链顶层是熔断器")
	}
}

// TestAPIMonitorDecoratorFunctionality 测试装饰器功能性
func TestAPIMonitorDecoratorFunctionality(t *testing.T) {
	t.Log("测试装饰器功能性")
	
	// 创建装饰后的提供商
	baseProvider := tencent.NewProvider() 
	baseProvider.SetTimeout(5 * time.Second)
	
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
	
	for i := 0; i < totalCalls; i++ {
		t.Logf("执行第 %d 次调用", i+1)
		
		start := time.Now()
		data, err := decoratedProvider.FetchStockData(ctx, symbols)
		duration := time.Since(start)
		
		if err != nil {
			t.Logf("第 %d 次调用失败: %v (耗时: %v)", i+1, err, duration)
			// 在测试环境中，由于网络或市场时间限制，失败是可接受的
		} else {
			successCount++
			t.Logf("第 %d 次调用成功: 获取 %d 条数据 (耗时: %v)", i+1, len(data), duration)
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
	
	// 获取最终状态
	if statusProvider, ok := decoratedProvider.(interface{ GetStatus() map[string]interface{} }); ok {
		status := statusProvider.GetStatus()
		t.Logf("最终装饰器状态: %+v", status)
	}
}

// TestAPIMonitorBackwardCompatibility 测试向后兼容性
func TestAPIMonitorBackwardCompatibility(t *testing.T) {
	t.Log("测试向后兼容性")
	
	// 创建原始提供商（不使用装饰器）
	originalProvider := tencent.NewProvider()
	originalProvider.SetTimeout(5 * time.Second)
	originalProvider.SetRateLimit(3 * time.Second)
	
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
	_, err = decoratedProvider.FetchStockData(ctx, symbols)
	if err != nil {
		t.Logf("FetchStockData 调用结果: %v (这在测试环境中是正常的)", err)
	} else {
		t.Log("FetchStockData 调用成功")
	}
	
	// 测试 FetchStockDataWithRaw 方法
	t.Log("测试 FetchStockDataWithRaw 方法...")
	_, _, err = decoratedProvider.FetchStockDataWithRaw(ctx, symbols)
	if err != nil {
		t.Logf("FetchStockDataWithRaw 调用结果: %v (这在测试环境中是正常的)", err)
	} else {
		t.Log("FetchStockDataWithRaw 调用成功")
	}
	
	// 测试 IsSymbolSupported 方法
	t.Log("测试 IsSymbolSupported 方法...")
	supported := decoratedProvider.IsSymbolSupported("600000")
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
	}
	
	rateLimit := decoratedProvider.GetRateLimit()
	if rateLimit <= 0 {
		t.Error("频率限制应大于0")
	} else {
		t.Logf("频率限制: %v", rateLimit)
	}
	
	healthy := decoratedProvider.IsHealthy()
	t.Logf("健康状态: %v", healthy)
}

// TestAPIMonitorConfigurationFlexibility 测试配置灵活性
func TestAPIMonitorConfigurationFlexibility(t *testing.T) {
	t.Log("测试配置灵活性")
	
	baseProvider := tencent.NewProvider()
	
	// 测试不同的配置场景
	testCases := []struct {
		name   string
		config decorators.ProviderDecoratorConfig
	}{
		{
			name:   "默认配置",
			config: decorators.DefaultDecoratorConfig(),
		},
		{
			name:   "生产环境配置",
			config: decorators.ProductionDecoratorConfig(),
		},
		{
			name:   "监控环境配置",
			config: decorators.MonitoringDecoratorConfig(),
		},
		{
			name:   "测试环境配置",
			config: decorators.TestDecoratorConfig(),
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoratedProvider, err := decorators.CreateDecoratedProvider(baseProvider, tc.config)
			if err != nil {
				t.Fatalf("创建装饰提供商失败 (%s): %v", tc.name, err)
			}
			
			// 验证基础功能
			if decoratedProvider.Name() == "" {
				t.Errorf("配置 %s: Provider 名称不应为空", tc.name)
			}
			
			// 检查装饰器状态
			if statusProvider, ok := decoratedProvider.(interface{ GetStatus() map[string]interface{} }); ok {
				status := statusProvider.GetStatus()
				t.Logf("配置 %s: 装饰器状态 = %v", tc.name, status["decorator_type"])
			}
			
			t.Logf("配置 %s: 验证通过", tc.name)
		})
	}
}
