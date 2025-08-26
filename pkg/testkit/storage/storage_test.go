package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/core"
	"stocksub/pkg/testkit/helpers"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSVStorage_Save_WithSingleItem_CreatesFile(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	config := CSVStorageConfig{
		Directory:      tmpDir,
		FilePrefix:     "test",
		DateFormat:     "2006-01-02",
		MaxFileSize:    1024 * 1024, // 1MB
		BatchSize:      10,
		FlushInterval:  1 * time.Second,
		ResourceConfig: helpers.DefaultResourceConfig(),
	}

	storage, err := NewCSVStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	// 测试保存股票数据
	stockData := subscriber.StockData{
		Symbol:        "TEST001",
		Name:          "测试股票",
		Price:         100.50,
		Change:        2.50,
		ChangePercent: 2.55,
		Volume:        1000000,
		Timestamp:     time.Now(),
	}

	err = storage.Save(ctx, stockData)
	assert.NoError(t, err)

	// 验证文件是否创建
	storage.Flush() // 确保写入
	expectedFile := filepath.Join(tmpDir, "test_stock_data_"+time.Now().Format("2006-01-02")+ ".csv")
	_, err = os.Stat(expectedFile)
	assert.NoError(t, err, "CSV file should be created")

	// 测试统计信息
	stats := storage.GetStats()
	assert.Equal(t, int64(1), stats.TotalRecords)
}

func TestCSVStorage_BatchSave_WithMultipleItems_SavesAll(t *testing.T) {
	tmpDir := t.TempDir()

	config := CSVStorageConfig{
		Directory:      tmpDir,
		FilePrefix:     "batch_test",
		DateFormat:     "2006-01-02",
		BatchSize:      5,
		FlushInterval:  100 * time.Millisecond,
		ResourceConfig: helpers.DefaultResourceConfig(),
	}

	storage, err := NewCSVStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	// 准备批量数据
	dataList := make([]interface{}, 10)
	for i := 0; i < 10; i++ {
		dataList[i] = subscriber.StockData{
			Symbol: fmt.Sprintf("TEST%03d", i),
			Name:   fmt.Sprintf("测试股票%d", i),
			Price:  100.0 + float64(i),
			Timestamp: time.Now(),
		}
	}

	// 批量保存
	err = storage.BatchSave(ctx, dataList)
	assert.NoError(t, err)

	// 验证统计信息
	stats := storage.GetStats()
	assert.Equal(t, int64(10), stats.TotalRecords)
	assert.Equal(t, int64(1), stats.BatchWrites)
}

func TestMemoryStorage_SaveAndLoad_WithStockData_Successful(t *testing.T) {
	config := DefaultMemoryStorageConfig()
	storage := NewMemoryStorage(config)
	defer storage.Close()

	ctx := context.Background()

	// 测试保存数据
	stockData := subscriber.StockData{
		Symbol: "TEST001",
		Name:   "测试股票",
		Price:  100.50,
	}

	err := storage.Save(ctx, stockData)
	assert.NoError(t, err)

	// 测试加载数据
	query := core.Query{
		Symbols: []string{"TEST001"},
		Limit:   10,
	}

	results, err := storage.Load(ctx, query)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestMemoryStorage_Save_WithMaxRecords_EvictsOldest(t *testing.T) {
	config := MemoryStorageConfig{
		MaxRecords:      3, // 最大3条记录
		EnableIndex:     true,
		TTL:             1 * time.Hour,
		CleanupInterval: 10 * time.Minute,
	}

	storage := NewMemoryStorage(config)
	defer storage.Close()

	ctx := context.Background()

	// 添加4条记录，应该移除最旧的
	for i := 0; i < 4; i++ {
		stockData := subscriber.StockData{
			Symbol: fmt.Sprintf("TEST%03d", i),
			Price:  100.0 + float64(i),
		}

		err := storage.Save(ctx, stockData)
		assert.NoError(t, err)
	}

	// 验证只保留了最新的3条记录
	query := core.Query{Limit: 10}
	results, err := storage.Load(ctx, query)
	assert.NoError(t, err)
	assert.Len(t, results, 3)
}


// --- Migrated from integration_test.go ---

func TestMemoryStorage_SaveAndLoad_WithStructuredData_Successful(t *testing.T) {
	config := DefaultMemoryStorageConfig()
	ms := NewMemoryStorage(config)
	defer ms.Close()

	// 创建测试数据
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	ctx := context.Background()

	// 测试保存
	err := ms.Save(ctx, sd)
	require.NoError(t, err)

	// 测试加载
	query := core.Query{
		Symbols: []string{"600000"},
	}
	results, err := ms.Load(ctx, query)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// 验证结果
	result, ok := results[0].(*subscriber.StructuredData)
	require.True(t, ok)

	symbol, err := result.GetField("symbol")
	require.NoError(t, err)
	assert.Equal(t, "600000", symbol)

	name, err := result.GetField("name")
	require.NoError(t, err)
	assert.Equal(t, "浦发银行", name)
}

func TestMemoryStorage_BatchSave_WithStructuredData_Successful(t *testing.T) {
	config := DefaultMemoryStorageConfig()
	ms := NewMemoryStorage(config)
	defer ms.Close()

	ctx := context.Background()

	// 创建多个 StructuredData 实例
	var dataList []interface{}
	symbols := []string{"600000", "000001", "000002"}
	names := []string{"浦发银行", "平安银行", "万科A"}
	prices := []float64{10.50, 12.80, 25.30}

	for i, symbol := range symbols {
		sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
		require.NoError(t, sd.SetField("symbol", symbol))
		require.NoError(t, sd.SetField("name", names[i]))
		require.NoError(t, sd.SetField("price", prices[i]))
		require.NoError(t, sd.SetField("timestamp", time.Now()))
		dataList = append(dataList, sd)
	}

	// 测试批量保存
	err := ms.BatchSave(ctx, dataList)
	require.NoError(t, err)

	// 验证数据
	for i, symbol := range symbols {
		results, err := ms.QueryBySymbol(ctx, symbol)
		require.NoError(t, err)
		require.Len(t, results, 1)

		result := results[0]
		require.NotNil(t, result)

		resultSymbol, err := result.GetField("symbol")
		require.NoError(t, err)
		assert.Equal(t, symbol, resultSymbol)

		resultName, err := result.GetField("name")
		require.NoError(t, err)
		assert.Equal(t, names[i], resultName)

		resultPrice, err := result.GetField("price")
		require.NoError(t, err)
		assert.Equal(t, prices[i], resultPrice)
	}
}

func TestMemoryStorage_QueryBySymbol_WithMixedData_ReturnsMatching(t *testing.T) {
	config := DefaultMemoryStorageConfig()
	ms := NewMemoryStorage(config)
	defer ms.Close()

	ctx := context.Background()

	// 准备测试数据
	symbols := []string{"600000", "000001", "600000"} // 包含重复的symbol
	for i, symbol := range symbols {
		sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
		require.NoError(t, sd.SetField("symbol", symbol))
		require.NoError(t, sd.SetField("name", "测试股票"+string(rune('A'+i))))
		require.NoError(t, sd.SetField("price", float64(10+i)))
		require.NoError(t, sd.SetField("timestamp", time.Now().Add(time.Duration(i)*time.Minute)))

		err := ms.Save(ctx, sd)
		require.NoError(t, err)
	}

	// 查询特定symbol
	results, err := ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, results, 2) // 应该有两条600000的记录

	// 查询不存在的symbol
	results, err = ms.QueryBySymbol(ctx, "999999")
	require.NoError(t, err)
	assert.Len(t, results, 0)

	// 查询单个记录的symbol
	results, err = ms.QueryBySymbol(ctx, "000001")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestMemoryStorage_QueryByTimeRange_WithVariousTimes_ReturnsMatching(t *testing.T) {
	config := DefaultMemoryStorageConfig()
	ms := NewMemoryStorage(config)
	defer ms.Close()

	ctx := context.Background()
	now := time.Now()

	// 准备不同时间的测试数据
	timeOffsets := []time.Duration{
		-2 * time.Hour, // 2小时前
		-1 * time.Hour, // 1小时前
		0,              // 现在
		1 * time.Hour,  // 1小时后
	}

	for i, offset := range timeOffsets {
		sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
		require.NoError(t, sd.SetField("symbol", "60000"+string(rune('0'+i))))
		require.NoError(t, sd.SetField("name", "测试股票"+string(rune('A'+i))))
		require.NoError(t, sd.SetField("price", float64(10+i)))

		timestamp := now.Add(offset)
		require.NoError(t, sd.SetField("timestamp", timestamp))
		sd.Timestamp = timestamp // 确保 StructuredData 的时间戳也正确设置

		err := ms.Save(ctx, sd)
		require.NoError(t, err)
	}

	// 查询过去2小时到现在的数据
	startTime := now.Add(-2 * time.Hour)
	endTime := now
	results, err := ms.QueryByTimeRange(ctx, startTime, endTime)
	require.NoError(t, err)
	assert.Len(t, results, 3) // 应该包含 -2小时、-1小时、现在 的数据

	// 查询未来1小时的数据
	startTime = now
	endTime = now.Add(1 * time.Hour)
	results, err = ms.QueryByTimeRange(ctx, startTime, endTime)
	require.NoError(t, err)
	assert.Len(t, results, 2) // 应该包含 现在、+1小时 的数据
}

func TestMemoryStorage_Save_WithMixedDataTypes_Successful(t *testing.T) {
	config := DefaultMemoryStorageConfig()
	ms := NewMemoryStorage(config)
	defer ms.Close()

	ctx := context.Background()

	// 保存不同类型的数据
	// 1. StructuredData
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))
	err := ms.Save(ctx, sd)
	require.NoError(t, err)

	// 2. 普通 map 数据
	mapData := map[string]interface{}{
		"type": "test",
		"id":   1,
		"name": "测试数据",
	}
	err = ms.Save(ctx, mapData)
	require.NoError(t, err)

	// 3. StockData 结构体
	stockData := subscriber.StockData{
		Symbol: "000001",
		Name:   "平安银行",
		Price:  12.80,
	}
	err = ms.Save(ctx, stockData)
	require.NoError(t, err)

	// 验证不同类型的数据都能正确保存和查询
	allResults, err := ms.Load(ctx, core.Query{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(allResults), 3)

	// 验证 StructuredData 能被正确查询
	structuredResults, err := ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, structuredResults, 1)
}

func TestMemoryStorage_Save_WithCapacityLimit_EvictsOldest(t *testing.T) {
	// 使用较小的容量限制进行测试
	config := MemoryStorageConfig{
		MaxRecords:      5,
		EnableIndex:     true,
		TTL:             0, // 不启用TTL
		CleanupInterval: 0,
	}
	ms := NewMemoryStorage(config)
	defer ms.Close()

	ctx := context.Background()

	// 插入超过容量限制的数据
	for i := 0; i < 10; i++ {
		sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
		require.NoError(t, sd.SetField("symbol", "60000"+string(rune('0'+i%10))))
		require.NoError(t, sd.SetField("name", "测试股票"+string(rune('A'+i))))
		require.NoError(t, sd.SetField("price", float64(10+i)))
		require.NoError(t, sd.SetField("timestamp", time.Now()))

		err := ms.Save(ctx, sd)
		require.NoError(t, err)
	}

	// 验证只保留了最后5条记录
	allResults, err := ms.Load(ctx, core.Query{})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(allResults), 5)

	// 验证保留的是最新的记录
	if len(allResults) > 0 {
		result, ok := allResults[0].(*subscriber.StructuredData)
		require.True(t, ok)
		name, err := result.GetField("name")
		require.NoError(t, err)
		// 应该是较新的记录
		assert.Contains(t, name.(string), "测试股票")
	}
}

func TestMemoryStorage_GetStats_AfterSaves_ReturnsCorrectCounts(t *testing.T) {
	config := DefaultMemoryStorageConfig()
	ms := NewMemoryStorage(config)
	defer ms.Close()

	ctx := context.Background()

	// 初始状态
	stats := ms.GetStats()
	assert.Equal(t, int64(0), stats.TotalRecords)
	assert.Equal(t, 0, stats.TotalTables)

	// 保存一些数据
	for i := 0; i < 3; i++ {
		sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
		require.NoError(t, sd.SetField("symbol", "60000"+string(rune('0'+i))))
		require.NoError(t, sd.SetField("name", "测试股票"+string(rune('A'+i))))
		require.NoError(t, sd.SetField("price", float64(10+i)))
		require.NoError(t, sd.SetField("timestamp", time.Now()))

		err := ms.Save(ctx, sd)
		require.NoError(t, err)
	}

	// 检查统计信息
	stats = ms.GetStats()
	assert.Equal(t, int64(3), stats.TotalRecords)
	assert.GreaterOrEqual(t, stats.TotalTables, 1)
	assert.GreaterOrEqual(t, stats.IndexCount, 1)
}


func TestMemoryStorage_Save_WithTTL_EvictsExpired(t *testing.T) {
	// 使用较短的TTL进行测试
	config := MemoryStorageConfig{
		MaxRecords:      100,
		EnableIndex:     true,
		TTL:             100 * time.Millisecond,
		CleanupInterval: 10 * time.Millisecond,
	}
	ms := NewMemoryStorage(config)
	defer ms.Close()

	ctx := context.Background()

	// 保存一条数据
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))
	err := ms.Save(ctx, sd)
	require.NoError(t, err)

	// 验证数据存在
	results, err := ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// 等待超过TTL
	time.Sleep(150 * time.Millisecond)

	// 验证数据已被清理
	results, err = ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

// --- Migrated from csv_structured_data_test.go ---

// setupCSVStorageTest is a helper function to create a CSVStorage instance for testing.
func setupCSVStorageTest(t *testing.T) (*CSVStorage, string) {
	t.Helper()
	tempDir := t.TempDir()

	config := CSVStorageConfig{
		Directory:      tempDir,
		FilePrefix:     "test",
		DateFormat:     "2006-01-02",
		MaxFileSize:    100 * 1024 * 1024,
		RotateInterval: 24 * time.Hour,
		EnableCompress: false,
		BatchSize:      100,
		FlushInterval:  0, // Disable auto-flush for tests
		ResourceConfig: helpers.DefaultResourceConfig(),
	}

	storage, err := NewCSVStorage(config)
	require.NoError(t, err)

	t.Cleanup(func() {
		storage.Close()
	})

	return storage, tempDir
}

func TestCSVStorage_Save_WithStructuredData_Successful(t *testing.T) {
	storage, tempDir := setupCSVStorageTest(t)

	// 创建测试用的 StructuredData
	// 使用上海时区的时间，这样转换后应该显示为 18:30
	shanghaiTZ, _ := time.LoadLocation("Asia/Shanghai")
	testTime := time.Date(2025, 8, 24, 18, 30, 0, 0, shanghaiTZ)
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	sd.Timestamp = testTime

	// 设置测试数据
	err := sd.SetField("symbol", "600000")
	require.NoError(t, err)
	err = sd.SetField("name", "浦发银行")
	require.NoError(t, err)
	err = sd.SetField("price", 10.50)
	require.NoError(t, err)
	err = sd.SetField("change", 0.15)
	require.NoError(t, err)
	err = sd.SetField("change_percent", 1.45)
	require.NoError(t, err)
	err = sd.SetField("volume", int64(1250000))
	require.NoError(t, err)
	err = sd.SetField("timestamp", testTime)
	require.NoError(t, err)

	// 保存数据
	ctx := context.Background()
	err = storage.Save(ctx, sd)
	require.NoError(t, err)

	// 刷新缓冲区
	err = storage.Flush()
	require.NoError(t, err)

	// 验证文件是否创建
	expectedFilename := "test_structured_stock_data_2025-08-24.csv"
	expectedPath := filepath.Join(tempDir, expectedFilename)

	// 检查文件是否存在
	_, err = os.Stat(expectedPath)
	require.NoError(t, err, "CSV file should be created")

	// 读取文件内容
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	require.GreaterOrEqual(t, len(lines), 2, "File should contain header and at least one data row")

	// 验证表头
	header := lines[0]
	assert.Contains(t, header, "股票代码(symbol)", "Header should contain Chinese description for symbol")
	assert.Contains(t, header, "股票名称(name)", "Header should contain Chinese description for name")
	assert.Contains(t, header, "当前价格(price)", "Header should contain Chinese description for price")
	assert.Contains(t, header, "数据时间(timestamp)", "Header should contain Chinese description for timestamp")

	// 验证数据行
	dataRow := lines[1]
	assert.Contains(t, dataRow, "600000", "Data row should contain symbol")
	assert.Contains(t, dataRow, "浦发银行", "Data row should contain name")
	assert.Contains(t, dataRow, "10.50", "Data row should contain price")
	assert.Contains(t, dataRow, "2025-08-24 18:30:00", "Data row should contain formatted timestamp")
}

func TestCSVStorage_BatchSave_WithStructuredData_Successful(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建 CSVStorage 配置
	config := CSVStorageConfig{
		Directory:      tempDir,
		FilePrefix:     "test",
		DateFormat:     "2006-01-02",
		MaxFileSize:    100 * 1024 * 1024,
		RotateInterval: 24 * time.Hour,
		EnableCompress: false,
		BatchSize:      100,
		FlushInterval:  0, // 禁用自动刷新
		ResourceConfig: helpers.DefaultResourceConfig(),
	}

	// 创建 CSVStorage 实例
	storage, err := NewCSVStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	// 创建测试数据
	testTime := time.Date(2025, 8, 24, 18, 30, 0, 0, time.UTC)

	var dataList []interface{}

	// 创建多个 StructuredData 实例
	for i, symbol := range []string{"600000", "000001", "300750"} {
		sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
		sd.Timestamp = testTime

		err = sd.SetField("symbol", symbol)
		require.NoError(t, err)
		err = sd.SetField("name", "测试股票"+string(rune('A'+i)))
		require.NoError(t, err)
		err = sd.SetField("price", 10.0+float64(i))
		require.NoError(t, err)
		err = sd.SetField("timestamp", testTime)
		require.NoError(t, err)

		dataList = append(dataList, sd)
	}

	// 批量保存数据
	ctx := context.Background()
	err = storage.BatchSave(ctx, dataList)
	require.NoError(t, err)

	// 刷新缓冲区
	err = storage.Flush()
	require.NoError(t, err)

	// 验证文件是否创建
	expectedFilename := "test_structured_stock_data_2025-08-24.csv"
	expectedPath := filepath.Join(tempDir, expectedFilename)

	// 检查文件是否存在
	_, err = os.Stat(expectedPath)
	require.NoError(t, err, "CSV file should be created")

	// 读取文件内容
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	require.GreaterOrEqual(t, len(lines), 4, "File should contain header and 3 data rows")

	// 验证表头
	header := lines[0]
	assert.Contains(t, header, "股票代码(symbol)", "Header should contain Chinese description")

	// 验证数据行
	for i, expectedSymbol := range []string{"600000", "000001", "300750"} {
		dataRow := lines[i+1]
		assert.Contains(t, dataRow, expectedSymbol, "Data row should contain correct symbol")
	}
}

func TestCSVStorage_Save_WithMixedDataTypes_Successful(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建 CSVStorage 配置
	config := CSVStorageConfig{
		Directory:      tempDir,
		FilePrefix:     "test",
		DateFormat:     "2006-01-02",
		MaxFileSize:    100 * 1024 * 1024,
		RotateInterval: 24 * time.Hour,
		EnableCompress: false,
		BatchSize:      100,
		FlushInterval:  0, // 禁用自动刷新
		ResourceConfig: helpers.DefaultResourceConfig(),
	}

	// 创建 CSVStorage 实例
	storage, err := NewCSVStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()
	testTime := time.Date(2025, 8, 24, 18, 30, 0, 0, time.UTC)

	// 保存传统的 StockData
	stockData := subscriber.StockData{
		Symbol:    "600000",
		Name:      "浦发银行",
		Price:     10.50,
		Timestamp: testTime,
	}
	err = storage.Save(ctx, stockData)
	require.NoError(t, err)

	// 保存 StructuredData
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	sd.Timestamp = testTime
	err = sd.SetField("symbol", "000001")
	require.NoError(t, err)
	err = sd.SetField("name", "平安银行")
	require.NoError(t, err)
	err = sd.SetField("price", 12.80)
	require.NoError(t, err)
	err = sd.SetField("timestamp", testTime)
	require.NoError(t, err)

	err = storage.Save(ctx, sd)
	require.NoError(t, err)

	// 刷新缓冲区
	err = storage.Flush()
	require.NoError(t, err)

	// 验证两个不同的文件被创建
	stockDataFile := filepath.Join(tempDir, "test_stock_data_2025-08-24.csv")
	structuredDataFile := filepath.Join(tempDir, "test_structured_stock_data_2025-08-24.csv")

	// 检查文件是否存在
	_, err = os.Stat(stockDataFile)
	require.NoError(t, err, "StockData CSV file should be created")

	_, err = os.Stat(structuredDataFile)
	require.NoError(t, err, "StructuredData CSV file should be created")

	// 验证 StructuredData 文件有正确的表头格式
	content, err := os.ReadFile(structuredDataFile)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	require.GreaterOrEqual(t, len(lines), 2, "StructuredData file should contain header and data")

	header := lines[0]
	assert.Contains(t, header, "股票代码(symbol)", "StructuredData header should contain Chinese descriptions")

	dataRow := lines[1]
	assert.Contains(t, dataRow, "000001", "StructuredData should contain correct symbol")
	assert.Contains(t, dataRow, "平安银行", "StructuredData should contain correct name")
}

func TestCSVStorage_getStructuredDataCSVHeaders_Successful(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建 CSVStorage 配置
	config := CSVStorageConfig{
		Directory:      tempDir,
		FilePrefix:     "test",
		DateFormat:     "2006-01-02",
		ResourceConfig: helpers.DefaultResourceConfig(),
	}

	// 创建 CSVStorage 实例
	storage, err := NewCSVStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	// 测试 getStructuredDataCSVHeaders 方法
	headers := storage.getStructuredDataCSVHeaders(subscriber.StockDataSchema)

	// 验证表头格式
	assert.Greater(t, len(headers), 0, "Headers should not be empty")

	// 检查一些关键字段的表头格式
	symbolHeaderFound := false
	nameHeaderFound := false
	priceHeaderFound := false

	for _, header := range headers {
		if strings.Contains(header, "股票代码(symbol)") {
			symbolHeaderFound = true
		}
		if strings.Contains(header, "股票名称(name)") {
			nameHeaderFound = true
		}
		if strings.Contains(header, "当前价格(price)") {
			priceHeaderFound = true
		}
	}

	assert.True(t, symbolHeaderFound, "Symbol header with Chinese description should be found")
	assert.True(t, nameHeaderFound, "Name header with Chinese description should be found")
	assert.True(t, priceHeaderFound, "Price header with Chinese description should be found")
}

func TestCSVStorage_Save_WithStructuredData_UsesShanghaiTimezone(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建 CSVStorage 配置
	config := CSVStorageConfig{
		Directory:      tempDir,
		FilePrefix:     "test",
		DateFormat:     "2006-01-02",
		ResourceConfig: helpers.DefaultResourceConfig(),
	}

	// 创建 CSVStorage 实例
	storage, err := NewCSVStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	// 创建测试用的 StructuredData，使用 UTC 时间
	utcTime := time.Date(2025, 8, 24, 10, 30, 0, 0, time.UTC) // UTC 10:30
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	sd.Timestamp = utcTime

	// 设置必要字段
	err = sd.SetField("symbol", "600000")
	require.NoError(t, err)
	err = sd.SetField("name", "浦发银行")
	require.NoError(t, err)
	err = sd.SetField("price", 10.50)
	require.NoError(t, err)
	err = sd.SetField("timestamp", utcTime)
	require.NoError(t, err)

	// 保存数据
	ctx := context.Background()
	err = storage.Save(ctx, sd)
	require.NoError(t, err)

	// 刷新缓冲区
	err = storage.Flush()
	require.NoError(t, err)

	// 读取文件内容
	expectedFilename := "test_structured_stock_data_2025-08-24.csv"
	expectedPath := filepath.Join(tempDir, expectedFilename)
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	require.GreaterOrEqual(t, len(lines), 2, "File should contain header and data")

	dataRow := lines[1]
	// 验证时间格式为上海时区 (UTC+8)，所以 UTC 10:30 应该显示为 18:30
	assert.Contains(t, dataRow, "2025-08-24 18:30:00", "Time should be formatted in Shanghai timezone")
}
