package decorators

import (
	"context"
	"errors"
	"stocksub/pkg/testkit/providers"
	"stocksub/pkg/timing"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSimplifiedCircuitBreaker_BasicFunctionality 测试基本熔断功能
func TestSimplifiedCircuitBreaker_BasicFunctionality(t *testing.T) {
	mockProvider := NewMockProviderAdapter(providers.DefaultMockProviderConfig())
	config := DefaultSimplifiedCircuitBreakerConfig()
	config.ReadyToTrip = 2
	config.Timeout = 100 * time.Millisecond
	config.MarketTimeAware = false // 禁用交易时段感知以简化测试
	decorator := NewSimplifiedCircuitBreakerProvider(mockProvider, config)

	// 第一次成功
	_, err := decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.NoError(t, err)
	assert.True(t, decorator.IsClosed())

	// 模拟连续失败
	mockProvider.SetFetchDataError(errors.New("network error"))
	_, err = decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.Error(t, err)
	_, err = decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.Error(t, err)

	// 此时应该熔断
	assert.True(t, decorator.IsOpen())

	// 熔断后请求应该立即失败
	_, err = decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")

	// 等待超时后恢复
	time.Sleep(150 * time.Millisecond)

	// 恢复正常
	mockProvider.SetFetchDataError(nil)

	// 进行足够多的成功请求以完全关闭熔断器
	// gobreaker在半开状态需要MaxRequests次成功请求才能关闭
	for i := 0; i < int(config.MaxRequests); i++ {
		_, err = decorator.FetchStockData(context.Background(), []string{"sh600000"})
		assert.NoError(t, err, "第%d次恢复请求应该成功", i+1)
	}

	// 现在应该完全关闭
	assert.True(t, decorator.IsClosed(), "应该完全关闭熔断器")
}

// TestSimplifiedCircuitBreaker_MarketTimeAwareness 测试交易时段感知
func TestSimplifiedCircuitBreaker_MarketTimeAwareness(t *testing.T) {
	mockProvider := NewMockProviderAdapter(providers.DefaultMockProviderConfig())
	config := DefaultSimplifiedCircuitBreakerConfig()
	config.MarketTimeAware = true
	decorator := NewSimplifiedCircuitBreakerProvider(mockProvider, config)

	// 模拟非交易时间
	mockTime := &MockTimeService{}
	nonTradingTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC) // Sunday
	mockTime.SetCurrentTime(nonTradingTime)
	decorator.marketTime = timing.NewMarketTime(mockTime)

	_, err := decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "当前不在交易时段内")
	assert.False(t, decorator.IsHealthy())

	// 模拟交易时间（周一上午10点，在交易时段内）
	tradingTime := time.Date(2023, 1, 2, 10, 0, 0, 0, time.UTC) // Monday 10:00
	mockTime.SetCurrentTime(tradingTime)
	decorator.marketTime = timing.NewMarketTime(mockTime)

	_, err = decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.NoError(t, err)
	assert.True(t, decorator.IsHealthy())
}

// TestSimplifiedCircuitBreaker_IPBanStats 测试IP封禁统计
func TestSimplifiedCircuitBreaker_IPBanStats(t *testing.T) {
	mockProvider := NewMockProviderAdapter(providers.DefaultMockProviderConfig())
	config := DefaultSimplifiedCircuitBreakerConfig()
	decorator := NewSimplifiedCircuitBreakerProvider(mockProvider, config)

	// 模拟IP封禁错误
	mockProvider.SetFetchDataError(errors.New("IP banned"))
	_, err := decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.Error(t, err)

	status := decorator.GetStatus()["stats"].(map[string]interface{})
	assert.Equal(t, int64(1), status["ip_ban_count"])

	// 模拟其他错误
	mockProvider.SetFetchDataError(errors.New("another error"))
	_, err = decorator.FetchStockData(context.Background(), []string{"sh600000"})
	assert.Error(t, err)

	status = decorator.GetStatus()["stats"].(map[string]interface{})
	assert.Equal(t, int64(1), status["ip_ban_count"]) // IP封禁计数不应该增加
}
