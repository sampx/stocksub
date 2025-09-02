package decorators

import (
	"context"
	"errors"
	"stocksub/pkg/limiter"
	"stocksub/pkg/testkit/providers"
	"stocksub/pkg/timing"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSimplifiedFrequencyControl_BasicFunctionality 测试基本功能
func TestSimplifiedFrequencyControl_BasicFunctionality(t *testing.T) {
	mockProvider := NewMockProviderAdapter(providers.DefaultMockProviderConfig())
	config := DefaultSimplifiedFrequencyControlConfig()
	config.MinInterval = 50 * time.Millisecond
	config.MarketTimeAware = false // 禁用交易时段感知以简化测试
	decorator := NewSimplifiedFrequencyControlProvider(mockProvider, config)

	start := time.Now()
	_, err := decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.NoError(t, err)
	_, err = decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.NoError(t, err)
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, config.MinInterval)
}

// TestSimplifiedFrequencyControl_MarketTimeAwareness 测试交易时段感知
func TestSimplifiedFrequencyControl_MarketTimeAwareness(t *testing.T) {
	mockProvider := NewMockProviderAdapter(providers.DefaultMockProviderConfig())
	config := DefaultSimplifiedFrequencyControlConfig()
	config.MarketTimeAware = true

	// 创建共享的MockTimeService
	mockTime := &MockTimeService{}

	// 先设置为非交易时间
	nonTradingTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.Local) // Sunday
	mockTime.SetCurrentTime(nonTradingTime)

	// 使用相同的时间服务创建decorator
	marketTime := timing.NewMarketTime(mockTime)
	decorator := NewSimplifiedFrequencyControlProvider(mockProvider, config)
	decorator.marketTime = marketTime
	decorator.limiter = limiter.NewIntelligentLimiter(marketTime)

	_, err := decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "简化频率控制阻止执行")

	// 模拟交易时间（周一上午10点，明确在09:13:30-11:30:10时段内）
	tradingTime := time.Date(2023, 1, 2, 10, 0, 0, 0, time.Local) // Monday 10:00 AM
	mockTime.SetCurrentTime(tradingTime)

	_, err = decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.NoError(t, err)
}

// TestSimplifiedFrequencyControl_IPBanHandling 测试IP封禁处理
func TestSimplifiedFrequencyControl_IPBanHandling(t *testing.T) {
	mockProvider := NewMockProviderAdapter(providers.DefaultMockProviderConfig())

	config := DefaultSimplifiedFrequencyControlConfig()
	config.IPBanRetryInterval = 100 * time.Millisecond
	config.MarketTimeAware = false // 禁用交易时段感知以简化测试
	decorator := NewSimplifiedFrequencyControlProvider(mockProvider, config)

	// 模拟返回IP封禁错误
	mockProvider.SetFetchDataError(errors.New("403 forbidden: IP banned"))

	// 第一次调用，触发IP封禁
	_, err := decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.Error(t, err)

	status := decorator.GetStatus()["ip_ban_status"].(map[string]interface{})
	assert.True(t, status["is_banned"].(bool))

	// 封禁期间调用，应该失败
	_, err = decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "简化频率控制阻止执行")

	// 等待超过重试间隔
	time.Sleep(150 * time.Millisecond)

	// 恢复正常返回
	mockProvider.SetFetchDataError(nil)

	// 再次调用，应该成功
	_, err = decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.NoError(t, err)

	status = decorator.GetStatus()["ip_ban_status"].(map[string]interface{})
	assert.False(t, status["is_banned"].(bool))
}

// TestSimplifiedFrequencyControl_Disabled 测试禁用装饰器
func TestSimplifiedFrequencyControl_Disabled(t *testing.T) {
	// 创建无延迟的MockProvider配置
	mockConfig := providers.DefaultMockProviderConfig()
	mockConfig.DefaultDelay = 0
	mockConfig.RandomDelay = false
	mockProvider := NewMockProviderAdapter(mockConfig)

	config := DefaultSimplifiedFrequencyControlConfig()
	config.Enabled = false
	config.MinInterval = 100 * time.Millisecond
	decorator := NewSimplifiedFrequencyControlProvider(mockProvider, config)

	start := time.Now()
	_, err := decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.NoError(t, err)
	_, err = decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.NoError(t, err)
	elapsed := time.Since(start)

	// 禁用装饰器时，应该绕过频率控制
	assert.Less(t, elapsed, config.MinInterval)
}
