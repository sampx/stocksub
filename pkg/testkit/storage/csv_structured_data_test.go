package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSVStorage_StructuredData_Save(t *testing.T) {
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

	// 创建测试用的 StructuredData
	// 使用上海时区的时间，这样转换后应该显示为 18:30
	shanghaiTZ, _ := time.LoadLocation("Asia/Shanghai")
	testTime := time.Date(2025, 8, 24, 18, 30, 0, 0, shanghaiTZ)
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	sd.Timestamp = testTime

	// 设置测试数据
	err = sd.SetField("symbol", "600000")
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

func TestCSVStorage_StructuredData_BatchSave(t *testing.T) {
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

func TestCSVStorage_StructuredData_MixedWithStockData(t *testing.T) {
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

func TestCSVStorage_StructuredData_HeaderGeneration(t *testing.T) {
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

func TestCSVStorage_StructuredData_TimeZoneFormatting(t *testing.T) {
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
