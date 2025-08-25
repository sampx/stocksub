package tests_test

import (
	"context"
	"testing"
	"time"

	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/core"
	"stocksub/pkg/testkit/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStorage_StructuredData_Basic(t *testing.T) {
	config := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(config)
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

func TestMemoryStorage_StructuredData_BatchSave(t *testing.T) {
	config := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(config)
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

func TestMemoryStorage_QueryBySymbol(t *testing.T) {
	config := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(config)
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

func TestMemoryStorage_QueryByTimeRange(t *testing.T) {
	config := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(config)
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

func TestMemoryStorage_MixedDataTypes(t *testing.T) {
	config := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(config)
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

func TestMemoryStorage_CapacityLimit(t *testing.T) {
	// 使用较小的容量限制进行测试
	config := storage.MemoryStorageConfig{
		MaxRecords:      5,
		EnableIndex:     true,
		TTL:             0, // 不启用TTL
		CleanupInterval: 0,
	}
	ms := storage.NewMemoryStorage(config)
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

func TestMemoryStorage_GetStats(t *testing.T) {
	config := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(config)
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
