
package cache

import (
	"context"
	"fmt"
	"stocksub/pkg/testkit/core"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// 测试MemoryCache基本操作
func TestMemoryCache_BasicOperations(t *testing.T) {
	config := MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	cache := NewMemoryCache(config)
	defer cache.Close()

	ctx := context.Background()

	// 测试Set和Get
	err := cache.Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err)

	value, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)

	// 测试不存在的键
	_, err = cache.Get(ctx, "nonexistent")
	assert.Error(t, err)
	var testKitErr *core.TestKitError
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)

	// 测试Delete
	err = cache.Delete(ctx, "key1")
	assert.NoError(t, err)

	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)
}

// TestMemoryCache_TTL 测试MemoryCache的TTL功能，并验证过期条目在Get时被删除
func TestMemoryCache_TTL(t *testing.T) {
	config := MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      100 * time.Millisecond,
		CleanupInterval: 50 * time.Millisecond,
	}

	cache := NewMemoryCache(config)
	defer cache.Close()

	ctx := context.Background()

	// 设置一个短TTL的值
	err := cache.Set(ctx, "key1", "value1", 50*time.Millisecond)
	assert.NoError(t, err)

	// 确认条目存在
	assert.Equal(t, int64(1), cache.Stats().Size)

	// 等待过期
	time.Sleep(60 * time.Millisecond)

	// 再次获取应该失败
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	var testKitErr *core.TestKitError
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)

	// 验证条目已在Get操作中被删除
	cache.mu.RLock()
	_, exists := cache.entries["key1"]
	cache.mu.RUnlock()
	assert.False(t, exists, "Expired entry should be deleted on Get")
	assert.Equal(t, int64(0), cache.Stats().Size)
}

// 测试MemoryCache统计信息
func TestMemoryCache_Stats(t *testing.T) {
	config := MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	cache := NewMemoryCache(config)
	defer cache.Close()

	ctx := context.Background()

	// 初始统计
	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.Size)
	assert.Equal(t, int64(0), stats.HitCount)
	assert.Equal(t, int64(0), stats.MissCount)

	// 添加数据
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	stats = cache.Stats()
	assert.Equal(t, int64(2), stats.Size)

	// 测试命中和未命中
	cache.Get(ctx, "key1") // hit
	cache.Get(ctx, "key3") // miss

	stats = cache.Stats()
	assert.Equal(t, int64(1), stats.HitCount)
	assert.Equal(t, int64(1), stats.MissCount)
	assert.Equal(t, 0.5, stats.HitRate)
}

// 测试MemoryCache清空功能
func TestMemoryCache_Clear(t *testing.T) {
	config := MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	cache := NewMemoryCache(config)
	defer cache.Close()

	ctx := context.Background()

	// 添加一些数据
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	stats := cache.Stats()
	assert.Equal(t, int64(2), stats.Size)

	// 清空缓存
	err := cache.Clear(ctx)
	assert.NoError(t, err)

	stats = cache.Stats()
	assert.Equal(t, int64(0), stats.Size)
	assert.Equal(t, int64(0), stats.HitCount)
	assert.Equal(t, int64(0), stats.MissCount)

	// 验证数据已清空
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	var testKitErr *core.TestKitError
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)
}

// 测试MemoryCache的evictOldest方法
func TestMemoryCache_EvictOldest(t *testing.T) {
	config := MemoryCacheConfig{
		MaxSize:         3, // 小容量以触发淘汰
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	cache := NewMemoryCache(config)
	defer cache.Close()

	ctx := context.Background()

	// 添加数据以达到最大容量
	cache.Set(ctx, "key1", "value1", 0)
	time.Sleep(10 * time.Millisecond) // 确保创建时间不同
	cache.Set(ctx, "key2", "value2", 0)
	time.Sleep(10 * time.Millisecond)
	cache.Set(ctx, "key3", "value3", 0)

	// 验证所有数据都存在
	_, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	_, err = cache.Get(ctx, "key2")
	assert.NoError(t, err)
	_, err = cache.Get(ctx, "key3")
	assert.NoError(t, err)

	// 添加第4个条目，应该触发淘汰
	time.Sleep(10 * time.Millisecond)
	cache.Set(ctx, "key4", "value4", 0)

	// 至少有一个旧条目应该被淘汰（key1应该是被淘汰的候选）
	// 注意：由于evictOldest是基于创建时间的FIFO策略，key1应该被淘汰
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	var testKitErr *core.TestKitError
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)
}

// 测试estimateSize函数的所有分支
func TestEstimateSize(t *testing.T) {
	assert.Equal(t, int64(5), estimateSize("hello"))
	assert.Equal(t, int64(10), estimateSize([]byte("0123456789")))
	assert.Equal(t, int64(64), estimateSize(12345)) // default case
	assert.Equal(t, int64(64), estimateSize(struct{}{})) // default case
}

// MemoryCache基准测试
func BenchmarkMemoryCache_Set(b *testing.B) {
	config := MemoryCacheConfig{
		MaxSize:         10000,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	cache := NewMemoryCache(config)
	defer cache.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		cache.Set(ctx, key, value, 0)
	}
}

func BenchmarkMemoryCache_Get(b *testing.B) {
	config := MemoryCacheConfig{
		MaxSize:         10000,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	cache := NewMemoryCache(config)
	defer cache.Close()

	ctx := context.Background()

	// 预填充数据
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		cache.Set(ctx, key, value, 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%1000)
		cache.Get(ctx, key)
	}
}

// 测试MemoryCache的cleanup方法
func TestMemoryCache_Cleanup(t *testing.T) {
	config := MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      50 * time.Millisecond,
		CleanupInterval: 10 * time.Millisecond,
	}

	cache := NewMemoryCache(config)
	defer cache.Close()

	ctx := context.Background()

	// 添加一些会过期的条目
	cache.Set(ctx, "key1", "value1", 50*time.Millisecond)
	cache.Set(ctx, "key2", "value2", 50*time.Millisecond)
	cache.Set(ctx, "key3", "value3", 1*time.Hour) // 不会过期的

	// 等待过期和清理
	time.Sleep(100 * time.Millisecond)

	// 验证过期的条目已被清理
	_, err := cache.Get(ctx, "key1")
	assert.Error(t, err)
	_, err = cache.Get(ctx, "key2")
	assert.Error(t, err)

	// 验证未过期的条目仍然存在
	_, err = cache.Get(ctx, "key3")
	assert.NoError(t, err)

	// 检查统计信息
	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.Size)
}
