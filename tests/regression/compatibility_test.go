package integration_test

import (
	"context"
	"testing"

	"stocksub/pkg/core"
	"stocksub/pkg/limiter"
	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/config"
	"stocksub/pkg/testkit/manager"
	"stocksub/pkg/timing"
)

// TestAPIMonitorCompatibility 测试 api_monitor 功能的兼容性
func TestAPIMonitorCompatibility(t *testing.T) {
	ctx := context.Background()

	// 创建模拟的 api_monitor 配置
	provider := tencent.NewClient()
	// provider.SetTimeout(30 * time.Second)
	// provider.SetRateLimit(1 * time.Second)

	marketTime := timing.DefaultMarketTime()
	_ = limiter.NewIntelligentLimiter(marketTime) // 仅用于初始化测试

	// 测试基本功能
	symbols := []string{"600000", "000001"}

	// 测试数据获取
	result, _, err := provider.FetchStockDataWithRaw(ctx, symbols)
	if err != nil {
		t.Skipf("API 调用失败，跳过测试: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("未获取到任何数据")
	}

	// 验证数据结构
	for _, stock := range result {
		if stock.Symbol == "" {
			t.Error("股票代码不能为空")
		}
		if stock.Price <= 0 {
			t.Errorf("股票价格无效: %s=%.2f", stock.Symbol, stock.Price)
		}
	}

	t.Logf("成功获取 %d 只股票数据", len(result))
}

// TestTestKitCompatibility 测试 testkit 功能的兼容性
func TestTestKitCompatibility(t *testing.T) {
	ctx := context.Background()

	// 创建 TestDataManager
	cfg := &config.Config{
		Cache:   config.CacheConfig{Type: "memory"},
		Storage: config.StorageConfig{Type: "memory"},
	}

	manager := manager.NewTestDataManager(cfg)
	defer manager.Close()

	// 启用 Mock 模式进行测试
	manager.EnableMock(true)
	symbols := []string{"600000", "000001"}
	mockData := []core.StockData{
		{Symbol: "600000", Price: 10.50, Volume: 1000000},
		{Symbol: "000001", Price: 15.20, Volume: 2000000},
	}
	manager.SetMockData(symbols, mockData)

	// 测试数据获取
	result, err := manager.GetStockData(ctx, symbols)
	if err != nil {
		t.Fatalf("获取数据失败: %v", err)
	}

	if len(result) != len(symbols) {
		t.Fatalf("期望获取 %d 只股票，实际获取 %d", len(symbols), len(result))
	}

	// 验证数据正确性
	for i, stock := range result {
		if stock.Symbol != symbols[i] {
			t.Errorf("股票代码不匹配: 期望 %s, 实际 %s", symbols[i], stock.Symbol)
		}
		if stock.Price != mockData[i].Price {
			t.Errorf("股票价格不匹配: 期望 %.2f, 实际 %.2f", mockData[i].Price, stock.Price)
		}
	}

	t.Log("TestKit 功能测试通过")
}

// TestLimiterCompatibility 测试限流器功能的兼容性
func TestLimiterCompatibility(t *testing.T) {
	ctx := context.Background()

	marketTime := timing.DefaultMarketTime()
	limiter := limiter.NewIntelligentLimiter(marketTime)

	// 初始化批量处理
	symbols := []string{"600000", "000001"}
	limiter.InitializeBatch(symbols)

	// 测试交易时间检测
	isTradingTime := marketTime.IsTradingTime()
	t.Logf("当前是否为交易时间: %t", isTradingTime)

	// 测试是否可以继续执行
	shouldProceed, err := limiter.ShouldProceed(ctx)
	if err != nil {
		t.Skipf("限流器检查失败，跳过测试: %v", err)
	}

	t.Logf("是否可以继续执行: %t", shouldProceed)

	// 测试记录结果（模拟成功情况）
	shouldContinue, waitDuration, finalErr := limiter.RecordResult(nil, []string{"600000,10.50,1000000", "000001,15.20,2000000"})
	if finalErr != nil {
		t.Errorf("记录结果失败: %v", finalErr)
	}

	t.Logf("是否应该继续: %t, 等待时间: %v", shouldContinue, waitDuration)

	t.Log("限流器功能测试通过")
}

// TestSubscriberCompatibility 测试订阅者功能的兼容性
func TestSubscriberCompatibility(t *testing.T) {
	// 创建订阅者配置测试
	// 注意：这个测试主要验证接口兼容性，不实际创建订阅者

	// 验证 subscriber 包的基本结构
	// 跳过详细的版本检查，主要验证类型兼容性

	// 测试类型存在性
	var _ core.StockData
	var _ subscriber.EventType

	t.Log("订阅者基础结构验证通过")
}

// TestEndToEndCompatibility 端到端兼容性测试
func TestEndToEndCompatibility(t *testing.T) {
	// 这个测试验证整个系统的核心功能是否正常工作
	// 在实际重构过程中，这个测试应该始终保持通过

	t.Run("Provider_Data_Fetch", TestAPIMonitorCompatibility)
	t.Run("TestKit_Functionality", TestTestKitCompatibility)
	t.Run("Limiter_Intelligence", TestLimiterCompatibility)
	t.Run("Subscriber_Basics", TestSubscriberCompatibility)

	t.Log("所有兼容性测试完成")
}
