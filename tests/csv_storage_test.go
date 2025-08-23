package tests

import (
	"context"
	"os"
	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit"
	"stocksub/pkg/testkit/config"
	"stocksub/pkg/testkit/core"
	"stocksub/pkg/testkit/storage"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestManager 创建一个新的测试管理器
func setupTestManager(t *testing.T) core.TestDataManager {
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type: "memory",
		},
		Storage: config.StorageConfig{
			Type:      "csv",
			Directory: t.TempDir(),
		},
	}
	manager := testkit.NewTestDataManager(cfg)
	return manager
}

// TestCSVStorage_Standalone 测试新的CSVStorage的Save功能
func TestCSVStorage_Standalone(t *testing.T) {
	tempDir := t.TempDir()
	cfg := storage.DefaultCSVStorageConfig()
	cfg.Directory = tempDir

	storage, err := storage.NewCSVStorage(cfg)
	require.NoError(t, err)
	defer storage.Close()

	// 测试数据点
	testData := subscriber.StockData{
		Symbol:        "TEST001",
		Name:          "测试股票",
		Price:         10.50,
		Volume:        123456789,
		Timestamp:     time.Now(),
	}

	// 测试保存数据点
	err = storage.Save(context.Background(), testData)
	require.NoError(t, err)

	// 刷新以确保数据写入文件
	err = storage.Flush()
	require.NoError(t, err)

	// 验证文件已创建
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.NotEmpty(t, files, "CSV file should have been created in temp dir")
	t.Logf("Found file: %s in temp dir", files[0].Name())

	t.Run("ReadDataPoints", func(t *testing.T) {
		t.Skip("Skipping ReadDataPoints test: testkit's CSVStorage.Load is not implemented yet.")
	})
}

// TestForceRefreshCache 测试强制刷新缓存
func TestForceRefreshCache(t *testing.T) {
	manager := setupTestManager(t)
	defer manager.Close()

	symbols := []string{"600000"}

	// 首次调用，填充缓存
	_, err := manager.GetStockData(context.Background(), symbols)
	require.NoError(t, err)

	// 强制刷新
	t.Run("ForceRefresh", func(t *testing.T) {
		// 通过禁用缓存来模拟强制刷新
		manager.EnableCache(false)
		defer manager.EnableCache(true)

		stats_before := manager.GetStats()
		refreshedResults, err := manager.GetStockData(context.Background(), symbols)
		require.NoError(t, err)
		require.Len(t, refreshedResults, 1)
		stats_after := manager.GetStats()

		assert.Equal(t, stats_before.CacheHits, stats_after.CacheHits, "Cache hits should not increase on force refresh")
		t.Log("强制刷新成功，预期重新调用了Provider")
	})
}

// TestCacheWithEmptySymbols 测试空股票列表
func TestCacheWithEmptySymbols(t *testing.T) {
	manager := setupTestManager(t)
	defer manager.Close()

	results, err := manager.GetStockData(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Empty(t, results)
}

// TestCachePartialMiss 测试部分缓存未命中
func TestCachePartialMiss(t *testing.T) {
	// 这是一个更高级的缓存策略，可以作为未来的优化方向
	t.Skip("部分缓存未命中的高级策略，当前版本会触发API调用")
}