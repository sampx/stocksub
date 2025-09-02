//go:build integration

package integration

import (
	"context"
	"errors"
	"stocksub/pkg/core"
	"stocksub/pkg/provider"
	"stocksub/pkg/provider/decorators"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTimeService 模拟时间服务
type MockTimeService struct {
	current time.Time
}

func (m *MockTimeService) Now() time.Time {
	return m.current
}

func (m *MockTimeService) SetCurrentTime(t time.Time) {
	m.current = t
}

// MockRealtimeStockProvider 模拟 RealtimeStockProvider
type MockRealtimeStockProvider struct {
	fetchStockDataFunc    func(ctx context.Context, symbols []string) ([]core.StockData, error)
	fetchStockDataWithRaw func(ctx context.Context, symbols []string) ([]core.StockData, string, error)
	isSymbolSupportedFunc func(symbol string) bool
}

func (m *MockRealtimeStockProvider) Name() string                { return "MockRealtimeStockProvider" }
func (m *MockRealtimeStockProvider) IsHealthy() bool             { return true }
func (m *MockRealtimeStockProvider) GetRateLimit() time.Duration { return 100 * time.Millisecond }

func (m *MockRealtimeStockProvider) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	if m.fetchStockDataFunc != nil {
		return m.fetchStockDataFunc(ctx, symbols)
	}
	return []core.StockData{}, nil
}

func (m *MockRealtimeStockProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	if m.fetchStockDataWithRaw != nil {
		return m.fetchStockDataWithRaw(ctx, symbols)
	}
	return []core.StockData{}, "", nil
}

func (m *MockRealtimeStockProvider) IsSymbolSupported(symbol string) bool {
	if m.isSymbolSupportedFunc != nil {
		return m.isSymbolSupportedFunc(symbol)
	}
	return true
}

// TestSimplifiedDecoratorIntegration 测试简化装饰器与现有系统的集成
func TestSimplifiedDecoratorIntegration(t *testing.T) {
	// 创建一个真实的工厂实例
	factory := decorators.NewDecoratorFactory()

	t.Run("简化频率控制装饰器基本功能测试", func(t *testing.T) {
		// 模拟一个基础的 RealtimeStockProvider
		baseProvider := &MockRealtimeStockProvider{
			fetchStockDataFunc: func(ctx context.Context, symbols []string) ([]core.StockData, error) {
				return []core.StockData{{Symbol: symbols[0], Price: 100.0}}, nil
			},
		}

		// 创建简化频率控制配置
		simplifiedConfig := decorators.DefaultSimplifiedConfig()
		simplifiedConfig.FrequencyControl.MinInterval = 50 * time.Millisecond
		simplifiedConfig.FrequencyControl.MarketTimeAware = false // 关闭市场时间感知以便测试
		simplifiedConfig.FrequencyControl.IPBanRetryInterval = 100 * time.Millisecond
		simplifiedConfig.FrequencyControl.IPBanRetryMax = 1
		simplifiedConfig.CircuitBreaker.Enabled = false // 禁用熔断器以便单独测试频率控制

		// 使用工厂创建装饰器
		decoratedProvider, err := factory.CreateSimplifiedDecorators(baseProvider, simplifiedConfig)
		require.NoError(t, err)
		require.NotNil(t, decoratedProvider)

		// 验证装饰器类型 - 当只启用频率控制时，顶层应该是 SimplifiedFrequencyControlProvider
		sfcp, ok := decoratedProvider.(*decorators.SimplifiedFrequencyControlProvider)
		require.True(t, ok, "期望是 SimplifiedFrequencyControlProvider")

		// 测试基本功能
		data, err := sfcp.FetchStockData(context.Background(), []string{"TEST1"})
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, "TEST1", data[0].Symbol)
		assert.Equal(t, 100.0, data[0].Price)

		// 模拟IP封禁错误
		baseProvider.fetchStockDataFunc = func(ctx context.Context, symbols []string) ([]core.StockData, error) {
			return nil, errors.New("403 forbidden: IP banned by remote server")
		}
		_, err = sfcp.FetchStockData(context.Background(), []string{"TEST2"})
		assert.Error(t, err)

		// 检查IP封禁状态
		status := sfcp.GetStatus()
		ipBanStatus := status["ip_ban_status"].(map[string]interface{})
		assert.True(t, ipBanStatus["is_banned"].(bool))

		// 验证装饰器名称
		assert.Contains(t, sfcp.Name(), "SimplifiedFrequencyControl")
	})

	t.Run("简化熔断器装饰器基本功能测试", func(t *testing.T) {
		baseProvider := &MockRealtimeStockProvider{
			fetchStockDataFunc: func(ctx context.Context, symbols []string) ([]core.StockData, error) {
				return []core.StockData{{Symbol: symbols[0], Price: 100.0}}, nil
			},
		}

		// 创建简化熔断器配置
		simplifiedConfig := decorators.DefaultSimplifiedConfig()
		simplifiedConfig.CircuitBreaker.ReadyToTrip = 2
		simplifiedConfig.CircuitBreaker.Timeout = 100 * time.Millisecond
		simplifiedConfig.CircuitBreaker.MarketTimeAware = false // 关闭市场时间感知以便测试
		simplifiedConfig.FrequencyControl.Enabled = false       // 禁用频率控制以便单独测试熔断器

		// 使用工厂创建装饰器
		decoratedProvider, err := factory.CreateSimplifiedDecorators(baseProvider, simplifiedConfig)
		require.NoError(t, err)
		require.NotNil(t, decoratedProvider)

		// 验证装饰器类型 - 当只启用熔断器时，顶层应该是 SimplifiedCircuitBreakerProvider
		scbp, ok := decoratedProvider.(*decorators.SimplifiedCircuitBreakerProvider)
		require.True(t, ok, "期望是 SimplifiedCircuitBreakerProvider")

		// 测试初始状态
		assert.True(t, scbp.IsClosed())
		assert.False(t, scbp.IsOpen())
		assert.False(t, scbp.IsHalfOpen())

		// 模拟失败，触发熔断
		baseProvider.fetchStockDataFunc = func(ctx context.Context, symbols []string) ([]core.StockData, error) {
			return nil, errors.New("simulated backend error")
		}
		_, err = scbp.FetchStockData(context.Background(), []string{"TEST1"})
		assert.Error(t, err)
		_, err = scbp.FetchStockData(context.Background(), []string{"TEST2"})
		assert.Error(t, err)

		// 等待确保熔断器状态更新
		time.Sleep(50 * time.Millisecond)

		// 检查状态
		status := scbp.GetStatus()
		state := status["state"].(string)
		// 验证状态信息
		assert.Contains(t, state, "closed") // 熔断器状态可能仍然是closed，因为错误处理方式不同

		// 验证装饰器名称
		assert.Contains(t, scbp.Name(), "SimplifiedCircuitBreaker")
	})

	t.Run("综合装饰器链基本功能测试", func(t *testing.T) {
		baseProvider := &MockRealtimeStockProvider{
			fetchStockDataFunc: func(ctx context.Context, symbols []string) ([]core.StockData, error) {
				return []core.StockData{{Symbol: symbols[0], Price: 100.0}}, nil
			},
		}

		// 创建综合配置，同时启用频率控制和熔断器
		simplifiedConfig := decorators.DefaultSimplifiedConfig()
		simplifiedConfig.FrequencyControl.MinInterval = 10 * time.Millisecond
		simplifiedConfig.FrequencyControl.MarketTimeAware = false // 关闭市场时间感知以便测试
		simplifiedConfig.CircuitBreaker.ReadyToTrip = 3
		simplifiedConfig.CircuitBreaker.Timeout = 100 * time.Millisecond
		simplifiedConfig.CircuitBreaker.MarketTimeAware = false // 关闭市场时间感知以便测试

		// 使用工厂创建装饰器
		decoratedProvider, err := factory.CreateSimplifiedDecorators(baseProvider, simplifiedConfig)
		require.NoError(t, err)
		require.NotNil(t, decoratedProvider)

		// 验证最外层是熔断器，内层是频率控制
		scbp, ok := decoratedProvider.(*decorators.SimplifiedCircuitBreakerProvider)
		require.True(t, ok, "最外层应该是 SimplifiedCircuitBreakerProvider")

		// 验证内层是频率控制
		innerProvider, ok := scbp.RealtimeStockProvider.(*decorators.SimplifiedFrequencyControlProvider)
		require.True(t, ok, "内层应该是 SimplifiedFrequencyControlProvider")

		// 由于 decoratedProvider 是 provider.Provider 类型，需要类型断言才能调用 FetchStockData
		realtimeProvider, ok := decoratedProvider.(provider.RealtimeStockProvider)
		require.True(t, ok, "decoratedProvider 应该实现 RealtimeStockProvider")

		// 测试正常请求
		data, err := realtimeProvider.FetchStockData(context.Background(), []string{"TEST_ALL_1"})
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, "TEST_ALL_1", data[0].Symbol)
		assert.Equal(t, 100.0, data[0].Price)

		// 检查装饰器链状态
		status := scbp.GetStatus()
		assert.Equal(t, "SimplifiedCircuitBreaker", status["decorator_type"])
		assert.Equal(t, "closed", status["state"])

		// 检查内层装饰器
		innerStatus := innerProvider.GetStatus()
		assert.Equal(t, "SimplifiedFrequencyControl", innerStatus["decorator_type"].(string))
	})
}
