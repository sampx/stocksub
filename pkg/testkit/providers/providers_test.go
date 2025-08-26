package providers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/cache"
	"stocksub/pkg/testkit/core"
)

// TestMockProvider_ImplementsSubscriberProvider ensures that MockProvider implements the subscriber.Provider interface.
func TestMockProvider_ImplementsSubscriberProvider(t *testing.T) {
	var _ subscriber.Provider = (*MockProvider)(nil)
}

func TestMockProvider_BasicOperations(t *testing.T) {
	config := DefaultMockProviderConfig()
	config.DefaultDelay = 10 * time.Millisecond

	provider := NewMockProvider(config)
	defer provider.Close()

	ctx := context.Background()
	symbols := []string{"TEST001", "TEST002"}

	// 测试基本数据获取
	data, err := provider.FetchData(ctx, symbols)
	assert.NoError(t, err)
	assert.Len(t, data, 2)
	assert.Equal(t, "TEST001", data[0].Symbol)
	assert.Equal(t, "TEST002", data[1].Symbol)

	// 验证统计信息
	stats := provider.GetStats()
	assert.Equal(t, int64(1), stats.TotalCalls)
	assert.Equal(t, int64(1), stats.SuccessfulCalls)
	assert.Equal(t, int64(0), stats.FailedCalls)
	assert.Greater(t, stats.AverageDelay, time.Duration(0))
}

func TestMockProvider_SetMockData(t *testing.T) {
	config := DefaultMockProviderConfig()
	provider := NewMockProvider(config)
	defer provider.Close()

	ctx := context.Background()

	// 设置自定义Mock数据
	symbols := []string{"CUSTOM001"}
	mockData := []subscriber.StockData{
		{
			Symbol: "CUSTOM001",
			Name:   "自定义股票",
			Price:  999.99,
			Change: 50.00,
		},
	}

	provider.SetMockData(symbols, mockData)

	// 获取数据
	data, err := provider.FetchData(ctx, symbols)
	assert.NoError(t, err)
	assert.Len(t, data, 1)
	assert.Equal(t, 999.99, data[0].Price)
	assert.Equal(t, "自定义股票", data[0].Name)
}

func TestMockProvider_Scenarios(t *testing.T) {
	config := DefaultMockProviderConfig()
	provider := NewMockProvider(config)
	defer provider.Close()

	ctx := context.Background()

	// 测试错误场景
	err := provider.SetScenario("error")
	assert.NoError(t, err)

	symbols := []string{"TEST001"}
	_, err = provider.FetchData(ctx, symbols)
	require.Error(t, err)
	require.Contains(t, err.Error(), "API服务暂时不可用")

	// 切换到正常场景
	err = provider.SetScenario("normal")
	assert.NoError(t, err)

	data, err := provider.FetchData(ctx, symbols)
	assert.NoError(t, err)
	assert.Len(t, data, 1)

	// 测试统计信息
	stats := provider.GetStats()
	assert.Equal(t, "normal", stats.CurrentScenario)
	assert.Greater(t, stats.TotalCalls, int64(0))
	assert.Greater(t, stats.FailedCalls, int64(0))
}

func TestMockProvider_SlowScenario(t *testing.T) {
	config := DefaultMockProviderConfig()
	provider := NewMockProvider(config)
	defer provider.Close()

	ctx := context.Background()

	// 设置慢速场景
	err := provider.SetScenario("slow")
	assert.NoError(t, err)

	symbols := []string{"TEST001"}

	start := time.Now()
	data, err := provider.FetchData(ctx, symbols)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Len(t, data, 1)
	assert.Greater(t, duration, 2*time.Second, "应该有延迟")
}

func TestMockProvider_CustomScenario(t *testing.T) {
	config := DefaultMockProviderConfig()
	provider := NewMockProvider(config)
	defer provider.Close()

	// 添加自定义场景
	customScenario := &core.MockScenario{
		Name:        "custom",
		Description: "自定义测试场景",
		Responses: map[string]core.MockResponse{
			"CUSTOM001": {
				Data: []subscriber.StockData{
					{
						Symbol: "CUSTOM001",
						Name:   "自定义股票",
						Price:  888.88,
					},
				},
			},
		},
		Delays: map[string]time.Duration{
			"CUSTOM001": 100 * time.Millisecond,
		},
		Errors: make(map[string]error),
	}

	provider.AddScenario(customScenario)

	// 使用自定义场景
	err := provider.SetScenario("custom")
	assert.NoError(t, err)

	ctx := context.Background()
	symbols := []string{"CUSTOM001"}

	start := time.Now()
	data, err := provider.FetchData(ctx, symbols)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Len(t, data, 1)
	assert.Equal(t, 888.88, data[0].Price)
	assert.Greater(t, duration, 100*time.Millisecond)
}

func TestCallRecorder(t *testing.T) {
	recorder := NewCallRecorder(10)

	// 记录调用
	record1 := CallRecord{
		ID:        "call1",
		Timestamp: time.Now(),
		Symbols:   []string{"TEST001"},
		Response:  []subscriber.StockData{{Symbol: "TEST001"}},
		Duration:  100 * time.Millisecond,
		Scenario:  "normal",
	}

	recorder.RecordCall(record1)

	record2 := CallRecord{
		ID:        "call2",
		Timestamp: time.Now(),
		Symbols:   []string{"TEST002"},
		Response:  []subscriber.StockData{{Symbol: "TEST002"}},
		Duration:  200 * time.Millisecond,
		Scenario:  "normal",
	}

	recorder.RecordCall(record2)

	// 获取所有记录
	calls := recorder.GetCalls()
	assert.Len(t, calls, 2)
	assert.Equal(t, "call1", calls[0].ID)
	assert.Equal(t, "call2", calls[1].ID)

	// 按股票符号获取记录
	test001Calls := recorder.GetCallsBySymbol("TEST001")
	assert.Len(t, test001Calls, 1)
	assert.Equal(t, "call1", test001Calls[0].ID)

	// 清空记录
	recorder.Clear()
	calls = recorder.GetCalls()
	assert.Len(t, calls, 0)
}

func TestCachedProvider_BasicOperations(t *testing.T) {
	// 创建内存缓存
	memConfig := cache.MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	cacheLayer := cache.NewMemoryCache(memConfig)
	defer cacheLayer.Close()

	// 创建一个Mock Provider作为底层的真实Provider
	realProvider := NewMockProvider(DefaultMockProviderConfig())

	// 创建缓存Provider
	config := DefaultCachedProviderConfig()
	config.CacheTTL = 1 * time.Minute

	provider := NewCachedProvider(realProvider, cacheLayer, config)
	defer provider.Close()

	ctx := context.Background()
	symbols := []string{"CACHED001"}

	// 第一次调用，应该会调用realProvider并缓存结果
	data1, err := provider.FetchData(ctx, symbols)
	assert.NoError(t, err)
	assert.Len(t, data1, 1)

	// 第二次调用，应该从缓存获取
	data2, err := provider.FetchData(ctx, symbols)
	assert.NoError(t, err)
	assert.Len(t, data2, 1)

	// 验证统计信息
	stats := provider.GetStats()
	assert.Equal(t, int64(2), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.CacheHits)
	assert.Equal(t, int64(1), stats.RealProviderCalls)
}

func TestCachedProvider_MockMode(t *testing.T) {
	memConfig := cache.MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	cacheLayer := cache.NewMemoryCache(memConfig)
	defer cacheLayer.Close()

	config := DefaultCachedProviderConfig()
	realProvider := NewMockProvider(DefaultMockProviderConfig())
	provider := NewCachedProvider(realProvider, cacheLayer, config)
	defer provider.Close()

	ctx := context.Background()
	symbols := []string{"MOCK001"}

	// 启用Mock模式
	provider.SetMockMode(true)

	// 设置Mock数据
	mockData := []subscriber.StockData{
		{
			Symbol: "MOCK001",
			Name:   "Mock股票",
			Price:  123.45,
		},
	}
	provider.SetMockData(symbols, mockData)

	// 获取数据
	data, err := provider.FetchData(ctx, symbols)
	assert.NoError(t, err)
	assert.Len(t, data, 1)
	assert.Equal(t, 123.45, data[0].Price)

	// 验证统计信息
	stats := provider.GetStats()
	assert.Greater(t, stats.MockProviderCalls, int64(0))
}
