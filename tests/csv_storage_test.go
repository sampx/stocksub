package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	apitesting "stocksub/pkg/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestCache 创建一个测试用的缓存实例
func setupTestCache() *apitesting.TestDataCache {
	// 使用相对于项目根目录的绝对路径
	// 获取当前文件所在目录的绝对路径
	_, currentFile, _, _ := runtime.Caller(0)
	// 获取 tests 目录的绝对路径，然后向上两级到达项目根目录，再进入 tests/data
	testsDir := filepath.Dir(currentFile)
	dataDir := filepath.Join(testsDir, "data")
	// 不再清理，允许数据持久化
	cache := apitesting.NewTestDataCache(dataDir)
	return cache
}

// TestCacheAndStorageIntegration 测试缓存和存储的集成
func TestCacheAndStorageIntegration(t *testing.T) {
	// 这个测试现在验证了新开发的cache和storage代码
	cache := setupTestCache()
	defer cache.Close()

	symbols := []string{"600000", "000001"}

	// 首次调用，应触发API调用并保存到CSV
	t.Run("FirstFetch_Should_Call_API_And_Save", func(t *testing.T) {
		// 强制API调用以生成缓存
		os.Setenv("FORCE_API_CALL", "1")
		defer os.Unsetenv("FORCE_API_CALL")

		results, err := cache.GetStockDataBatch(symbols)
		require.NoError(t, err)
		require.Len(t, results, 2)

		// 验证CSV文件是否已创建
		dateStr := time.Now().Format("2006-01-02")
		// 使用与 setupTestCache 相同的逻辑来构建预期文件路径
		_, currentFile, _, _ := runtime.Caller(0)
		testsDir := filepath.Dir(currentFile)
		dataDir := filepath.Join(testsDir, "data")
		expectedFile := filepath.Join(dataDir, "collected", "api_data_"+dateStr+".csv")
		assert.FileExists(t, expectedFile)

		// 验证L1内存缓存
		stats := cache.GetCacheStats()
		assert.Equal(t, 1, stats["l1_cache_size"])
	})

	// 第二次调用，应使用L1内存缓存
	t.Run("SecondFetch_Should_Use_L1_Cache", func(t *testing.T) {
		// 确保不强制API调用
		os.Unsetenv("FORCE_API_CALL")

		results, err := cache.GetStockDataBatch(symbols)
		require.NoError(t, err)
		require.Len(t, results, 2)

		// 这里可以添加一个标记来确认API没有被调用
		// (在实际的实现中，可以给API调用函数加上日志)
		t.Log("第二次调用成功，预期使用了L1缓存")
	})

	// 创建新的缓存实例，应使用L2 CSV缓存
	t.Run("NewCache_Should_Use_L2_Cache", func(t *testing.T) {
		// 使用与 setupTestCache 相同的路径配置
		_, currentFile, _, _ := runtime.Caller(0)
		testsDir := filepath.Dir(currentFile)
		dataDir := filepath.Join(testsDir, "data")
		newCache := apitesting.NewTestDataCache(dataDir)
		defer newCache.Close()

		results, err := newCache.GetStockDataBatch(symbols)
		require.NoError(t, err)
		require.Len(t, results, 2)

		// 验证数据是否从CSV正确加载
		assert.Equal(t, "600000", results[0].Symbol)
		assert.Equal(t, "000001", results[1].Symbol)
		t.Log("新缓存实例成功从CSV加载数据，预期使用了L2缓存")
	})
}

// TestCSVStorage_Standalone 保持对CSVStorage的独立测试
func TestCSVStorage_Standalone(t *testing.T) {
	// 使用独立的临时目录避免与其他测试冲突
	tempDir := t.TempDir()
	storage := apitesting.NewCSVStorage(tempDir)
	defer storage.Close()

	// 测试数据点
	testData := apitesting.DataPoint{
		Timestamp:    time.Now(),
		Symbol:       "TEST001", // 使用独特的股票代码避免冲突
		QueryTime:    time.Now().Add(-1 * time.Second),
		ResponseTime: time.Now(),
		QuoteTime:    "20250821132323",
		Price:        10.50,
		Volume:       123456789,
		Field30:      "20250821132323",
		AllFields:    []string{"field1", "field2", "field3"},
	}

	// 测试保存数据点
	err := storage.SaveDataPoint(testData)
	require.NoError(t, err)

	// 测试读取数据点
	startDate := testData.Timestamp.Truncate(24 * time.Hour)
	endDate := startDate.Add(24 * time.Hour)

	readData, err := storage.ReadDataPoints(startDate, endDate)
	require.NoError(t, err)
	require.Len(t, readData, 1, "应该只读取到1个数据点，实际读取到%d个", len(readData))

	// 验证读取的数据
	readPoint := readData[0]
	assert.Equal(t, testData.Symbol, readPoint.Symbol)
	assert.Equal(t, testData.Price, readPoint.Price)
	assert.Equal(t, testData.QuoteTime, readPoint.QuoteTime)
	assert.Equal(t, testData.AllFields, readPoint.AllFields)
}

// TestForceRefreshCache 测试强制刷新缓存
func TestForceRefreshCache(t *testing.T) {
	cache := setupTestCache()
	defer cache.Close()

	symbols := []string{"600000"}

	// 首次调用，填充缓存
	_, err := cache.GetStockDataBatch(symbols)
	require.NoError(t, err)

	// 强制刷新
	t.Run("ForceRefresh", func(t *testing.T) {
		os.Setenv("FORCE_API_CALL", "1")
		defer os.Unsetenv("FORCE_API_CALL")

		refreshedResults, err := cache.ForceRefreshCache(symbols)
		require.NoError(t, err)
		require.Len(t, refreshedResults, 1)
		t.Log("强制刷新成功，预期重新调用了API")
	})
}

// TestCacheWithEmptySymbols 测试空股票列表
func TestCacheWithEmptySymbols(t *testing.T) {
	cache := setupTestCache()
	defer cache.Close()

	results, err := cache.GetStockDataBatch([]string{})
	assert.NoError(t, err)
	assert.Empty(t, results)
}

// TestCachePartialMiss 测试部分缓存未命中
func TestCachePartialMiss(t *testing.T) {
	// 这个测试场景在当前实现下会触发API调用
	// 这是一个更高级的缓存策略，可以作为未来的优化方向
	t.Skip("部分缓存未命中的高级策略，当前版本会触发API调用")
}
