//go:build integration

package tests

import (
	"context"
	"fmt"
	"runtime"
	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit"
	"stocksub/pkg/testkit/config"
	"stocksub/pkg/testkit/storage"
	"testing"
	"time"
)

// BenchmarkTestDataManager 基准测试TestDataManager的性能
func BenchmarkTestDataManager(b *testing.B) {
	cfg := &config.Config{
		Cache:   config.CacheConfig{Type: "memory"},
		Storage: config.StorageConfig{Type: "csv", Directory: b.TempDir()},
	}
	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

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
func BenchmarkCSVStorage(b *testing.B) {
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
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("Manager_MemoryUsage", func(b *testing.B) {
		cfg := &config.Config{
			Cache:   config.CacheConfig{Type: "memory"},
			Storage: config.StorageConfig{Type: "csv", Directory: b.TempDir()},
		}
		manager := testkit.NewTestDataManager(cfg)
		defer manager.Close()

		symbols := []string{"600000", "000001", "688036", "835174", "300750"}
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
func BenchmarkConcurrency(b *testing.B) {
	b.Run("Concurrent_Read_Manager", func(b *testing.B) {
		cfg := &config.Config{
			Cache:   config.CacheConfig{Type: "memory"},
			Storage: config.StorageConfig{Type: "csv", Directory: b.TempDir()},
		}
		manager := testkit.NewTestDataManager(cfg)
		defer manager.Close()

		symbols := []string{"600000", "000001"}
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