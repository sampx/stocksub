//go:build integration

package testkit_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stocksub/pkg/core"
	"stocksub/pkg/testkit"
	"stocksub/pkg/testkit/config"
	"stocksub/pkg/testkit/manager"
)

func TestTestDataManager_GetStockData_WithMockData_ReturnsExpectedResults(t *testing.T) {
	tmpDir := t.TempDir()
	fixedTime := time.Date(2025, 8, 22, 15, 30, 0, 0, time.UTC)

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "memory",
			MaxSize: 100,
			TTL:     5 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "csv",
			Directory: tmpDir,
		},
	}

	manager := manager.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()
	symbols := []string{"INTG001", "INTG002"}

	manager.EnableMock(true)

	mockData := []core.StockData{
		{
			Symbol:        "INTG001",
			Name:          "集成测试股票1",
			Price:         100.50,
			Change:        2.50,
			ChangePercent: 2.55,
			Volume:        1000000,
			Timestamp:     fixedTime,
		},
		{
			Symbol:        "INTG002",
			Name:          "集成测试股票2",
			Price:         200.75,
			Change:        -1.25,
			ChangePercent: -0.62,
			Volume:        500000,
			Timestamp:     fixedTime,
		},
	}

	manager.SetMockData(symbols, mockData)

	data1, err := manager.GetStockData(ctx, symbols)
	assert.NoError(t, err)
	assert.Len(t, data1, 2)
	assert.Equal(t, "INTG001", data1[0].Symbol)
	assert.Equal(t, "INTG002", data1[1].Symbol)

	// Note: In this integration test, we are primarily concerned with the end-to-end flow.
	// Detailed cache stats are verified in TestTestkit_TestDataManager_CacheIntegration.
	_ = manager.GetStats() // 获取统计信息但不使用，避免编译警告
}

func TestTestDataManager_GetStockData_WithLayeredCache_ShowsCacheHits(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "layered",
			MaxSize: 50,
			TTL:     1 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "memory",
			Directory: tmpDir,
		},
	}

	manager := manager.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()
	symbols := []string{"CACHE001"}

	manager.EnableMock(true)

	mockData := []core.StockData{
		{
			Symbol: "CACHE001",
			Name:   "缓存测试股票",
			Price:  150.00,
		},
	}

	manager.SetMockData(symbols, mockData)

	for i := 0; i < 5; i++ {
		data, err := manager.GetStockData(ctx, symbols)
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, "CACHE001", data[0].Symbol)
	}

	stats := manager.GetStats()
	assert.Greater(t, stats.CacheHits, int64(0))
}

func TestTestDataManager_GetStockData_WithCSVStorage_CreatesFiles(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "memory",
			MaxSize: 10,
			TTL:     1 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "csv",
			Directory: tmpDir,
		},
	}

	manager := manager.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()
	symbols := []string{"STORE001", "STORE002"}

	manager.EnableMock(true)

	mockData := []core.StockData{
		{
			Symbol: "STORE001",
			Name:   "存储测试股票1",
			Price:  75.25,
		},
		{
			Symbol: "STORE002",
			Name:   "存储测试股票2",
			Price:  125.75,
		},
	}

	manager.SetMockData(symbols, mockData)

	data, err := manager.GetStockData(ctx, symbols)
	assert.NoError(t, err)
	assert.Len(t, data, 2)

	time.Sleep(500 * time.Millisecond)

	files, err := os.ReadDir(tmpDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, files, "存储文件应已创建")
}

// --- Migrated from pkg/testkit/storage/integration_test.go ---

// setupTestManager 创建一个新的测试管理器
func setupTestManager(t *testing.T) testkit.TestDataManager {
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type: "memory",
		},
		Storage: config.StorageConfig{
			Type:      "csv",
			Directory: t.TempDir(),
		},
	}
	manager := manager.NewTestDataManager(cfg)
	// 启用 mock 模式，确保测试的确定性
	manager.EnableMock(true)
	return manager
}

// TestManager_GetStockData_WithCacheDisabled_RefreshesData 测试在禁用缓存时，获取数据会强制刷新
func TestTestDataManager_GetStockData_WithCacheDisabled_ForcesProviderRefresh(t *testing.T) {
	manager := setupTestManager(t)
	defer manager.Close()

	symbols := []string{"600000"}
	mockData := []core.StockData{{Symbol: "600000", Price: 123.45}}
	manager.SetMockData(symbols, mockData)

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

// TestManager_GetStockData_WithEmptySymbols_ReturnsEmpty 测试使用空股票列表调用时，返回空结果
func TestTestDataManager_GetStockData_WithEmptySymbolList_ReturnsEmptySlice(t *testing.T) {
	manager := setupTestManager(t)
	defer manager.Close()

	results, err := manager.GetStockData(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Empty(t, results)
}
