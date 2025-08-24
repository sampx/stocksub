package cache

import (
	"context"
	"fmt"
	"sync"
	"stocksub/pkg/testkit/core"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// 测试分层缓存的基本操作
func TestLayeredCache_BasicOperations(t *testing.T) {
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
			{
				Type:    LayerMemory,
				MaxSize: 500,
				TTL:     30 * time.Minute,
				Enabled: true,
				Policy:  PolicyLFU,
			},
		},
		PromoteEnabled: true,
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := NewLayeredCache(config)
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 测试Set和Get
	err = layeredCache.Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err)

	value, err := layeredCache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)

	// 测试不存在的键
	_, err = layeredCache.Get(ctx, "nonexistent")
	assert.Error(t, err)
	var testKitErr *core.TestKitError
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)

	// 测试Delete
	err = layeredCache.Delete(ctx, "key1")
	assert.NoError(t, err)

	_, err = layeredCache.Get(ctx, "key1")
	assert.Error(t, err)
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)
}

// 测试分层缓存的写穿透
func TestLayeredCache_WriteThrough(t *testing.T) {
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
			{
				Type:    LayerMemory,
				MaxSize: 500,
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
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 设置值
	err = layeredCache.Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err)

	// 验证两个层都有这个值
	value1, err := layeredCache.layers[0].Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value1)

	value2, err := layeredCache.layers[1].Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value2)
}

// 测试分层缓存的数据提升
func TestLayeredCache_Promotion(t *testing.T) {
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
			{
				Type:    LayerMemory,
				MaxSize: 500,
				TTL:     30 * time.Minute,
				Enabled: true,
				Policy:  PolicyLFU,
			},
		},
		PromoteEnabled: true, // 启用数据提升
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := NewLayeredCache(config)
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 直接在第二层设置数据
	err = layeredCache.layers[1].Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err)

	// 验证第二层有数据
	secondLayerValue, err := layeredCache.layers[1].Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", secondLayerValue)

	// 从分层缓存获取数据（应该从第二层获取并提升到第一层）
	// 注意：数据提升是异步的，但我们主要验证获取操作成功
	value, err := layeredCache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)

	// 等待异步提升完成
	time.Sleep(100 * time.Millisecond)

	// 验证第一层现在也有这个数据（数据提升成功）
	firstLayerValue, err := layeredCache.layers[0].Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", firstLayerValue)
}

// 测试分层缓存的Clear操作
func TestLayeredCache_Clear(t *testing.T) {
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
			{
				Type:    LayerMemory,
				MaxSize: 500,
				TTL:     30 * time.Minute,
				Enabled: true,
				Policy:  PolicyLFU,
			},
		},
		PromoteEnabled: true,
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := NewLayeredCache(config)
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 在各层添加数据
	layeredCache.Set(ctx, "key1", "value1", 0)
	layeredCache.layers[1].Set(ctx, "key2", "value2", 0)

	// 验证数据存在
	_, err = layeredCache.Get(ctx, "key1")
	assert.NoError(t, err)
	_, err = layeredCache.layers[1].Get(ctx, "key2")
	assert.NoError(t, err)

	// 清空缓存
	err = layeredCache.Clear(ctx)
	assert.NoError(t, err)

	// 验证数据已被清除
	_, err = layeredCache.Get(ctx, "key1")
	assert.Error(t, err)
	_, err = layeredCache.layers[1].Get(ctx, "key2")
	assert.Error(t, err)
}

// 测试分层缓存统计信息
func TestLayeredCache_Stats(t *testing.T) {
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
		},
		PromoteEnabled: true,
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := NewLayeredCache(config)
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 初始统计
	stats := layeredCache.Stats()
	assert.Equal(t, int64(0), stats.Size)
	assert.Equal(t, int64(0), stats.HitCount)
	assert.Equal(t, int64(0), stats.MissCount)

	// 添加数据
	layeredCache.Set(ctx, "key1", "value1", 0)
	layeredCache.Set(ctx, "key2", "value2", 0)

	// 重新获取统计信息以更新各层统计
	stats = layeredCache.Stats()
	assert.Equal(t, int64(2), stats.Size)

	// 测试命中和未命中
	layeredCache.Get(ctx, "key1") // hit
	layeredCache.Get(ctx, "key3") // miss

	// 获取分层统计信息来检查命中和未命中计数
	layerStats := layeredCache.GetLayerStats()
	assert.GreaterOrEqual(t, layerStats.TotalHits, int64(1))
	assert.GreaterOrEqual(t, layerStats.TotalMisses, int64(1))
}

// 测试分层缓存GetLayerStats方法
func TestLayeredCache_GetLayerStats(t *testing.T) {
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
			{
				Type:    LayerMemory,
				MaxSize: 500,
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
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 获取初始统计
	stats := layeredCache.GetLayerStats()
	assert.Equal(t, 2, len(stats.LayerStats))

	// 添加数据
	layeredCache.Set(ctx, "key1", "value1", 0)
	layeredCache.layers[1].Set(ctx, "key2", "value2", 0)

	// 再次获取统计
	stats = layeredCache.GetLayerStats()
	assert.Equal(t, int64(1), stats.LayerStats[0].Size)
	assert.Equal(t, int64(1), stats.LayerStats[1].Size)
}

// 测试分层缓存预热功能
func TestLayeredCache_Warm(t *testing.T) {
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
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
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 预热数据
	data := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	err = layeredCache.Warm(ctx, data)
	assert.NoError(t, err)

	// 验证数据已加载
	value, err := layeredCache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)

	value, err = layeredCache.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.Equal(t, "value2", value)

	value, err = layeredCache.Get(ctx, "key3")
	assert.NoError(t, err)
	assert.Equal(t, "value3", value)
}

// 测试创建缓存层的不同类型
func TestCreateCacheLayer(t *testing.T) {
	// 测试内存层
	layerConfig := LayerConfig{
		Type:    LayerMemory,
		MaxSize: 100,
		TTL:     5 * time.Minute,
		Enabled: true,
		Policy:  PolicyLRU,
	}

	// 创建默认工厂映射
	factories := make(map[LayerType]LayerFactory)
	registerDefaultFactories(factories)
	
	layer, err := createCacheLayer(layerConfig, 0, factories)
	assert.NoError(t, err)
	assert.NotNil(t, layer)

	// 测试磁盘层（当前实现返回内存缓存）
	layerConfig.Type = LayerDisk
	layer, err = createCacheLayer(layerConfig, 1, factories)
	assert.NoError(t, err)
	assert.NotNil(t, layer)

	// 测试远程层（当前实现返回内存缓存）
	layerConfig.Type = LayerRemote
	layer, err = createCacheLayer(layerConfig, 2, factories)
	assert.NoError(t, err)
	assert.NotNil(t, layer)

	// 测试不支持的类型
	layerConfig.Type = "unsupported"
	layer, err = createCacheLayer(layerConfig, 3, factories)
	assert.Error(t, err)
	assert.Nil(t, layer)
}

// 测试默认分层缓存配置
func TestDefaultLayeredCacheConfig(t *testing.T) {
	config := DefaultLayeredCacheConfig()
	assert.Equal(t, 2, len(config.Layers))
	assert.True(t, config.PromoteEnabled)
	assert.False(t, config.WriteThrough)
	assert.False(t, config.WriteBack)

	// 验证第一层配置
	firstLayer := config.Layers[0]
	assert.Equal(t, LayerMemory, firstLayer.Type)
	assert.Equal(t, int64(1000), firstLayer.MaxSize)
	assert.Equal(t, 5*time.Minute, firstLayer.TTL)
	assert.True(t, firstLayer.Enabled)
	assert.Equal(t, PolicyLRU, firstLayer.Policy)

	// 验证第二层配置
	secondLayer := config.Layers[1]
	assert.Equal(t, LayerMemory, secondLayer.Type)
	assert.Equal(t, int64(5000), secondLayer.MaxSize)
	assert.Equal(t, 30*time.Minute, secondLayer.TTL)
	assert.True(t, secondLayer.Enabled)
	assert.Equal(t, PolicyLFU, secondLayer.Policy)
}

// 测试分层缓存的批量操作功能
func TestLayeredCache_BatchOperations(t *testing.T) {
	// 测试目的：验证分层缓存的批量获取和批量设置功能
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
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
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 测试批量设置
	items := map[string]any{
		"key1": "value1",
		"key2": "value2", 
		"key3": "value3",
	}

	err = layeredCache.BatchSet(ctx, items, 0)
	assert.NoError(t, err)

	// 测试批量获取
	keys := []string{"key1", "key2", "key3", "key4"}
	result, err := layeredCache.BatchGet(ctx, keys)
	assert.NoError(t, err)

	// 验证结果
	assert.Equal(t, 3, len(result))
	assert.Equal(t, "value1", result["key1"])
	assert.Equal(t, "value2", result["key2"])
	assert.Equal(t, "value3", result["key3"])
	assert.Nil(t, result["key4"])

	// 验证统计信息
	stats := layeredCache.GetLayerStats()
	assert.Equal(t, int64(3), stats.TotalHits) // 三个命中
	assert.Equal(t, int64(1), stats.TotalMisses) // 一个未命中
}

// 测试写穿透模式下的批量操作
func TestLayeredCache_BatchWriteThrough(t *testing.T) {
	// 测试目的：验证写穿透模式下批量操作的正确性
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
			{
				Type:    LayerMemory,
				MaxSize: 500,
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
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 批量设置数据
	items := map[string]any{
		"key1": "value1",
		"key2": "value2",
	}

	err = layeredCache.BatchSet(ctx, items, 0)
	assert.NoError(t, err)

	// 验证两个层都有数据
	for _, key := range []string{"key1", "key2"} {
		value1, err := layeredCache.layers[0].Get(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, items[key], value1)

		value2, err := layeredCache.layers[1].Get(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, items[key], value2)
	}

	// 验证写穿透统计
	stats := layeredCache.GetLayerStats()
	assert.Equal(t, int64(1), stats.WriteThrough) // 一次批量写穿透操作
}

// 测试自定义工厂创建分层缓存
func TestLayeredCache_CustomFactories(t *testing.T) {
	// 测试目的：验证使用自定义工厂创建分层缓存的功能
	
	// 创建自定义内存层工厂
	customFactories := map[LayerType]LayerFactory{
		LayerMemory: &customMemoryLayerFactory{},
	}

	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
		},
		PromoteEnabled: false,
		WriteThrough:   false,
		WriteBack:      false,
	}

	// 使用自定义工厂创建分层缓存
	layeredCache, err := NewLayeredCacheWithFactories(config, customFactories)
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 测试基本操作
	err = layeredCache.Set(ctx, "test_key", "test_value", 0)
	assert.NoError(t, err)

	value, err := layeredCache.Get(ctx, "test_key")
	assert.NoError(t, err)
	assert.Equal(t, "test_value", value)
}

// 自定义内存层工厂实现
// 测试目的：验证工厂模式的扩展性
type customMemoryLayerFactory struct{}

func (f *customMemoryLayerFactory) LayerType() LayerType {
	return LayerMemory
}

func (f *customMemoryLayerFactory) CreateLayer(config LayerConfig, layerIndex int) (core.Cache, error) {
	// 创建自定义配置的内存缓存
	memConfig := MemoryCacheConfig{
		MaxSize:         config.MaxSize,
		DefaultTTL:      config.TTL,
		CleanupInterval: config.CleanupInterval,
	}
	return NewMemoryCache(memConfig), nil
}

// 测试分层缓存的错误处理
func TestLayeredCache_ErrorHandling(t *testing.T) {
	// 测试目的：验证分层缓存的错误处理和上下文信息
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
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
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 测试获取不存在的键的错误信息
	_, err = layeredCache.Get(ctx, "nonexistent_key")
	assert.Error(t, err)
	
	var testKitErr *core.TestKitError
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)
	
	// 验证错误消息包含有用的上下文信息
	errorMsg := err.Error()
	assert.Contains(t, errorMsg, "cache miss")
}

// 测试并发安全性
func TestLayeredCache_Concurrency(t *testing.T) {
	// 测试目的：验证分层缓存在高并发场景下的线程安全性
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
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 并发设置和获取
	numGoroutines := 10
	numOperations := 100
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", goroutineID, j)
				value := fmt.Sprintf("value_%d_%d", goroutineID, j)
				
				// 并发设置
				err := layeredCache.Set(ctx, key, value, 0)
				assert.NoError(t, err)
				
				// 并发获取
				retrieved, err := layeredCache.Get(ctx, key)
				assert.NoError(t, err)
				assert.Equal(t, value, retrieved)
			}
		}(i)
	}

	wg.Wait()

	// 验证所有数据都正确存储
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numOperations; j++ {
			key := fmt.Sprintf("key_%d_%d", i, j)
			expected := fmt.Sprintf("value_%d_%d", i, j)
			
			value, err := layeredCache.Get(ctx, key)
			assert.NoError(t, err)
			assert.Equal(t, expected, value)
		}
	}
}

// 测试关闭操作和资源清理
func TestLayeredCache_Close(t *testing.T) {
	// 测试目的：验证分层缓存关闭时的资源清理
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
		},
		PromoteEnabled: true, // 启用数据提升，需要验证工作协程关闭
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := NewLayeredCache(config)
	assert.NoError(t, err)

	ctx := context.Background()

	// 添加一些数据
	err = layeredCache.Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err)

	// 关闭缓存
	err = layeredCache.Close()
	assert.NoError(t, err)

	// 验证关闭后不能再操作
	_, err = layeredCache.Get(ctx, "key1")
	assert.Error(t, err)
	
	err = layeredCache.Set(ctx, "key2", "value2", 0)
	assert.Error(t, err)
}

// 测试分层缓存的刷新功能（写回模式）
func TestLayeredCache_Flush(t *testing.T) {
	// 测试目的：验证写回模式下的刷新功能
	config := LayeredCacheConfig{
		Layers: []LayerConfig{
			{
				Type:    LayerMemory,
				MaxSize: 100,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  PolicyLRU,
			},
		},
		PromoteEnabled: false,
		WriteThrough:   false,
		WriteBack:      true, // 启用写回模式
	}

	layeredCache, err := NewLayeredCache(config)
	assert.NoError(t, err)
	defer layeredCache.Close()

	ctx := context.Background()

	// 添加数据
	err = layeredCache.Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err)

	// 执行刷新操作
	err = layeredCache.Flush(ctx)
	assert.NoError(t, err)

	// 验证写回统计
	stats := layeredCache.GetLayerStats()
	assert.Equal(t, int64(1), stats.WriteBack) // 一次写回操作
}
