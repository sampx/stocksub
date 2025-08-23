package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/core"
	"stocksub/pkg/testkit/helpers"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSVStorage_BasicOperations(t *testing.T) {
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
	expectedFile := filepath.Join(tmpDir, "test_stock_data_"+time.Now().Format("2006-01-02")+".csv")
	_, err = os.Stat(expectedFile)
	assert.NoError(t, err, "CSV file should be created")

	// 测试统计信息
	stats := storage.GetStats()
	assert.Equal(t, int64(1), stats.TotalRecords)
}

func TestCSVStorage_BatchSave(t *testing.T) {
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

func TestMemoryStorage_BasicOperations(t *testing.T) {
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

func TestMemoryStorage_MaxRecords(t *testing.T) {
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
