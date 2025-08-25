package tests_test

import (
	"context"
	"testing"
	"time"

	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchWriter_StructuredData_Basic(t *testing.T) {
	// 创建内存存储
	memConfig := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(memConfig)
	defer ms.Close()

	// 创建启用 StructuredData 优化的 BatchWriter
	batchConfig := storage.OptimizedBatchWriterConfig()
	bw := storage.NewBatchWriter(ms, batchConfig)
	defer bw.Close()

	ctx := context.Background()

	// 创建测试数据
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	// 写入数据
	err := bw.Write(ctx, sd)
	require.NoError(t, err)

	// 手动刷新
	err = bw.Flush()
	require.NoError(t, err)

	// 验证数据已保存
	results, err := ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// 验证统计信息
	stats := bw.GetStats()
	assert.Equal(t, int64(1), stats.StructuredDataRecords)
	assert.GreaterOrEqual(t, stats.StructuredDataFlushes, int64(1))
}

func TestBatchWriter_StructuredData_Batching(t *testing.T) {
	// 创建内存存储
	memConfig := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(memConfig)
	defer ms.Close()

	// 创建小批次大小的 BatchWriter 配置
	batchConfig := storage.BatchWriterConfig{
		BatchSize:                 10,
		FlushInterval:             100 * time.Millisecond,
		MaxBufferSize:             100,
		EnableAsync:               false, // 同步模式便于测试
		EnableStructuredDataOptim: true,
		StructuredDataBatchSize:   3, // 小批次用于测试
		StructuredDataFlushDelay:  50 * time.Millisecond,
	}
	bw := storage.NewBatchWriter(ms, batchConfig)
	defer bw.Close()

	ctx := context.Background()

	// 批量写入多个 StructuredData
	symbols := []string{"600000", "000001", "000002", "600036", "000858"}
	for i, symbol := range symbols {
		sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
		require.NoError(t, sd.SetField("symbol", symbol))
		require.NoError(t, sd.SetField("name", "测试股票"+string(rune('A'+i))))
		require.NoError(t, sd.SetField("price", float64(10+i)))
		require.NoError(t, sd.SetField("timestamp", time.Now()))

		err := bw.Write(ctx, sd)
		require.NoError(t, err)
	}

	// 等待自动刷新或手动刷新
	time.Sleep(100 * time.Millisecond)
	err := bw.Flush()
	require.NoError(t, err)

	// 验证所有数据都已保存
	for _, symbol := range symbols {
		results, err := ms.QueryBySymbol(ctx, symbol)
		require.NoError(t, err)
		assert.Len(t, results, 1, "Symbol %s should have exactly one record", symbol)
	}

	// 验证统计信息
	stats := bw.GetStats()
	assert.Equal(t, int64(len(symbols)), stats.StructuredDataRecords)
	assert.GreaterOrEqual(t, stats.StructuredDataBatches, int64(1))
}

func TestBatchWriter_StructuredData_SchemaGrouping(t *testing.T) {
	// 创建内存存储
	memConfig := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(memConfig)
	defer ms.Close()

	// 创建 BatchWriter
	batchConfig := storage.OptimizedBatchWriterConfig()
	batchConfig.StructuredDataBatchSize = 2 // 小批次便于测试
	bw := storage.NewBatchWriter(ms, batchConfig)
	defer bw.Close()

	ctx := context.Background()

	// 创建自定义 schema
	customSchema := &subscriber.DataSchema{
		Name:        "custom_test",
		Description: "自定义测试模式",
		Fields: map[string]*subscriber.FieldDefinition{
			"id": {
				Name:        "id",
				Type:        subscriber.FieldTypeInt,
				Description: "ID",
				Required:    true,
			},
			"value": {
				Name:        "value",
				Type:        subscriber.FieldTypeString,
				Description: "值",
				Required:    true,
			},
		},
		FieldOrder: []string{"id", "value"},
	}

	// 写入不同 schema 的数据
	// 1. 股票数据
	for i := 0; i < 3; i++ {
		sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
		require.NoError(t, sd.SetField("symbol", "60000"+string(rune('0'+i))))
		require.NoError(t, sd.SetField("name", "股票"+string(rune('A'+i))))
		require.NoError(t, sd.SetField("price", float64(10+i)))
		require.NoError(t, sd.SetField("timestamp", time.Now()))

		err := bw.Write(ctx, sd)
		require.NoError(t, err)
	}

	// 2. 自定义数据
	for i := 0; i < 3; i++ {
		sd := subscriber.NewStructuredData(customSchema)
		require.NoError(t, sd.SetField("id", i+1))
		require.NoError(t, sd.SetField("value", "test_value_"+string(rune('A'+i))))

		err := bw.Write(ctx, sd)
		require.NoError(t, err)
	}

	// 刷新数据
	err := bw.Flush()
	require.NoError(t, err)

	// 验证数据按 schema 分组保存
	stockResults, err := ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, stockResults, 1)

	// 验证统计信息
	stats := bw.GetStats()
	assert.Equal(t, int64(6), stats.StructuredDataRecords)          // 3+3个记录
	assert.GreaterOrEqual(t, stats.StructuredDataBatches, int64(2)) // 至少2个批次
}

func TestBatchWriter_StructuredData_MixedWithRegularData(t *testing.T) {
	// 创建内存存储
	memConfig := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(memConfig)
	defer ms.Close()

	// 创建 BatchWriter
	batchConfig := storage.DefaultBatchWriterConfig()
	bw := storage.NewBatchWriter(ms, batchConfig)
	defer bw.Close()

	ctx := context.Background()

	// 混合写入不同类型的数据
	// 1. StructuredData
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))
	err := bw.Write(ctx, sd)
	require.NoError(t, err)

	// 2. 普通的 StockData
	stockData := subscriber.StockData{
		Symbol: "000001",
		Name:   "平安银行",
		Price:  12.80,
	}
	err = bw.Write(ctx, stockData)
	require.NoError(t, err)

	// 3. Map 数据
	mapData := map[string]interface{}{
		"type": "test",
		"id":   1,
		"name": "测试数据",
	}
	err = bw.Write(ctx, mapData)
	require.NoError(t, err)

	// 刷新数据
	err = bw.Flush()
	require.NoError(t, err)

	// 验证数据都已保存
	stats := bw.GetStats()
	assert.Equal(t, int64(3), stats.TotalRecords)
	assert.Equal(t, int64(1), stats.StructuredDataRecords)

	// 验证 StructuredData 查询
	structuredResults, err := ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, structuredResults, 1)
}

func TestBatchWriter_StructuredData_FlushDelay(t *testing.T) {
	// 创建内存存储
	memConfig := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(memConfig)
	defer ms.Close()

	// 创建具有延迟刷新的 BatchWriter
	batchConfig := storage.BatchWriterConfig{
		BatchSize:                 100, // 大批次，不会触发大小限制
		FlushInterval:             0,   // 不启用定时刷新
		MaxBufferSize:             1000,
		EnableAsync:               false,
		EnableStructuredDataOptim: true,
		StructuredDataBatchSize:   100,                    // 大批次，不会触发大小限制
		StructuredDataFlushDelay:  200 * time.Millisecond, // 200ms 延迟
	}
	bw := storage.NewBatchWriter(ms, batchConfig)
	defer bw.Close()

	ctx := context.Background()

	// 写入第一个数据
	sd1 := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd1.SetField("symbol", "600000"))
	require.NoError(t, sd1.SetField("name", "浦发银行"))
	require.NoError(t, sd1.SetField("price", 10.50))
	require.NoError(t, sd1.SetField("timestamp", time.Now()))
	err := bw.Write(ctx, sd1)
	require.NoError(t, err)

	// 立即检查，数据还未刷新
	results, err := ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, results, 0, "Data should not be flushed yet")

	// 等待延迟时间
	time.Sleep(250 * time.Millisecond)

	// 写入另一个数据，应该触发前一个的延迟刷新
	sd2 := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd2.SetField("symbol", "000001"))
	require.NoError(t, sd2.SetField("name", "平安银行"))
	require.NoError(t, sd2.SetField("price", 12.80))
	require.NoError(t, sd2.SetField("timestamp", time.Now()))
	err = bw.Write(ctx, sd2)
	require.NoError(t, err)

	// 现在第一个数据应该已经刷新
	results, err = ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, results, 1, "First data should be flushed due to delay")

	// 手动刷新剩余数据
	err = bw.Flush()
	require.NoError(t, err)

	// 验证第二个数据也已保存
	results, err = ms.QueryBySymbol(ctx, "000001")
	require.NoError(t, err)
	assert.Len(t, results, 1, "Second data should be flushed")
}

func TestBatchWriter_StructuredData_Stats(t *testing.T) {
	// 创建内存存储
	memConfig := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(memConfig)
	defer ms.Close()

	// 创建 BatchWriter
	batchConfig := storage.OptimizedBatchWriterConfig()
	batchConfig.StructuredDataBatchSize = 2 // 小批次便于测试统计
	bw := storage.NewBatchWriter(ms, batchConfig)
	defer bw.Close()

	ctx := context.Background()

	// 初始统计信息
	initialStats := bw.GetStats()
	assert.Equal(t, int64(0), initialStats.StructuredDataRecords)
	assert.Equal(t, int64(0), initialStats.StructuredDataBatches)
	assert.Equal(t, 0, initialStats.StructuredDataBufferSize)

	// 写入多个 StructuredData
	for i := 0; i < 5; i++ {
		sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
		require.NoError(t, sd.SetField("symbol", "60000"+string(rune('0'+i))))
		require.NoError(t, sd.SetField("name", "测试股票"+string(rune('A'+i))))
		require.NoError(t, sd.SetField("price", float64(10+i)))
		require.NoError(t, sd.SetField("timestamp", time.Now()))

		err := bw.Write(ctx, sd)
		require.NoError(t, err)
	}

	// 刷新数据
	err := bw.Flush()
	require.NoError(t, err)

	// 检查最终统计信息
	finalStats := bw.GetStats()
	assert.Equal(t, int64(5), finalStats.StructuredDataRecords)
	assert.GreaterOrEqual(t, finalStats.StructuredDataBatches, int64(2)) // 至少2个批次（2+2+1）
	assert.GreaterOrEqual(t, finalStats.StructuredDataFlushes, int64(2))
	assert.Equal(t, 0, finalStats.StructuredDataBufferSize) // 刷新后缓冲区应为空
}

func TestBatchWriter_StructuredData_ErrorHandling(t *testing.T) {
	// 创建内存存储
	memConfig := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(memConfig)
	defer ms.Close()

	// 创建 BatchWriter
	batchConfig := storage.DefaultBatchWriterConfig()
	bw := storage.NewBatchWriter(ms, batchConfig)
	defer bw.Close()

	ctx := context.Background()

	// 测试写入无效的 StructuredData（缺少 schema）
	invalidSD := &subscriber.StructuredData{
		Schema:    nil, // 无效：缺少 schema
		Values:    make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	err := bw.Write(ctx, invalidSD)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema")

	// 验证正常数据仍然可以写入
	validSD := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, validSD.SetField("symbol", "600000"))
	require.NoError(t, validSD.SetField("name", "浦发银行"))
	require.NoError(t, validSD.SetField("price", 10.50))
	require.NoError(t, validSD.SetField("timestamp", time.Now()))

	err = bw.Write(ctx, validSD)
	require.NoError(t, err)

	err = bw.Flush()
	require.NoError(t, err)

	// 验证有效数据已保存
	results, err := ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}
