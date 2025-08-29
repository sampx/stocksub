package decorators

import (
	"context"
	"stocksub/pkg/core"
	"stocksub/pkg/provider"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRealtimeProvider 是一个用于测试的模拟实时提供商
type MockRealtimeProvider struct{}

func (m *MockRealtimeProvider) Name() string                { return "mock-realtime" }
func (m *MockRealtimeProvider) IsHealthy() bool             { return true }
func (m *MockRealtimeProvider) GetRateLimit() time.Duration { return 100 * time.Millisecond }
func (m *MockRealtimeProvider) FetchStockData(ctx context.Context, s []string) ([]core.StockData, error) {
	// 返回模拟数据
	data := make([]core.StockData, len(s))
	for i, symbol := range s {
		data[i] = core.StockData{
			Symbol: symbol,
			Price:  100.0 + float64(i),
		}
	}
	return data, nil
}
func (m *MockRealtimeProvider) FetchStockDataWithRaw(ctx context.Context, s []string) ([]core.StockData, string, error) {
	data, err := m.FetchStockData(ctx, s)
	return data, "mock-raw-data", err
}
func (m *MockRealtimeProvider) IsSymbolSupported(s string) bool { return true }

// MockHistoricalProvider 是一个用于测试的模拟历史提供商
type MockHistoricalProvider struct{}

func (m *MockHistoricalProvider) Name() string                { return "mock-historical" }
func (m *MockHistoricalProvider) IsHealthy() bool             { return true }
func (m *MockHistoricalProvider) GetRateLimit() time.Duration { return 1 * time.Second }
func (m *MockHistoricalProvider) FetchHistoricalData(ctx context.Context, s string, start, end time.Time, p string) ([]core.HistoricalData, error) {
	return []core.HistoricalData{
		{
			Symbol:    s,
			Timestamp: start,
			Open:      100.0,
			High:      110.0,
			Low:       95.0,
			Close:     105.0,
			Volume:    1000,
		},
	}, nil
}
func (m *MockHistoricalProvider) GetSupportedPeriods() []string { return []string{"1d", "1h"} }

func TestConfigurableDecoratorChain(t *testing.T) {
	t.Run("为实时提供商应用正确的装饰器", func(t *testing.T) {
		realtimeProvider := &MockRealtimeProvider{}
		config := ProductionDecoratorConfig()

		decorated, err := CreateDecoratedProvider(realtimeProvider, config)
		require.NoError(t, err)
		require.NotNil(t, decorated)

		// 验证装饰器链的类型
		// 验证装饰器链的结构
		// 顶层应该是熔断器
		cbProvider, ok := decorated.(*CircuitBreakerProvider)
		assert.True(t, ok, "顶层装饰器应该是熔断器，实际类型: %T", decorated)

		if cbProvider != nil {
			// 验证名称包含装饰器信息
			assert.Contains(t, cbProvider.Name(), "CircuitBreaker")
			assert.Contains(t, cbProvider.Name(), "mock-realtime")

			// 验证基础提供商链
			baseProvider := cbProvider.GetBaseProvider()
			require.NotNil(t, baseProvider)

			// 下一层应该是频率控制
			fcProvider, ok := baseProvider.(*FrequencyControlProvider)
			assert.True(t, ok, "第二层装饰器应该是频率控制，实际类型: %T", baseProvider)

			if fcProvider != nil {
				// 验证频率控制装饰器
				assert.Contains(t, fcProvider.Name(), "FrequencyControl")

				// 最底层应该是模拟提供商
				originalProvider := fcProvider.GetBaseProvider()
				assert.Equal(t, realtimeProvider, originalProvider)
			}
		}
	})

	t.Run("为历史提供商应用正确的装饰器", func(t *testing.T) {
		historicalProvider := &MockHistoricalProvider{}
		config := ProductionDecoratorConfig()

		decorated, err := CreateDecoratedProvider(historicalProvider, config)
		require.NoError(t, err)
		require.NotNil(t, decorated)

		// 验证装饰器链的类型
		// 顶层应该是熔断器
		cbProvider, ok := decorated.(*CircuitBreakerForHistoricalProvider)
		assert.True(t, ok, "顶层装饰器应该是历史数据熔断器")

		if cbProvider != nil {
			// 验证名称包含装饰器信息
			assert.Contains(t, cbProvider.Name(), "CircuitBreaker")
			assert.Contains(t, cbProvider.Name(), "mock-historical")

			// 验证健康状态
			assert.True(t, cbProvider.IsHealthy())

			// 验证基础提供商链
			baseProvider := cbProvider.GetBaseProvider()
			require.NotNil(t, baseProvider)

			// 下一层应该是频率控制
			fcProvider, ok := baseProvider.(*FrequencyControlForHistoricalProvider)
			assert.True(t, ok, "第二层装饰器应该是历史数据频率控制")

			if fcProvider != nil {
				// 验证频率控制装饰器
				assert.Contains(t, fcProvider.Name(), "FrequencyControl")

				// 最底层应该是模拟提供商
				originalProvider := fcProvider.GetBaseProvider()
				assert.Equal(t, historicalProvider, originalProvider)
			}
		}
	})

	t.Run("测试装饰器功能", func(t *testing.T) {
		realtimeProvider := &MockRealtimeProvider{}
		config := ProductionDecoratorConfig()

		decorated, err := CreateDecoratedProvider(realtimeProvider, config)
		require.NoError(t, err)

		// 测试实际的数据获取功能
		ctx := context.Background()
		symbols := []string{"AAPL", "GOOGL"}

		// 通过类型断言访问 RealtimeStockProvider 接口
		if stockProvider, ok := decorated.(provider.RealtimeStockProvider); ok {
			data, err := stockProvider.FetchStockData(ctx, symbols)
			assert.NoError(t, err)
			assert.Len(t, data, 2)
			assert.Equal(t, "AAPL", data[0].Symbol)
			assert.Equal(t, "GOOGL", data[1].Symbol)

			// 测试带原始数据的方法
			dataWithRaw, raw, err := stockProvider.FetchStockDataWithRaw(ctx, symbols)
			assert.NoError(t, err)
			assert.Len(t, dataWithRaw, 2)
			assert.Equal(t, "mock-raw-data", raw)

			// 测试符号支持检查
			assert.True(t, stockProvider.IsSymbolSupported("AAPL"))
		} else {
			t.Fatal("装饰后的提供商应该实现 RealtimeStockProvider 接口")
		}
	})

	t.Run("测试历史数据装饰器功能", func(t *testing.T) {
		historicalProvider := &MockHistoricalProvider{}
		config := ProductionDecoratorConfig()

		decorated, err := CreateDecoratedProvider(historicalProvider, config)
		require.NoError(t, err)

		// 通过类型断言访问 HistoricalProvider 接口
		if histProvider, ok := decorated.(provider.HistoricalProvider); ok {
			ctx := context.Background()
			start := time.Now().AddDate(0, -1, 0)
			end := time.Now()

			data, err := histProvider.FetchHistoricalData(ctx, "AAPL", start, end, "1d")
			assert.NoError(t, err)
			assert.Len(t, data, 1)
			assert.Equal(t, "AAPL", data[0].Symbol)

			// 测试支持的周期
			periods := histProvider.GetSupportedPeriods()
			assert.Contains(t, periods, "1d")
			assert.Contains(t, periods, "1h")
		} else {
			t.Fatal("装饰后的历史提供商应该实现 HistoricalProvider 接口")
		}
	})
}

func TestDecoratorConfiguration(t *testing.T) {
	t.Run("测试默认配置", func(t *testing.T) {
		config := DefaultDecoratorConfig()
		assert.NotNil(t, config)
		assert.NotEmpty(t, config.All)
	})

	t.Run("测试生产环境配置", func(t *testing.T) {
		config := ProductionDecoratorConfig()
		assert.NotNil(t, config)
		assert.NotEmpty(t, config.All)

		// 验证配置包含期望的装饰器类型
		decoratorTypes := make(map[provider.DecoratorType]bool)
		for _, decorator := range config.All {
			decoratorTypes[decorator.Type] = true
		}
		for _, decorator := range config.Realtime {
			decoratorTypes[decorator.Type] = true
		}
		for _, decorator := range config.Historical {
			decoratorTypes[decorator.Type] = true
		}
		assert.True(t, decoratorTypes[provider.FrequencyControlType])
		assert.True(t, decoratorTypes[provider.CircuitBreakerType])
	})
}

func TestDecoratorChainOrdering(t *testing.T) {
	t.Run("验证装饰器应用顺序", func(t *testing.T) {
		realtimeProvider := &MockRealtimeProvider{}
		config := ProductionDecoratorConfig()

		// 获取已排序的装饰器配置
		chain := NewConfigurableDecoratorChain()
		chain.LoadFromConfig(config)
		sortedDecorators := chain.getSortedEnabledDecorators(realtimeProvider)

		// 验证排序（优先级高的在前）
		for i := 1; i < len(sortedDecorators); i++ {
			assert.True(t, sortedDecorators[i-1].Priority <= sortedDecorators[i].Priority,
				"装饰器应该按优先级升序排列")
		}

		// 应用装饰器并验证链
		decorated, err := chain.Apply(realtimeProvider)
		require.NoError(t, err)
		require.NotNil(t, decorated)

		// 验证最外层是优先级最高的装饰器
		if len(sortedDecorators) > 0 {
			// 由于是升序排列，最后一个应该是优先级最高的外层装饰器
			highestPriority := sortedDecorators[len(sortedDecorators)-1]
			switch highestPriority.Type {
			case provider.CircuitBreakerType:
				_, ok := decorated.(*CircuitBreakerProvider)
				assert.True(t, ok, "最外层应该是熔断器装饰器")
			case provider.FrequencyControlType:
				_, ok := decorated.(*FrequencyControlProvider)
				assert.True(t, ok, "最外层应该是频率控制装饰器")
			}
		}
	})
}

func TestDecoratorChainDetailsImplementation(t *testing.T) {
	t.Run("测试装饰器链内部实现", func(t *testing.T) {
		realtimeProvider := &MockRealtimeProvider{}

		// 创建自定义配置进行更细致的测试
		config := provider.ProviderDecoratorConfig{
			All: []provider.DecoratorConfig{
				{
					Type:         provider.FrequencyControlType,
					Enabled:      true,
					Priority:     1,
					ProviderType: "all",
					Config: map[string]interface{}{
						"min_interval_ms": 100,
						"max_retries":     2,
						"enabled":         true,
					},
				},
				{
					Type:         provider.CircuitBreakerType,
					Enabled:      true,
					Priority:     2,
					ProviderType: "all",
					Config: map[string]interface{}{
						"max_requests":  3,
						"interval":      "30s",
						"timeout":       "15s",
						"ready_to_trip": 2,
						"enabled":       true,
					},
				},
			},
		}

		chain := NewConfigurableDecoratorChain()
		chain.LoadFromConfig(config)

		// 测试 GetAppliedDecorators 方法
		appliedTypes := chain.GetAppliedDecorators(realtimeProvider)
		assert.Len(t, appliedTypes, 2)
		assert.Contains(t, appliedTypes, provider.FrequencyControlType)
		assert.Contains(t, appliedTypes, provider.CircuitBreakerType)

		// 应用装饰器
		decorated, err := chain.Apply(realtimeProvider)
		require.NoError(t, err)
		require.NotNil(t, decorated)

		// 验证装饰器确实被应用
		assert.NotEqual(t, realtimeProvider, decorated)
	})
}

func TestErrorHandling(t *testing.T) {
	t.Run("测试无效提供商类型", func(t *testing.T) {
		config := DefaultDecoratorConfig()
		chain := NewConfigurableDecoratorChain()
		chain.LoadFromConfig(config)

		// 传入 nil 提供商
		decorated, err := chain.Apply(nil)
		assert.Error(t, err)
		assert.Nil(t, decorated)
	})

	t.Run("测试空配置", func(t *testing.T) {
		emptyConfig := provider.ProviderDecoratorConfig{}
		chain := NewConfigurableDecoratorChain()
		chain.LoadFromConfig(emptyConfig)

		realtimeProvider := &MockRealtimeProvider{}
		decorated, err := chain.Apply(realtimeProvider)
		// 空配置应该仍然成功，只是不应用任何装饰器
		assert.NoError(t, err)
		assert.Equal(t, realtimeProvider, decorated)
	})

	t.Run("测试禁用的装饰器", func(t *testing.T) {
		config := provider.ProviderDecoratorConfig{
			All: []provider.DecoratorConfig{
				{
					Type:         provider.FrequencyControlType,
					Enabled:      false, // 禁用
					Priority:     1,
					ProviderType: "all",
				},
			},
		}
		chain := NewConfigurableDecoratorChain()
		chain.LoadFromConfig(config)

		realtimeProvider := &MockRealtimeProvider{}
		decorated, err := chain.Apply(realtimeProvider)
		// 禁用的装饰器不应该被应用
		assert.NoError(t, err)
		assert.Equal(t, realtimeProvider, decorated)
	})
}

// 基准测试
func BenchmarkDecoratorChain(b *testing.B) {
	realtimeProvider := &MockRealtimeProvider{}
	config := ProductionDecoratorConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decorated, err := CreateDecoratedProvider(realtimeProvider, config)
		if err != nil {
			b.Fatal(err)
		}
		_ = decorated
	}
}

func BenchmarkDecoratedFetchStockData(b *testing.B) {
	realtimeProvider := &MockRealtimeProvider{}
	config := ProductionDecoratorConfig()

	decorated, err := CreateDecoratedProvider(realtimeProvider, config)
	if err != nil {
		b.Fatal(err)
	}

	stockProvider := decorated.(provider.RealtimeStockProvider)
	ctx := context.Background()
	symbols := []string{"AAPL", "GOOGL", "MSFT"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := stockProvider.FetchStockData(ctx, symbols)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestSpecificDecoratorTypes(t *testing.T) {
	t.Run("测试仅频率控制装饰器", func(t *testing.T) {
		realtimeProvider := &MockRealtimeProvider{}
		config := provider.ProviderDecoratorConfig{
			All: []provider.DecoratorConfig{
				{
					Type:         provider.FrequencyControlType,
					Enabled:      true,
					Priority:     1,
					ProviderType: "all",
					Config: map[string]interface{}{
						"min_interval_ms": 50,
						"enabled":         true,
					},
				},
			},
		}

		decorated, err := CreateDecoratedProvider(realtimeProvider, config)
		require.NoError(t, err)

		// 应该只有频率控制装饰器
		fcProvider, ok := decorated.(*FrequencyControlProvider)
		assert.True(t, ok, "应该是频率控制装饰器")
		if fcProvider != nil {
			assert.Equal(t, realtimeProvider, fcProvider.GetBaseProvider())
		}
	})

	t.Run("测试仅熔断器装饰器", func(t *testing.T) {
		realtimeProvider := &MockRealtimeProvider{}
		config := provider.ProviderDecoratorConfig{
			All: []provider.DecoratorConfig{
				{
					Type:         provider.CircuitBreakerType,
					Enabled:      true,
					Priority:     1,
					ProviderType: "all",
					Config: map[string]interface{}{
						"enabled": true,
					},
				},
			},
		}

		decorated, err := CreateDecoratedProvider(realtimeProvider, config)
		require.NoError(t, err)

		// 应该只有熔断器装饰器
		cbProvider, ok := decorated.(*CircuitBreakerProvider)
		assert.True(t, ok, "应该是熔断器装饰器")
		if cbProvider != nil {
			assert.Equal(t, realtimeProvider, cbProvider.GetBaseProvider())
		}
	})
}
