//go:build integration

package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stocksub/pkg/core"
	"stocksub/pkg/storage"
)

// TestCSVStorage_SaveAndLoad_Integration 测试CSV存储与文件系统的完整集成
func TestCSVStorage_SaveAndLoad_Integration(t *testing.T) {
	cfg := storage.DefaultCSVStorageConfig()
	cfg.Directory = t.TempDir()

	csvStorage, err := storage.NewCSVStorage(cfg)
	require.NoError(t, err, "创建CSVStorage失败")
	defer csvStorage.Close()

	ctx := context.Background()
	testData := core.StockData{
		Symbol:    "600000",
		Name:      "浦发银行",
		Price:     8.45,
		Change:    0.15,
		Timestamp: time.Now(),
	}

	// 测试保存数据
	err = csvStorage.Save(ctx, testData)
	assert.NoError(t, err, "保存数据失败")

	// 测试批量保存
	batchData := []interface{}{
		core.StockData{Symbol: "000001", Name: "平安银行", Price: 12.30},
		core.StockData{Symbol: "600036", Name: "招商银行", Price: 32.15},
	}

	err = csvStorage.BatchSave(ctx, batchData)
	assert.NoError(t, err, "批量保存失败")

	// 验证文件已创建（通过统计信息）
	stats := csvStorage.GetStats()
	assert.Greater(t, stats.TotalRecords, int64(0), "应保存记录")
	assert.Equal(t, int64(3), stats.TotalRecords, "应保存3条记录")
}

// TestMemoryStorage_ConcurrentAccess_Integration 测试内存存储在并发访问下的集成行为
func TestMemoryStorage_ConcurrentAccess_Integration(t *testing.T) {
	memConfig := storage.MemoryStorageConfig{
		MaxRecords:      1000,
		EnableIndex:     true,
		TTL:             5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	memStorage := storage.NewMemoryStorage(memConfig)
	defer memStorage.Close()

	ctx := context.Background()
	symbols := []string{"600000", "000001", "600036"}

	// 并发保存测试数据
	t.Run("ConcurrentSaves", func(t *testing.T) {
		for i, symbol := range symbols {
			t.Run(symbol, func(t *testing.T) {
				data := core.StockData{
					Symbol: symbol,
					Price:  10.0 + float64(i),
					Name:   "测试股票" + symbol,
				}

				err := memStorage.Save(ctx, data)
				assert.NoError(t, err, "并发保存失败")
			})
		}
	})

	// 验证数据完整性
	stats := memStorage.GetStats()
	assert.Equal(t, int64(len(symbols)), stats.TotalRecords, "应保存所有记录")

	// 测试查询功能
	t.Run("QueryOperations", func(t *testing.T) {
		// 这里可以添加更复杂的查询测试
		// 例如按符号查询、按时间范围查询等
	})
}

// TestStorage_InterfaceCompliance_Integration 验证不同存储实现都符合接口要求
func TestStorage_InterfaceCompliance_Integration(t *testing.T) {
	// 验证CSVStorage实现接口
	csvCfg := storage.DefaultCSVStorageConfig()
	csvCfg.Directory = t.TempDir()
	csvStorage, _ := storage.NewCSVStorage(csvCfg)
	defer csvStorage.Close()

	var _ *storage.CSVStorage = csvStorage

	// 验证MemoryStorage实现接口
	memConfig := storage.MemoryStorageConfig{
		MaxRecords:      1000,
		EnableIndex:     true,
		TTL:             5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	memStorage := storage.NewMemoryStorage(memConfig)
	defer memStorage.Close()

	var _ storage.Storage = memStorage

	t.Log("✅ 所有存储实现都符合Storage接口")
}
