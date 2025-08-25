package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/core"
	"stocksub/pkg/testkit/helpers"
	"stocksub/pkg/testkit/storage"
)

// TestCompatibilityWithExistingStorage 验证新结构体与现有 pkg/testkit/storage 模块的完全兼容性
func TestCompatibilityWithExistingStorage(t *testing.T) {
	t.Run("MemoryStorage兼容性测试", func(t *testing.T) {
		testMemoryStorageCompatibility(t)
	})

	t.Run("CSVStorage兼容性测试", func(t *testing.T) {
		testCSVStorageCompatibility(t)
	})

	t.Run("BatchWriter兼容性测试", func(t *testing.T) {
		testBatchWriterCompatibility(t)
	})
}

func testMemoryStorageCompatibility(t *testing.T) {
	// 创建内存存储
	config := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(config)
	defer ms.Close()

	ctx := context.Background()

	// 1. 验证 StructuredData 实现了 core.Storage 接口
	var _ core.Storage = ms

	// 2. 测试基本存储操作
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	// Save 操作
	err := ms.Save(ctx, sd)
	require.NoError(t, err)

	// Load 操作
	query := core.Query{
		Symbols: []string{"600000"},
	}
	results, err := ms.Load(ctx, query)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// 验证返回的数据类型
	loadedSD, ok := results[0].(*subscriber.StructuredData)
	require.True(t, ok, "返回的数据应该是 *StructuredData 类型")

	// 验证数据内容
	symbol, err := loadedSD.GetField("symbol")
	require.NoError(t, err)
	assert.Equal(t, "600000", symbol)

	// BatchSave 操作
	var batchData []interface{}
	for i := 0; i < 5; i++ {
		batchSD := subscriber.NewStructuredData(subscriber.StockDataSchema)
		require.NoError(t, batchSD.SetField("symbol", "60000"+string(rune('1'+i))))
		require.NoError(t, batchSD.SetField("name", "测试股票"+string(rune('A'+i))))
		require.NoError(t, batchSD.SetField("price", float64(10+i)))
		require.NoError(t, batchSD.SetField("timestamp", time.Now()))
		batchData = append(batchData, batchSD)
	}

	err = ms.BatchSave(ctx, batchData)
	require.NoError(t, err)

	// 验证批量数据
	allResults, err := ms.Load(ctx, core.Query{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(allResults), 5, "应该至少有5条批量保存的数据")

	// Delete 操作
	deleteQuery := core.Query{
		Symbols: []string{"600001"},
	}
	err = ms.Delete(ctx, deleteQuery)
	require.NoError(t, err)

	// 验证删除效果
	deletedResults, err := ms.Load(ctx, deleteQuery)
	require.NoError(t, err)
	assert.Len(t, deletedResults, 0, "删除后应该没有匹配的记录")
}

func testCSVStorageCompatibility(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建CSV存储
	config := storage.CSVStorageConfig{
		Directory:      tempDir,
		FilePrefix:     "compat_test",
		DateFormat:     "2006-01-02",
		MaxFileSize:    100 * 1024 * 1024,
		RotateInterval: 24 * time.Hour,
		EnableCompress: false,
		BatchSize:      100,
		FlushInterval:  0,
		ResourceConfig: helpers.DefaultResourceConfig(),
	}

	csvStorage, err := storage.NewCSVStorage(config)
	require.NoError(t, err)
	defer csvStorage.Close()

	// 验证实现了 core.Storage 接口
	var _ core.Storage = csvStorage

	ctx := context.Background()

	// 测试 StructuredData 保存
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	err = csvStorage.Save(ctx, sd)
	require.NoError(t, err)

	// 刷新确保写入
	err = csvStorage.Flush()
	require.NoError(t, err)

	// 测试批量保存
	var batchData []interface{}
	for i := 0; i < 3; i++ {
		batchSD := subscriber.NewStructuredData(subscriber.StockDataSchema)
		require.NoError(t, batchSD.SetField("symbol", "00000"+string(rune('1'+i))))
		require.NoError(t, batchSD.SetField("name", "测试股票"+string(rune('A'+i))))
		require.NoError(t, batchSD.SetField("price", float64(12+i)))
		require.NoError(t, batchSD.SetField("timestamp", time.Now()))
		batchData = append(batchData, batchSD)
	}

	err = csvStorage.BatchSave(ctx, batchData)
	require.NoError(t, err)

	err = csvStorage.Flush()
	require.NoError(t, err)

	// 验证统计信息
	stats := csvStorage.GetStats()
	assert.GreaterOrEqual(t, stats.TotalRecords, int64(4), "应该至少有4条记录")
}

func testBatchWriterCompatibility(t *testing.T) {
	// 创建内存存储
	memConfig := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(memConfig)
	defer ms.Close()

	// 创建批量写入器
	batchConfig := storage.DefaultBatchWriterConfig()
	bw := storage.NewBatchWriter(ms, batchConfig)
	defer bw.Close()

	ctx := context.Background()

	// 测试 StructuredData 写入
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	err := bw.Write(ctx, sd)
	require.NoError(t, err)

	err = bw.Flush()
	require.NoError(t, err)

	// 验证数据已保存
	results, err := ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// 验证统计信息
	stats := bw.GetStats()
	assert.GreaterOrEqual(t, stats.StructuredDataRecords, int64(1))
}

// TestMixedDataUsage 测试混合使用 StockData 和 StructuredData 的场景
func TestMixedDataUsage(t *testing.T) {
	t.Run("内存存储混合数据", func(t *testing.T) {
		testMemoryStorageMixedData(t)
	})

	t.Run("CSV存储混合数据", func(t *testing.T) {
		testCSVStorageMixedData(t)
	})

	t.Run("BatchWriter混合数据", func(t *testing.T) {
		testBatchWriterMixedData(t)
	})

	t.Run("数据转换测试", func(t *testing.T) {
		testDataConversion(t)
	})
}

func testMemoryStorageMixedData(t *testing.T) {
	// 创建内存存储
	config := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(config)
	defer ms.Close()

	ctx := context.Background()

	// 1. 保存传统的 StockData
	stockData := subscriber.StockData{
		Symbol:        "600000",
		Name:          "浦发银行",
		Price:         10.50,
		Change:        0.15,
		ChangePercent: 1.45,
		Volume:        1250000,
		Timestamp:     time.Now(),
	}

	err := ms.Save(ctx, stockData)
	require.NoError(t, err)

	// 2. 保存 StructuredData
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "000001"))
	require.NoError(t, sd.SetField("name", "平安银行"))
	require.NoError(t, sd.SetField("price", 12.80))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	err = ms.Save(ctx, sd)
	require.NoError(t, err)

	// 3. 保存 Map 数据
	mapData := map[string]interface{}{
		"type":      "test",
		"id":        1,
		"name":      "测试数据",
		"timestamp": time.Now(),
	}

	err = ms.Save(ctx, mapData)
	require.NoError(t, err)

	// 4. 验证所有数据都已保存
	allResults, err := ms.Load(ctx, core.Query{})
	require.NoError(t, err)
	assert.Len(t, allResults, 3, "应该有3条不同类型的数据")

	// 5. 验证数据类型分布
	var stockDataCount, structuredDataCount, mapDataCount int
	for _, result := range allResults {
		switch result.(type) {
		case subscriber.StockData:
			stockDataCount++
		case *subscriber.StructuredData:
			structuredDataCount++
		case map[string]interface{}:
			mapDataCount++
		}
	}

	assert.Equal(t, 1, stockDataCount, "应该有1条StockData")
	assert.Equal(t, 1, structuredDataCount, "应该有1条StructuredData")
	assert.Equal(t, 1, mapDataCount, "应该有1条Map数据")

	// 6. 测试 StructuredData 特有的查询
	structuredResults, err := ms.QueryBySymbol(ctx, "000001")
	require.NoError(t, err)
	assert.Len(t, structuredResults, 1, "应该查询到1条StructuredData")
}

func testCSVStorageMixedData(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建CSV存储
	config := storage.CSVStorageConfig{
		Directory:      tempDir,
		FilePrefix:     "mixed_test",
		DateFormat:     "2006-01-02",
		MaxFileSize:    100 * 1024 * 1024,
		RotateInterval: 24 * time.Hour,
		EnableCompress: false,
		BatchSize:      100,
		FlushInterval:  0,
		ResourceConfig: helpers.DefaultResourceConfig(),
	}

	csvStorage, err := storage.NewCSVStorage(config)
	require.NoError(t, err)
	defer csvStorage.Close()

	ctx := context.Background()

	// 1. 保存传统的 StockData
	stockData := subscriber.StockData{
		Symbol:    "600000",
		Name:      "浦发银行",
		Price:     10.50,
		Timestamp: time.Now(),
	}

	err = csvStorage.Save(ctx, stockData)
	require.NoError(t, err)

	// 2. 保存 StructuredData
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "000001"))
	require.NoError(t, sd.SetField("name", "平安银行"))
	require.NoError(t, sd.SetField("price", 12.80))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	err = csvStorage.Save(ctx, sd)
	require.NoError(t, err)

	// 3. 批量保存混合数据
	var mixedData []interface{}

	// 添加更多 StockData
	for i := 0; i < 2; i++ {
		stock := subscriber.StockData{
			Symbol:    "60000" + string(rune('1'+i)),
			Name:      "股票" + string(rune('A'+i)),
			Price:     float64(15 + i),
			Timestamp: time.Now(),
		}
		mixedData = append(mixedData, stock)
	}

	// 添加更多 StructuredData
	for i := 0; i < 2; i++ {
		structData := subscriber.NewStructuredData(subscriber.StockDataSchema)
		require.NoError(t, structData.SetField("symbol", "00000"+string(rune('2'+i))))
		require.NoError(t, structData.SetField("name", "结构化股票"+string(rune('A'+i))))
		require.NoError(t, structData.SetField("price", float64(20+i)))
		require.NoError(t, structData.SetField("timestamp", time.Now()))
		mixedData = append(mixedData, structData)
	}

	err = csvStorage.BatchSave(ctx, mixedData)
	require.NoError(t, err)

	err = csvStorage.Flush()
	require.NoError(t, err)

	// 验证统计信息
	stats := csvStorage.GetStats()
	assert.GreaterOrEqual(t, stats.TotalRecords, int64(6), "应该至少有6条记录")
}

func testBatchWriterMixedData(t *testing.T) {
	// 创建内存存储
	memConfig := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(memConfig)
	defer ms.Close()

	// 创建批量写入器
	batchConfig := storage.DefaultBatchWriterConfig()
	bw := storage.NewBatchWriter(ms, batchConfig)
	defer bw.Close()

	ctx := context.Background()

	// 1. 写入 StockData
	stockData := subscriber.StockData{
		Symbol:    "600000",
		Name:      "浦发银行",
		Price:     10.50,
		Timestamp: time.Now(),
	}

	err := bw.Write(ctx, stockData)
	require.NoError(t, err)

	// 2. 写入 StructuredData
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "000001"))
	require.NoError(t, sd.SetField("name", "平安银行"))
	require.NoError(t, sd.SetField("price", 12.80))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	err = bw.Write(ctx, sd)
	require.NoError(t, err)

	// 3. 写入 Map 数据
	mapData := map[string]interface{}{
		"type": "test",
		"id":   1,
		"name": "测试数据",
	}

	err = bw.Write(ctx, mapData)
	require.NoError(t, err)

	// 4. 刷新所有数据
	err = bw.Flush()
	require.NoError(t, err)

	// 5. 验证统计信息
	stats := bw.GetStats()
	assert.Equal(t, int64(3), stats.TotalRecords, "应该有3条总记录")
	assert.Equal(t, int64(1), stats.StructuredDataRecords, "应该有1条StructuredData记录")

	// 6. 验证数据查询
	allResults, err := ms.Load(ctx, core.Query{})
	require.NoError(t, err)
	assert.Len(t, allResults, 3, "应该查询到3条记录")

	// 7. 验证 StructuredData 专用查询
	structuredResults, err := ms.QueryBySymbol(ctx, "000001")
	require.NoError(t, err)
	assert.Len(t, structuredResults, 1, "应该查询到1条StructuredData")
}

func testDataConversion(t *testing.T) {
	// 1. 测试 StockData 到 StructuredData 的转换
	originalStock := subscriber.StockData{
		Symbol:        "600000",
		Name:          "浦发银行",
		Price:         10.50,
		Change:        0.15,
		ChangePercent: 1.45,
		Volume:        1250000,
		Timestamp:     time.Now(),
	}

	// 转换为 StructuredData
	convertedSD, err := subscriber.StockDataToStructuredData(originalStock)
	require.NoError(t, err)
	require.NotNil(t, convertedSD)

	// 验证转换结果
	symbol, err := convertedSD.GetField("symbol")
	require.NoError(t, err)
	assert.Equal(t, "600000", symbol)

	name, err := convertedSD.GetField("name")
	require.NoError(t, err)
	assert.Equal(t, "浦发银行", name)

	price, err := convertedSD.GetField("price")
	require.NoError(t, err)
	assert.Equal(t, 10.50, price)

	// 验证数据完整性
	err = convertedSD.ValidateData()
	require.NoError(t, err, "转换后的数据应该通过验证")

	// 2. 测试转换后的序列化
	serializer := subscriber.NewStructuredDataSerializer(subscriber.FormatCSV)
	csvData, err := serializer.Serialize(convertedSD)
	require.NoError(t, err)
	assert.NotEmpty(t, csvData, "序列化结果不应为空")

	// 3. 测试转换后的存储
	memStorage := storage.NewMemoryStorage(storage.DefaultMemoryStorageConfig())
	defer memStorage.Close()

	ctx := context.Background()
	err = memStorage.Save(ctx, convertedSD)
	require.NoError(t, err, "转换后的数据应该能够正常保存")

	// 查询验证
	results, err := memStorage.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, results, 1, "应该能够查询到转换后的数据")
}

// TestBackwardCompatibility 测试向后兼容性
func TestBackwardCompatibility(t *testing.T) {
	// 创建内存存储
	config := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(config)
	defer ms.Close()

	ctx := context.Background()

	// 1. 保存传统数据类型
	legacyData := map[string]interface{}{
		"id":   1,
		"name": "legacy_data",
		"type": "old_format",
	}

	err := ms.Save(ctx, legacyData)
	require.NoError(t, err, "应该能够保存传统数据格式")

	// 2. 保存新的 StructuredData
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "新格式股票"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	err = ms.Save(ctx, sd)
	require.NoError(t, err, "应该能够保存新的StructuredData格式")

	// 3. 查询所有数据
	allResults, err := ms.Load(ctx, core.Query{})
	require.NoError(t, err)
	assert.Len(t, allResults, 2, "应该能够查询到所有数据")

	// 4. 验证数据类型正确性
	var legacyCount, structuredCount int
	for _, result := range allResults {
		switch result.(type) {
		case map[string]interface{}:
			legacyCount++
		case *subscriber.StructuredData:
			structuredCount++
		}
	}

	assert.Equal(t, 1, legacyCount, "应该有1条传统数据")
	assert.Equal(t, 1, structuredCount, "应该有1条结构化数据")
}

// TestIntegrationWithExistingFeatures 测试与现有功能的集成
func TestIntegrationWithExistingFeatures(t *testing.T) {
	// 创建CSV存储
	tempDir := t.TempDir()
	config := storage.CSVStorageConfig{
		Directory:      tempDir,
		FilePrefix:     "integration_test",
		DateFormat:     "2006-01-02",
		MaxFileSize:    100 * 1024 * 1024,
		RotateInterval: 24 * time.Hour,
		EnableCompress: false,
		BatchSize:      10,
		FlushInterval:  100 * time.Millisecond,
		ResourceConfig: helpers.DefaultResourceConfig(),
	}

	csvStorage, err := storage.NewCSVStorage(config)
	require.NoError(t, err)
	defer csvStorage.Close()

	// 创建批量写入器
	batchConfig := storage.DefaultBatchWriterConfig()
	bw := storage.NewBatchWriter(csvStorage, batchConfig)
	defer bw.Close()

	ctx := context.Background()

	// 1. 测试所有功能组合使用
	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			// 写入 StockData
			stockData := subscriber.StockData{
				Symbol:    "60000" + string(rune('0'+i%10)),
				Name:      "传统股票" + string(rune('A'+i%10)),
				Price:     float64(10 + i),
				Timestamp: time.Now(),
			}
			err := bw.Write(ctx, stockData)
			require.NoError(t, err)
		} else {
			// 写入 StructuredData
			sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
			require.NoError(t, sd.SetField("symbol", "00000"+string(rune('0'+i%10))))
			require.NoError(t, sd.SetField("name", "结构化股票"+string(rune('A'+i%10))))
			require.NoError(t, sd.SetField("price", float64(20+i)))
			require.NoError(t, sd.SetField("timestamp", time.Now()))
			err := bw.Write(ctx, sd)
			require.NoError(t, err)
		}
	}

	// 2. 刷新所有数据
	err = bw.Flush()
	require.NoError(t, err)

	// 3. 验证统计信息
	batchStats := bw.GetStats()
	assert.Equal(t, int64(20), batchStats.TotalRecords, "应该有20条总记录")
	assert.Equal(t, int64(10), batchStats.StructuredDataRecords, "应该有10条StructuredData记录")

	csvStats := csvStorage.GetStats()
	assert.Equal(t, int64(20), csvStats.TotalRecords, "CSV存储应该有20条记录")
}
