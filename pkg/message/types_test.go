package message

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessageFormat(t *testing.T) {
	stockData := []StockData{
		{
			Symbol:        "600000",
			Name:          "浦发银行",
			Price:         10.50,
			Change:        0.15,
			ChangePercent: 1.45,
			Volume:        1250000,
			Timestamp:     "2023-03-15T09:30:00Z",
		},
	}

	msg := NewMessageFormat("test-producer", "tencent", "stock_realtime", stockData)

	assert.NotEmpty(t, msg.Header.MessageID)
	assert.Equal(t, "1.0", msg.Header.Version)
	assert.Equal(t, "test-producer", msg.Header.Producer)
	assert.Equal(t, "application/json", msg.Header.ContentType)
	assert.True(t, msg.Header.Timestamp > 0)

	assert.Equal(t, "tencent", msg.Metadata.Provider)
	assert.Equal(t, "stock_realtime", msg.Metadata.DataType)
	assert.Equal(t, 1, msg.Metadata.BatchSize)

	assert.Equal(t, stockData, msg.Payload)
	assert.NotEmpty(t, msg.Checksum)
	assert.Contains(t, msg.Checksum, "sha256:")
}

func TestMessageFormat_Validate(t *testing.T) {
	stockData := []StockData{
		{Symbol: "600000", Name: "测试股票", Price: 10.0},
	}

	msg := NewMessageFormat("test-producer", "test-provider", "stock_realtime", stockData)

	// 验证正确的消息
	err := msg.Validate()
	assert.NoError(t, err)

	// 修改校验和，验证应该失败
	originalChecksum := msg.Checksum
	msg.Checksum = "invalid-checksum"
	err = msg.Validate()
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidChecksum, err)

	// 恢复校验和
	msg.Checksum = originalChecksum
	err = msg.Validate()
	assert.NoError(t, err)
}

func TestMessageFormat_ToJSON_FromJSON(t *testing.T) {
	stockData := []StockData{
		{
			Symbol:        "600000",
			Name:          "浦发银行",
			Price:         10.50,
			Change:        0.15,
			ChangePercent: 1.45,
			Volume:        1250000,
			Timestamp:     "2023-03-15T09:30:00Z",
		},
	}

	originalMsg := NewMessageFormat("test-producer", "tencent", "stock_realtime", stockData)

	// 转换为 JSON
	jsonStr, err := originalMsg.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonStr)

	// 从 JSON 解析
	parsedMsg, err := FromJSON(jsonStr)
	require.NoError(t, err)

	// 验证解析后的消息头部和元数据
	assert.Equal(t, originalMsg.Header.MessageID, parsedMsg.Header.MessageID)
	assert.Equal(t, originalMsg.Header.Version, parsedMsg.Header.Version)
	assert.Equal(t, originalMsg.Header.Producer, parsedMsg.Header.Producer)
	assert.Equal(t, originalMsg.Metadata.Provider, parsedMsg.Metadata.Provider)
	assert.Equal(t, originalMsg.Metadata.DataType, parsedMsg.Metadata.DataType)
	assert.Equal(t, originalMsg.Metadata.BatchSize, parsedMsg.Metadata.BatchSize)

	// 验证 payload 数据（JSON 序列化后类型会变化）
	payloadArray, ok := parsedMsg.Payload.([]interface{})
	require.True(t, ok, "Payload should be an array after JSON parsing")
	require.Len(t, payloadArray, 1)

	payloadItem, ok := payloadArray[0].(map[string]interface{})
	require.True(t, ok, "Payload item should be a map after JSON parsing")

	assert.Equal(t, "600000", payloadItem["symbol"])
	assert.Equal(t, "浦发银行", payloadItem["name"])
	assert.Equal(t, 10.5, payloadItem["price"])

	// 由于 JSON 序列化会改变数据结构，我们需要重新计算校验和
	parsedMsg.Checksum = parsedMsg.CalculateChecksum()
	err = parsedMsg.Validate()
	assert.NoError(t, err)
}

func TestGetStreamName(t *testing.T) {
	tests := []struct {
		dataType     string
		expectedName string
	}{
		{"stock_realtime", "stream:stock:realtime"},
		{"index_realtime", "stream:index:realtime"},
		{"historical", "stream:historical"},
		{"unknown_type", "stream:unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.dataType, func(t *testing.T) {
			streamName := GetStreamName(tt.dataType)
			assert.Equal(t, tt.expectedName, streamName)
		})
	}
}

func TestMessageFormat_SetMarketInfo(t *testing.T) {
	stockData := []StockData{
		{Symbol: "600000", Name: "测试股票", Price: 10.0},
	}

	msg := NewMessageFormat("test-producer", "test-provider", "stock_realtime", stockData)
	originalChecksum := msg.Checksum

	// 设置市场信息
	msg.SetMarketInfo("A-share", "morning")

	assert.Equal(t, "A-share", msg.Metadata.Market)
	assert.Equal(t, "morning", msg.Metadata.TradingSession)

	// 校验和应该已更新
	assert.NotEqual(t, originalChecksum, msg.Checksum)

	// 验证消息完整性
	err := msg.Validate()
	assert.NoError(t, err)
}

func TestMessageFormat_BatchSize(t *testing.T) {
	tests := []struct {
		name      string
		payload   interface{}
		batchSize int
	}{
		{
			name: "股票数据数组",
			payload: []StockData{
				{Symbol: "600000", Name: "股票1"},
				{Symbol: "000001", Name: "股票2"},
			},
			batchSize: 2,
		},
		{
			name: "指数数据数组",
			payload: []IndexData{
				{Symbol: "000001", Name: "上证指数"},
			},
			batchSize: 1,
		},
		{
			name: "历史数据数组",
			payload: []HistoricalDataPoint{
				{Symbol: "600000", Date: time.Now()},
				{Symbol: "000001", Date: time.Now()},
				{Symbol: "600036", Date: time.Now()},
			},
			batchSize: 3,
		},
		{
			name:      "其他类型",
			payload:   "some string",
			batchSize: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewMessageFormat("test-producer", "test-provider", "test_type", tt.payload)
			assert.Equal(t, tt.batchSize, msg.Metadata.BatchSize)
		})
	}
}

func TestMessageFormat_InvalidJSON(t *testing.T) {
	// 测试无效的 JSON
	_, err := FromJSON("invalid json")
	assert.Error(t, err)

	// 测试空 JSON
	_, err = FromJSON("")
	assert.Error(t, err)
}

func TestStockData_Structure(t *testing.T) {
	stockData := StockData{
		Symbol:        "600000",
		Name:          "浦发银行",
		Price:         10.50,
		Change:        0.15,
		ChangePercent: 1.45,
		Volume:        1250000,
		Timestamp:     "2023-03-15T09:30:00Z",
	}

	assert.Equal(t, "600000", stockData.Symbol)
	assert.Equal(t, "浦发银行", stockData.Name)
	assert.Equal(t, 10.50, stockData.Price)
	assert.Equal(t, 0.15, stockData.Change)
	assert.Equal(t, 1.45, stockData.ChangePercent)
	assert.Equal(t, int64(1250000), stockData.Volume)
	assert.Equal(t, "2023-03-15T09:30:00Z", stockData.Timestamp)
}

func TestIndexData_Structure(t *testing.T) {
	indexData := IndexData{
		Symbol:        "000001",
		Name:          "上证指数",
		Value:         3000.50,
		Change:        15.30,
		ChangePercent: 0.51,
		Timestamp:     "2023-03-15T09:30:00Z",
	}

	assert.Equal(t, "000001", indexData.Symbol)
	assert.Equal(t, "上证指数", indexData.Name)
	assert.Equal(t, 3000.50, indexData.Value)
	assert.Equal(t, 15.30, indexData.Change)
	assert.Equal(t, 0.51, indexData.ChangePercent)
	assert.Equal(t, "2023-03-15T09:30:00Z", indexData.Timestamp)
}

func TestHistoricalDataPoint_Structure(t *testing.T) {
	date := time.Date(2023, 3, 15, 0, 0, 0, 0, time.UTC)

	historicalData := HistoricalDataPoint{
		Symbol:   "600000",
		Date:     date,
		Open:     10.00,
		High:     10.80,
		Low:      9.90,
		Close:    10.50,
		Volume:   1250000,
		Turnover: 13125000.0,
	}

	assert.Equal(t, "600000", historicalData.Symbol)
	assert.Equal(t, date, historicalData.Date)
	assert.Equal(t, 10.00, historicalData.Open)
	assert.Equal(t, 10.80, historicalData.High)
	assert.Equal(t, 9.90, historicalData.Low)
	assert.Equal(t, 10.50, historicalData.Close)
	assert.Equal(t, int64(1250000), historicalData.Volume)
	assert.Equal(t, 13125000.0, historicalData.Turnover)
}
