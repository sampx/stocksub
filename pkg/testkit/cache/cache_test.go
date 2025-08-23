package cache

import (
	"context"
	"fmt"
	"stocksub/pkg/testkit/core"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
	err := cache.Set(ctx, "key1", "value1", 100*time.Millisecond)
	assert.NoError(t, err)

	// 立即获取应该成功
	value, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 再次获取应该失败
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	var testKitErr *core.TestKitError
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)
}

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

// 基准测试
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
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i)
			value := fmt.Sprintf("value%d", i)
			cache.Set(ctx, key, value, 0)
			i++
		}
	})
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
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%1000)
			cache.Get(ctx, key)
			i++
		}
	})
}