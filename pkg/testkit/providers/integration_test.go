//go:build integration

package providers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stocksub/pkg/cache"
	"stocksub/pkg/testkit/providers"
)

// TestCachedProvider_WithRealAPI 是一个集成测试，用于验证CachedProvider与真实API的交互。
// 这个测试会发起真实的外部网络请求，并且只在提供了 'integration' 构建标签时运行。
func TestCachedProvider_FetchData_WithRealAPI_ShowsCacheHitAndMissBehavior(t *testing.T) {
	// 1. 创建依赖项：一个内存缓存和一个Provider工厂
	memConfig := cache.MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	cacheLayer := cache.NewMemoryCache(memConfig)
	defer cacheLayer.Close()

	factory := providers.NewProviderFactory(cacheLayer)

	// 2. 使用工厂创建CachedProvider，它内部会包含一个真实的TencentProvider
	config := providers.DefaultCachedProviderConfig()
	config.CacheTTL = 1 * time.Minute
	provider := factory.CreateCachedProvider(config)
	defer provider.Close()

	ctx := context.Background()
	// 使用一个真实的、不带前缀的股票代码
	symbols := []string{"600519"} // 贵州茅台

	// 3. 第一次调用，应该会触发真实API调用，并将结果存入缓存
	t.Log("第一次调用 (缓存未命中，将调用真实API)...")
	data1, err := provider.FetchData(ctx, symbols)
	require.NoError(t, err, "第一次API调用不应失败")
	require.Len(t, data1, 1, "第一次调用应返回1条数据")
	assert.Equal(t, "600519", data1[0].Symbol, "返回的股票代码应为无前缀格式")
	assert.NotEmpty(t, data1[0].Name, "股票名称不应为空")

	// 4. 第二次调用，应该直接从缓存获取数据
	t.Log("第二次调用 (应从缓存命中)...")
	data2, err := provider.FetchData(ctx, symbols)
	require.NoError(t, err, "第二次缓存获取不应失败")
	require.Len(t, data2, 1, "第二次调用应返回1条数据")
	assert.Equal(t, "600519", data2[0].Symbol, "从缓存返回的股票代码应为无前缀格式")

	// 5. 验证数据一致性
	assert.Equal(t, data1[0].Name, data2[0].Name, "两次获取的股票名称应一致")

	// 6. 验证缓存统计信息
	stats := provider.GetStats()
	t.Logf("最终统计信息: %+v", stats)
	assert.Equal(t, int64(2), stats.TotalRequests, "总请求数应为2")
	assert.Equal(t, int64(1), stats.CacheHits, "缓存命中数应为1")
	assert.Equal(t, int64(1), stats.CacheMisses, "缓存未命中数应为1")
	assert.Equal(t, int64(1), stats.RealProviderCalls, "真实Provider调用次数应为1")
}
