package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// 分层缓存设置操作的基准测试
// 测试目的：测量分层缓存的设置操作性能
func BenchmarkLayeredCache_Set(b *testing.B) {
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 1000,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
			{
				Type:    LayerMemory,
				MaxSize: 5000,
				TTL:     30 * time.Minute,
				Enabled: true,
				Policy:  PolicyLFU,
			},
		},
		PromoteEnabled: false,
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := NewLayeredCache(config)
	if err != nil {
		b.Fatal(err)
	}
	defer layeredCache.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		layeredCache.Set(ctx, key, value, 0)
	}
}

// 分层缓存获取操作的基准测试
// 测试目的：测量分层缓存的获取操作性能
func BenchmarkLayeredCache_Get(b *testing.B) {
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 1000,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
		},
		PromoteEnabled: false,
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := NewLayeredCache(config)
	if err != nil {
		b.Fatal(err)
	}
	defer layeredCache.Close()

	ctx := context.Background()

	// 预填充数据
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		layeredCache.Set(ctx, key, value, 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%1000)
		layeredCache.Get(ctx, key)
	}
}

// 分层缓存批量操作的基准测试
// 测试目的：测量分层缓存的批量操作性能
func BenchmarkLayeredCache_BatchOperations(b *testing.B) {
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 10000,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
		},
		PromoteEnabled: false,
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := NewLayeredCache(config)
	if err != nil {
		b.Fatal(err)
	}
	defer layeredCache.Close()

	ctx := context.Background()

	// 准备批量数据
	batchSize := 100
	items := make(map[string]any, batchSize)
	for i := 0; i < batchSize; i++ {
		key := fmt.Sprintf("batch_key%d", i)
		items[key] = fmt.Sprintf("batch_value%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 批量设置
		layeredCache.BatchSet(ctx, items, 0)

		// 批量获取
		keys := make([]string, 0, batchSize)
		for k := range items {
			keys = append(keys, k)
		}
		layeredCache.BatchGet(ctx, keys)
	}
}

// 写穿透模式下的基准测试
// 测试目的：测量写穿透模式对性能的影响
func BenchmarkLayeredCache_WriteThrough(b *testing.B) {
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 1000,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
			{
				Type:    LayerMemory,
				MaxSize: 5000,
				TTL:     30 * time.Minute,
				Enabled: true,
				Policy:  PolicyLFU,
			},
		},
		PromoteEnabled: false,
		WriteThrough:   true, // 启用写穿透
		WriteBack:      false,
	}

	layeredCache, err := NewLayeredCache(config)
	if err != nil {
		b.Fatal(err)
	}
	defer layeredCache.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("write_through_key%d", i)
		value := fmt.Sprintf("write_through_value%d", i)
		layeredCache.Set(ctx, key, value, 0)
	}
}
