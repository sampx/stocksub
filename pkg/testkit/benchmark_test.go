
package testkit_test

import (
	"context"
	"fmt"
	"runtime"
	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit"
	"stocksub/pkg/testkit/cache"
	"stocksub/pkg/testkit/config"
	
	"stocksub/pkg/testkit/storage"
	"testing"
	"time"
)

// BenchmarkTestDataManager 基准测试TestDataManager的性能
func BenchmarkTestDataManager_GetStockData_VariousSizes(b *testing.B) {
	cfg := &config.Config{
		Cache:   config.CacheConfig{Type: "memory"},
		Storage: config.StorageConfig{Type: "csv", Directory: b.TempDir()},
	}
	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	// 启用Mock模式，避免真实API调用
	manager.EnableMock(true)
	allSymbols := []string{"600000", "000001", "688036", "835174", "300750"}
	mockData := make([]subscriber.StockData, len(allSymbols))
	for i, s := range allSymbols {
		mockData[i] = subscriber.StockData{Symbol: s, Price: 100.0 + float64(i)}
	}
	manager.SetMockData(allSymbols, mockData)

	b.Run("GetStockData_SmallBatch", func(b *testing.B) {
		symbols := []string{"600000", "000001"}
		ctx := context.Background()
		_, _ = manager.GetStockData(ctx, symbols) // 预热

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := manager.GetStockData(ctx, symbols)
			if err != nil {
				b.Fatalf("基准测试失败: %v", err)
			}
		}
	})

	b.Run("GetStockData_LargeBatch", func(b *testing.B) {
		symbols := []string{"600000", "000001", "688036", "835174", "300750"}
		ctx := context.Background()
		_, _ = manager.GetStockData(ctx, symbols) // 预热

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := manager.GetStockData(ctx, symbols)
			if err != nil {
				b.Fatalf("批量基准测试失败: %v", err)
			}
		}
	})

	b.Run("GetStats_Performance", func(b *testing.B) {
		_, _ = manager.GetStockData(context.Background(), []string{"600000"}) // 预填充

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			stats := manager.GetStats()
			if stats.CacheHits == 0 && stats.CacheMisses == 0 {
				// 在高频调用下，stats可能暂时为空，但不应是致命错误
			}
		}
	})
}

// BenchmarkCSVStorage 基准测试CSV存储性能
func BenchmarkCSVStorage_WriteOperations_SingleAndBatch(b *testing.B) {
	cfg := storage.DefaultCSVStorageConfig()
	cfg.Directory = b.TempDir()

	storage, err := storage.NewCSVStorage(cfg)
	if err != nil {
		b.Fatalf("创建storage失败: %v", err)
	}
	defer storage.Close()

	b.Run("Save_Single", func(b *testing.B) {
		testData := subscriber.StockData{Symbol: "BENCH001", Price: 123.45}
		ctx := context.Background()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			testData.Timestamp = time.Now().Add(time.Duration(i) * time.Microsecond)
			if err := storage.Save(ctx, testData); err != nil {
				b.Fatalf("保存数据点失败: %v", err)
			}
		}
	})

	b.Run("BatchSave_10_Items", func(b *testing.B) {
		var batchData []interface{}
		for i := 0; i < 10; i++ {
			batchData = append(batchData, subscriber.StockData{Symbol: fmt.Sprintf("BATCH%03d", i), Price: 100.0 + float64(i)})
		}
		ctx := context.Background()

		b.ResetTimer()
        b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			if err := storage.BatchSave(ctx, batchData); err != nil {
				b.Fatalf("批量保存数据点失败: %v", err)
			}
		}
	})

	b.Run("Read_Performance", func(b *testing.B) {
		b.Skip("Skipping Read benchmark: testkit's CSVStorage.Load is not implemented yet.")
	})
}

// BenchmarkMemoryUsage 内存使用基准测试
func BenchmarkTestDataManager_GetStockData_MemoryUsage(b *testing.B) {
	b.Run("Manager_MemoryUsage", func(b *testing.B) {
		cfg := &config.Config{
			Cache:   config.CacheConfig{Type: "memory"},
			Storage: config.StorageConfig{Type: "csv", Directory: b.TempDir()},
		}
		manager := testkit.NewTestDataManager(cfg)
		defer manager.Close()

		// 启用Mock模式，避免真实API调用
		manager.EnableMock(true)
		symbols := []string{"600000", "000001", "688036", "835174", "300750"}
		mockData := make([]subscriber.StockData, len(symbols))
		for i, s := range symbols {
			mockData[i] = subscriber.StockData{Symbol: s, Price: 100.0 + float64(i)}
		}
		manager.SetMockData(symbols, mockData)
		ctx := context.Background()
		_, _ = manager.GetStockData(ctx, symbols) // 预热

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = manager.GetStockData(ctx, symbols)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
		b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
	})
}

// BenchmarkConcurrency 并发性能基准测试
func BenchmarkManagerAndStorage_Concurrency_ReadAndWrite(b *testing.B) {
	b.Run("Concurrent_Read_Manager", func(b *testing.B) {
		cfg := &config.Config{
			Cache:   config.CacheConfig{Type: "memory"},
			Storage: config.StorageConfig{Type: "csv", Directory: b.TempDir()},
		}
		manager := testkit.NewTestDataManager(cfg)
		defer manager.Close()

		// 启用Mock模式，避免真实API调用
		manager.EnableMock(true)
		symbols := []string{"600000", "000001"}
		mockData := []subscriber.StockData{
			{Symbol: "600000", Price: 100.0},
			{Symbol: "000001", Price: 200.0},
		}
		manager.SetMockData(symbols, mockData)
		ctx := context.Background()
		_, _ = manager.GetStockData(ctx, symbols) // 预热

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := manager.GetStockData(ctx, symbols)
				if err != nil {
					b.Fatalf("并发读取失败: %v", err)
				}
			}
		})
	})

	b.Run("Concurrent_Write_Storage", func(b *testing.B) {
		cfg := storage.DefaultCSVStorageConfig()
		cfg.Directory = b.TempDir()
		storage, _ := storage.NewCSVStorage(cfg)
		defer storage.Close()
		ctx := context.Background()

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			counter := 0
			for pb.Next() {
				data := subscriber.StockData{Symbol: fmt.Sprintf("CONC%06d", counter), Price: 1.0}
				counter++
				if err := storage.Save(ctx, data); err != nil {
					b.Fatalf("并发写入失败: %v", err)
				}
			}
		})
	})
}

func BenchmarkTestDataManager_GetStockData(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "memory",
			MaxSize: 1000,
			TTL:     10 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "memory",
			Directory: tmpDir,
		},
	}

	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()
	symbols := []string{"BENCH001", "BENCH002"}

	// 启用Mock模式
	manager.EnableMock(true)
	mockData := []subscriber.StockData{
		{Symbol: "BENCH001", Price: 100.00},
		{Symbol: "BENCH002", Price: 200.00},
	}
	manager.SetMockData(symbols, mockData)

	// 预热
	manager.GetStockData(ctx, symbols)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			manager.GetStockData(ctx, symbols)
		}
	})
}

// --- Migrated Benchmarks from pkg/testkit/cache ---

// BenchmarkCache_Layered_Set measures the performance of setting values in a layered cache.
// Origin: pkg/testkit/cache/layered_bench_test.go
func BenchmarkCache_Layered_Set(b *testing.B) {
	config := cache.LayeredCacheConfig{
		Layers: []cache.LayerConfig{
			{
				Type:    cache.LayerMemory,
				MaxSize: 1000,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  cache.PolicyLRU,
			},
			{
				Type:    cache.LayerMemory,
				MaxSize: 5000,
				TTL:     30 * time.Minute,
				Enabled: true,
				Policy:  cache.PolicyLFU,
			},
		},
		PromoteEnabled: false,
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := cache.NewLayeredCache(config)
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

// BenchmarkCache_Layered_Get measures the performance of getting values from a layered cache.
// Origin: pkg/testkit/cache/layered_bench_test.go
func BenchmarkCache_Layered_Get(b *testing.B) {
	config := cache.LayeredCacheConfig{
		Layers: []cache.LayerConfig{
			{
				Type:    cache.LayerMemory,
				MaxSize: 1000,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  cache.PolicyLRU,
			},
		},
		PromoteEnabled: false,
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := cache.NewLayeredCache(config)
	if err != nil {
		b.Fatal(err)
	}
	defer layeredCache.Close()

	ctx := context.Background()

	// Pre-fill data
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

// BenchmarkCache_Layered_BatchOperations measures the performance of batch operations in a layered cache.
// Origin: pkg/testkit/cache/layered_bench_test.go
func BenchmarkCache_Layered_BatchOperations(b *testing.B) {
	config := cache.LayeredCacheConfig{
		Layers: []cache.LayerConfig{
			{
				Type:    cache.LayerMemory,
				MaxSize: 10000,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  cache.PolicyLRU,
			},
		},
		PromoteEnabled: false,
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := cache.NewLayeredCache(config)
	if err != nil {
		b.Fatal(err)
	}
	defer layeredCache.Close()

	ctx := context.Background()

	// Prepare batch data
	batchSize := 100
	items := make(map[string]any, batchSize)
	for i := 0; i < batchSize; i++ {
		key := fmt.Sprintf("batch_key%d", i)
		items[key] = fmt.Sprintf("batch_value%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Batch set
		layeredCache.BatchSet(ctx, items, 0)

		// Batch get
		keys := make([]string, 0, batchSize)
		for k := range items {
			keys = append(keys, k)
		}
		layeredCache.BatchGet(ctx, keys)
	}
}

// BenchmarkCache_Layered_WriteThrough measures the performance of write-through mode in a layered cache.
// Origin: pkg/testkit/cache/layered_bench_test.go
func BenchmarkCache_Layered_WriteThrough(b *testing.B) {
	config := cache.LayeredCacheConfig{
		Layers: []cache.LayerConfig{
			{
				Type:    cache.LayerMemory,
				MaxSize: 1000,
				TTL:     5 * time.Minute,
				Enabled: true,
				Policy:  cache.PolicyLRU,
			},
			{
				Type:    cache.LayerMemory,
				MaxSize: 5000,
				TTL:     30 * time.Minute,
				Enabled: true,
				Policy:  cache.PolicyLFU,
			},
		},
		PromoteEnabled: false,
		WriteThrough:   true, // Enable write-through
		WriteBack:      false,
	}

	layeredCache, err := cache.NewLayeredCache(config)
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

// --- Migrated Benchmarks from pkg/testkit/cache/memory_test.go ---

// BenchmarkCache_Memory_Set measures the performance of setting values in a memory cache.
// Origin: pkg/testkit/cache/memory_test.go
func BenchmarkCache_Memory_Set(b *testing.B) {
	config := cache.MemoryCacheConfig{
		MaxSize:         10000,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	memCache := cache.NewMemoryCache(config)
	defer memCache.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		memCache.Set(ctx, key, value, 0)
	}
}

// BenchmarkCache_Memory_Get measures the performance of getting values from a memory cache.
// Origin: pkg/testkit/cache/memory_test.go
func BenchmarkCache_Memory_Get(b *testing.B) {
	config := cache.MemoryCacheConfig{
		MaxSize:         10000,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	memCache := cache.NewMemoryCache(config)
	defer memCache.Close()

	ctx := context.Background()

	// Pre-fill data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		memCache.Set(ctx, key, value, 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%1000)
		memCache.Get(ctx, key)
	}
}
