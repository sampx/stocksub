//go:build integration

package subscriber_test

import (
	"context"
	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/core"
	"stocksub/pkg/testkit/helpers"
	"stocksub/pkg/testkit/storage"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubscriber_StructuredData_CompatibilityWithStorage 验证新结构体与现有 pkg/testkit/storage 模块的完全兼容性
func TestSubscriber_StructuredData_CompatibilityWithStorage(t *testing.T) {
	t.Run("MemoryStorage兼容性测试", func(t *testing.T) {
		testStructuredDataWithMemoryStorage(t)
	})

	t.Run("CSVStorage兼容性测试", func(t *testing.T) {
		testStructuredDataWithCSVStorage(t)
	})

	t.Run("BatchWriter兼容性测试", func(t *testing.T) {
		testStructuredDataWithBatchWriter(t)
	})
}

func testStructuredDataWithMemoryStorage(t *testing.T) {
	config := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(config)
	defer ms.Close()

	ctx := context.Background()

	var _ core.Storage = ms

	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	err := ms.Save(ctx, sd)
	require.NoError(t, err)

	query := core.Query{Symbols: []string{"600000"}}
	results, err := ms.Load(ctx, query)
	require.NoError(t, err)
	require.Len(t, results, 1)

	loadedSD, ok := results[0].(*subscriber.StructuredData)
	require.True(t, ok, "返回的数据应该是 *subscriber.StructuredData 类型")

	symbol, err := loadedSD.GetField("symbol")
	require.NoError(t, err)
	assert.Equal(t, "600000", symbol)
}

func testStructuredDataWithCSVStorage(t *testing.T) {
	tempDir := t.TempDir()
	config := storage.CSVStorageConfig{
		Directory:      tempDir,
		FilePrefix:     "compat_test",
		ResourceConfig: helpers.DefaultResourceConfig(),
	}
	csvStorage, err := storage.NewCSVStorage(config)
	require.NoError(t, err)
	defer csvStorage.Close()

	var _ core.Storage = csvStorage

	ctx := context.Background()

	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	err = csvStorage.Save(ctx, sd)
	require.NoError(t, err)

	err = csvStorage.Flush()
	require.NoError(t, err)

	stats := csvStorage.GetStats()
	assert.GreaterOrEqual(t, stats.TotalRecords, int64(1))
}

func testStructuredDataWithBatchWriter(t *testing.T) {
	memConfig := storage.DefaultMemoryStorageConfig()
	ms := storage.NewMemoryStorage(memConfig)
	defer ms.Close()

	batchConfig := storage.DefaultBatchWriterConfig()
	bw := storage.NewBatchWriter(ms, batchConfig)
	defer bw.Close()

	ctx := context.Background()

	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("timestamp", time.Now()))

	err := bw.Write(ctx, sd)
	require.NoError(t, err)

	err = bw.Flush()
	require.NoError(t, err)

	results, err := ms.QueryBySymbol(ctx, "600000")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}
