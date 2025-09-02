//go:build integration

package integration

import (
	"context"
	"stocksub/pkg/provider"
	"stocksub/pkg/provider/decorators"
	"stocksub/pkg/provider/tencent"
	"testing"
	"time"
)

// TestSimplifiedDecoratorCompatibilityWithExisting 测试简化装饰器与现有装饰器的兼容性
func TestSimplifiedDecoratorCompatibilityWithExisting(t *testing.T) {
	t.Log("测试简化装饰器与现有装饰器的兼容性")

	// 创建基础腾讯提供商
	baseProvider := tencent.NewClient()

	// 使用默认装饰器配置（现有装饰器）
	defaultDecoratorConfig := decorators.DefaultDecoratorConfig()

	// 应用现有装饰器
	existingDecoratedProvider, err := decorators.CreateDecoratedProvider(baseProvider, defaultDecoratorConfig)
	if err != nil {
		t.Fatalf("创建现有装饰提供商失败: %v", err)
	}

	// 使用简化装饰器配置
	simplifiedDecoratorConfig := decorators.DefaultSimplifiedConfig()

	// 应用简化装饰器
	simplifiedDecoratedProvider, err := decorators.CreateSimplifiedDecoratedProvider(baseProvider, simplifiedDecoratorConfig)
	if err != nil {
		t.Fatalf("创建简化装饰提供商失败: %v", err)
	}

	// 验证两种装饰器链配置
	t.Log("验证现有装饰器配置...")
	validateProviderChain(t, existingDecoratedProvider, "现有")

	t.Log("验证简化装饰器配置...")
	validateProviderChain(t, simplifiedDecoratedProvider, "简化")
}

// validateProviderChain 验证装饰器链配置
func validateProviderChain(t *testing.T, provider provider.Provider, providerType string) {
	// 检查是否为熔断器装饰器
	if cbProvider, ok := provider.(*decorators.CircuitBreakerProvider); ok {
		status := cbProvider.GetStatus()
		t.Logf("%s熔断器状态: %v", providerType, status["state"])

		// 检查频率控制装饰器
		if fcProvider, ok := cbProvider.RealtimeStockProvider.(*decorators.FrequencyControlProvider); ok {
			fcStatus := fcProvider.GetStatus()
			t.Logf("%s频率控制状态: 间隔=%v, 活跃=%v", providerType, fcStatus["min_interval"], fcStatus["is_active"])
		} else if sfcProvider, ok := cbProvider.RealtimeStockProvider.(*decorators.SimplifiedFrequencyControlProvider); ok {
			sfcStatus := sfcProvider.GetStatus()
			t.Logf("%s简化频率控制状态: 间隔=%v, 活跃=%v", providerType, sfcStatus["min_interval"], sfcStatus["enabled"])
		} else {
			t.Errorf("期望在%s熔断器下找到频率控制装饰器", providerType)
		}
	} else if scbProvider, ok := provider.(*decorators.SimplifiedCircuitBreakerProvider); ok {
		status := scbProvider.GetStatus()
		t.Logf("%s简化熔断器状态: %v", providerType, status["state"])

		// 检查简化频率控制装饰器
		if sfcProvider, ok := scbProvider.RealtimeStockProvider.(*decorators.SimplifiedFrequencyControlProvider); ok {
			sfcStatus := sfcProvider.GetStatus()
			t.Logf("%s简化频率控制状态: 间隔=%v, 活跃=%v", providerType, sfcStatus["min_interval"], sfcStatus["enabled"])
		} else {
			t.Errorf("期望在%s简化熔断器下找到简化频率控制装饰器", providerType)
		}
	} else {
		t.Errorf("期望%s装饰器链顶层是熔断器", providerType)
	}
}

// TestSimplifiedDecoratorBackwardCompatibility 测试简化装饰器向后兼容性
func TestSimplifiedDecoratorBackwardCompatibility(t *testing.T) {
	t.Log("测试简化装饰器向后兼容性")

	// 创建原始提供商（不使用装饰器）
	originalProvider := tencent.NewClient()

	// 创建简化装饰后的提供商
	simplifiedDecoratorConfig := decorators.DefaultSimplifiedConfig()
	decoratedProvider, err := decorators.CreateSimplifiedDecoratedProvider(originalProvider, simplifiedDecoratorConfig)
	if err != nil {
		t.Fatalf("创建简化装饰提供商失败: %v", err)
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
	_, err = realtimeProvider.FetchStockData(ctx, symbols)
	if err != nil {
		t.Logf("FetchStockData 调用结果: %v (这在测试环境中是正常的)", err)
	} else {
		t.Log("FetchStockData 调用成功")
	}

	// 测试 FetchStockDataWithRaw 方法
	t.Log("测试 FetchStockDataWithRaw 方法...")
	_, _, err = realtimeProvider.FetchStockDataWithRaw(ctx, symbols)
	if err != nil {
		t.Logf("FetchStockDataWithRaw 调用结果: %v (这在测试环境中是正常的)", err)
	} else {
		t.Log("FetchStockDataWithRaw 调用成功")
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

// TestSimplifiedDecoratorCoexistence 测试简化装饰器与现有装饰器共存
func TestSimplifiedDecoratorCoexistence(t *testing.T) {
	t.Log("测试简化装饰器与现有装饰器共存")

	// 创建基础提供商
	baseProvider := tencent.NewClient()

	// 创建现有装饰器链
	existingDecoratorConfig := decorators.DefaultDecoratorConfig()
	existingDecoratedProvider, err := decorators.CreateDecoratedProvider(baseProvider, existingDecoratorConfig)
	if err != nil {
		t.Fatalf("创建现有装饰提供商失败: %v", err)
	}

	// 创建简化装饰器链
	simplifiedDecoratorConfig := decorators.DefaultSimplifiedConfig()
	simplifiedDecoratedProvider, err := decorators.CreateSimplifiedDecoratedProvider(baseProvider, simplifiedDecoratorConfig)
	if err != nil {
		t.Fatalf("创建简化装饰提供商失败: %v", err)
	}

	// 验证两个装饰器链可以独立工作
	ctx := context.Background()
	symbols := []string{"600000"}

	// 测试现有装饰器链
	t.Log("测试现有装饰器链...")
	existingRealtimeProvider, ok := existingDecoratedProvider.(provider.RealtimeStockProvider)
	if !ok {
		t.Fatalf("existingDecoratedProvider is not a RealtimeStockProvider")
	}
	_, err = existingRealtimeProvider.FetchStockData(ctx, symbols)
	if err != nil {
		t.Logf("现有装饰器链调用结果: %v (这在测试环境中是正常的)", err)
	} else {
		t.Log("现有装饰器链调用成功")
	}

	// 测试简化装饰器链
	t.Log("测试简化装饰器链...")
	simplifiedRealtimeProvider, ok := simplifiedDecoratedProvider.(provider.RealtimeStockProvider)
	if !ok {
		t.Fatalf("simplifiedDecoratedProvider is not a RealtimeStockProvider")
	}
	_, err = simplifiedRealtimeProvider.FetchStockData(ctx, symbols)
	if err != nil {
		t.Logf("简化装饰器链调用结果: %v (这在测试环境中是正常的)", err)
	} else {
		t.Log("简化装饰器链调用成功")
	}

	// 验证两种装饰器链的状态
	t.Log("验证两种装饰器链状态...")
	if statusProvider, ok := existingDecoratedProvider.(interface{ GetStatus() map[string]interface{} }); ok {
		status := statusProvider.GetStatus()
		t.Logf("现有装饰器链状态: %v", status["decorator_type"])
	}

	if statusProvider, ok := simplifiedDecoratedProvider.(interface{ GetStatus() map[string]interface{} }); ok {
		status := statusProvider.GetStatus()
		t.Logf("简化装饰器链状态: %v", status["decorator_type"])
	}
}

// TestSimplifiedDecoratorConfigurationCompatibility 测试简化装饰器配置兼容性
func TestSimplifiedDecoratorConfigurationCompatibility(t *testing.T) {
	t.Log("测试简化装饰器配置兼容性")

	baseProvider := tencent.NewClient()

	// 测试不同的简化配置场景
	testCases := []struct {
		name   string
		config *decorators.SimplifiedDecoratorConfig
	}{
		{
			name:   "默认简化配置",
			config: decorators.DefaultSimplifiedConfig(),
		},
		{
			name:   "生产环境简化配置",
			config: decorators.ProductionSimplifiedConfig(),
		},
		{
			name:   "测试环境简化配置",
			config: decorators.TestSimplifiedConfig(),
		},
		{
			name:   "监控环境简化配置",
			config: decorators.MonitoringSimplifiedConfig(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoratedProvider, err := decorators.CreateSimplifiedDecoratedProvider(baseProvider, tc.config)
			if err != nil {
				t.Fatalf("创建简化装饰提供商失败 (%s): %v", tc.name, err)
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

			// 验证配置值
			validateSimplifiedConfigValues(t, tc.config, tc.name)

			t.Logf("配置 %s: 验证通过", tc.name)
		})
	}
}

// validateSimplifiedConfigValues 验证简化配置值
func validateSimplifiedConfigValues(t *testing.T, config *decorators.SimplifiedDecoratorConfig, configName string) {
	if config.FrequencyControl == nil {
		t.Errorf("配置 %s: 频率控制配置不应为空", configName)
		return
	}

	if config.CircuitBreaker == nil {
		t.Errorf("配置 %s: 熔断器配置不应为空", configName)
		return
	}

	// 验证频率控制配置
	if config.FrequencyControl.MinInterval <= 0 {
		t.Errorf("配置 %s: 最小请求间隔应大于0", configName)
	}

	if config.FrequencyControl.MaxRetries < 0 {
		t.Errorf("配置 %s: 最大重试次数不能为负数", configName)
	}

	if config.FrequencyControl.IPBanRetryMax < 0 {
		t.Errorf("配置 %s: IP封禁最大重试次数不能为负数", configName)
	}

	if config.FrequencyControl.IPBanRetryInterval <= 0 {
		t.Errorf("配置 %s: IP封禁重试间隔应大于0", configName)
	}

	// 验证熔断器配置
	if config.CircuitBreaker.MaxRequests == 0 {
		t.Errorf("配置 %s: 半开状态最大请求数不能为0", configName)
	}

	if config.CircuitBreaker.Interval <= 0 {
		t.Errorf("配置 %s: 统计窗口时间应大于0", configName)
	}

	if config.CircuitBreaker.Timeout <= 0 {
		t.Errorf("配置 %s: 熔断器超时时间应大于0", configName)
	}

	if config.CircuitBreaker.ReadyToTrip == 0 {
		t.Errorf("配置 %s: 触发熔断的失败次数不能为0", configName)
	}

	t.Logf("配置 %s: 所有配置值验证通过", configName)
}

// TestSimplifiedDecoratorIntegrationWithFactory 测试简化装饰器与工厂的集成
func TestSimplifiedDecoratorIntegrationWithFactory(t *testing.T) {
	t.Log("测试简化装饰器与工厂的集成")

	// 创建装饰器工厂
	factory := decorators.NewDecoratorFactory()

	// 创建基础提供商
	baseProvider := tencent.NewClient()

	// 测试使用工厂创建简化装饰器
	t.Log("测试使用工厂创建简化频率控制装饰器...")
	freqProvider, err := factory.CreateDecorator(decorators.SimplifiedFrequencyControlType, baseProvider, nil)
	if err != nil {
		t.Fatalf("使用工厂创建简化频率控制装饰器失败: %v", err)
	}

	t.Log("测试使用工厂创建简化熔断器装饰器...")
	cbProvider, err := factory.CreateDecorator(decorators.SimplifiedCircuitBreakerType, baseProvider, nil)
	if err != nil {
		t.Fatalf("使用工厂创建简化熔断器装饰器失败: %v", err)
	}

	// 验证创建的装饰器类型
	if _, ok := freqProvider.(*decorators.SimplifiedFrequencyControlProvider); !ok {
		t.Error("期望创建简化频率控制装饰器")
	}

	if _, ok := cbProvider.(*decorators.SimplifiedCircuitBreakerProvider); !ok {
		t.Error("期望创建简化熔断器装饰器")
	}

	t.Log("工厂创建简化装饰器测试通过")
}

// TestSimplifiedDecoratorErrorHandlingCompatibility 测试简化装饰器错误处理兼容性
func TestSimplifiedDecoratorErrorHandlingCompatibility(t *testing.T) {
	t.Log("测试简化装饰器错误处理兼容性")

	// 创建基础提供商
	baseProvider := tencent.NewClient()

	// 创建简化装饰器配置
	config := decorators.DefaultSimplifiedConfig()
	config.FrequencyControl.Enabled = true
	config.FrequencyControl.MinInterval = 10 * time.Millisecond // 使用很短的间隔以快速触发频率控制
	config.CircuitBreaker.Enabled = true
	config.CircuitBreaker.ReadyToTrip = 1 // 降低阈值以快速触发熔断

	// 创建简化装饰器链
	decoratedProvider, err := decorators.CreateSimplifiedDecoratedProvider(baseProvider, config)
	if err != nil {
		t.Fatalf("创建简化装饰提供商失败: %v", err)
	}

	// 验证错误处理
	ctx := context.Background()
	symbols := []string{"600000"}

	// 进行多次调用以触发错误处理机制
	for i := 0; i < 3; i++ {
		t.Logf("执行第 %d 次调用", i+1)
		_, err := decoratedProvider.(provider.RealtimeStockProvider).FetchStockData(ctx, symbols)
		if err != nil {
			t.Logf("第 %d 次调用失败: %v (这在测试环境中是正常的)", i+1, err)
		} else {
			t.Logf("第 %d 次调用成功", i+1)
		}

		// 等待一小段时间
		time.Sleep(50 * time.Millisecond)
	}

	// 检查装饰器状态
	if statusProvider, ok := decoratedProvider.(interface{ GetStatus() map[string]interface{} }); ok {
		status := statusProvider.GetStatus()
		t.Logf("最终装饰器状态: %+v", status)

		// 检查是否有错误统计信息
		if stats, ok := status["stats"]; ok {
			t.Logf("错误统计信息: %+v", stats)
		}
	}
}
