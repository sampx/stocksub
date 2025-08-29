package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStructuredDataSerializer(t *testing.T) {
	tests := []struct {
		name   string
		format SerializationFormat
	}{
		{"CSV format", FormatCSV},
		{"JSON format", FormatJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serializer := NewStructuredDataSerializer(tt.format)
			assert.NotNil(t, serializer)
			assert.Equal(t, tt.format, serializer.format)
			assert.NotNil(t, serializer.timezone)
		})
	}
}

func TestStructuredDataSerializer_MimeType(t *testing.T) {
	tests := []struct {
		name     string
		format   SerializationFormat
		expected string
	}{
		{"CSV mime type", FormatCSV, "text/csv"},
		{"JSON mime type", FormatJSON, "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serializer := NewStructuredDataSerializer(tt.format)
			assert.Equal(t, tt.expected, serializer.MimeType())
		})
	}
}

func TestStructuredDataSerializer_SerializeCSV(t *testing.T) {
	// 创建测试数据
	sd := createTestStructuredData(t)
	serializer := NewStructuredDataSerializer(FormatCSV)

	// 序列化
	data, err := serializer.Serialize(sd)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// 验证CSV格式
	csvContent := string(data)
	lines := strings.Split(strings.TrimSpace(csvContent), "\n")
	require.Len(t, lines, 2) // 表头 + 数据行

	// 验证表头包含中文描述
	header := lines[0]
	assert.Contains(t, header, "股票代码(symbol)")
	assert.Contains(t, header, "股票名称(name)")
	assert.Contains(t, header, "当前价格(price)")
	assert.Contains(t, header, "数据时间(timestamp)")

	// 验证数据行
	dataRow := lines[1]
	assert.Contains(t, dataRow, "600000")
	assert.Contains(t, dataRow, "浦发银行")
	assert.Contains(t, dataRow, "10.50")
}

func TestStructuredDataSerializer_SerializeJSON(t *testing.T) {
	// 创建测试数据
	sd := createTestStructuredData(t)
	serializer := NewStructuredDataSerializer(FormatJSON)

	// 序列化
	data, err := serializer.Serialize(sd)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// 验证JSON格式
	jsonContent := string(data)
	assert.Contains(t, jsonContent, "\"schema\":")
	assert.Contains(t, jsonContent, "\"values\":")
	assert.Contains(t, jsonContent, "\"timestamp\":")
	assert.Contains(t, jsonContent, "600000")
	assert.Contains(t, jsonContent, "浦发银行")
}

func TestStructuredDataSerializer_DeserializeCSV(t *testing.T) {
	// 创建原始数据
	originalSD := createTestStructuredData(t)
	serializer := NewStructuredDataSerializer(FormatCSV)

	// 序列化
	csvData, err := serializer.Serialize(originalSD)
	require.NoError(t, err)

	// 反序列化
	deserializedSD := NewStructuredData(StockDataSchema)
	err = serializer.Deserialize(csvData, deserializedSD)
	require.NoError(t, err)

	// 验证关键字段
	symbol, err := deserializedSD.GetField("symbol")
	require.NoError(t, err)
	assert.Equal(t, "600000", symbol)

	name, err := deserializedSD.GetField("name")
	require.NoError(t, err)
	assert.Equal(t, "浦发银行", name)

	price, err := deserializedSD.GetField("price")
	require.NoError(t, err)
	assert.Equal(t, 10.50, price)
}

func TestStructuredDataSerializer_DeserializeJSON(t *testing.T) {
	// 创建原始数据
	originalSD := createTestStructuredData(t)
	serializer := NewStructuredDataSerializer(FormatJSON)

	// 序列化
	jsonData, err := serializer.Serialize(originalSD)
	require.NoError(t, err)

	// 反序列化
	deserializedSD := &StructuredData{}
	err = serializer.Deserialize(jsonData, deserializedSD)
	require.NoError(t, err)

	// 验证schema
	assert.NotNil(t, deserializedSD.Schema)
	assert.Equal(t, "stock_data", deserializedSD.Schema.Name)

	// 验证关键字段
	symbol, exists := deserializedSD.Values["symbol"]
	assert.True(t, exists)
	assert.Equal(t, "600000", symbol)

	name, exists := deserializedSD.Values["name"]
	assert.True(t, exists)
	assert.Equal(t, "浦发银行", name)

	price, exists := deserializedSD.Values["price"]
	assert.True(t, exists)
	assert.Equal(t, 10.50, price)
}

func TestStructuredDataSerializer_TimeFormatting(t *testing.T) {
	// 创建包含时间的测试数据
	sd := NewStructuredData(StockDataSchema)
	testTime := time.Date(2025, 8, 24, 18, 30, 0, 0, time.UTC)

	err := sd.SetField("symbol", "600000")
	require.NoError(t, err)
	err = sd.SetField("name", "浦发银行")
	require.NoError(t, err)
	err = sd.SetField("price", 10.50)
	require.NoError(t, err)
	err = sd.SetField("timestamp", testTime)
	require.NoError(t, err)

	serializer := NewStructuredDataSerializer(FormatCSV)

	// 序列化
	csvData, err := serializer.Serialize(sd)
	require.NoError(t, err)

	// 验证时间格式（应该是上海时区的 YYYY-MM-DD HH:mm:ss 格式）
	csvContent := string(csvData)
	lines := strings.Split(strings.TrimSpace(csvContent), "\n")
	require.Len(t, lines, 2)

	// 时间应该转换为上海时区（UTC+8）
	dataRow := lines[1]
	assert.Contains(t, dataRow, "2025-08-25 02:30:00") // UTC 18:30 + 8小时 = 02:30
}

func TestStructuredDataSerializer_SerializeMultiple(t *testing.T) {
	// 创建多个测试数据
	dataList := []*StructuredData{
		createTestStructuredData(t),
		createTestStructuredData2(t),
	}

	serializer := NewStructuredDataSerializer(FormatCSV)

	// 批量序列化
	csvData, err := serializer.SerializeMultiple(dataList)
	require.NoError(t, err)
	require.NotEmpty(t, csvData)

	// 验证CSV格式
	csvContent := string(csvData)
	lines := strings.Split(strings.TrimSpace(csvContent), "\n")
	require.Len(t, lines, 3) // 表头 + 2个数据行

	// 验证表头
	header := lines[0]
	assert.Contains(t, header, "股票代码(symbol)")

	// 验证数据行
	assert.Contains(t, lines[1], "600000")
	assert.Contains(t, lines[2], "000001")
}

func TestStructuredDataSerializer_ErrorHandling(t *testing.T) {
	serializer := NewStructuredDataSerializer(FormatCSV)

	t.Run("serialize wrong type", func(t *testing.T) {
		_, err := serializer.Serialize("not a structured data")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be *StructuredData")
	})

	t.Run("deserialize wrong type", func(t *testing.T) {
		csvData := []byte("symbol,name\n600000,浦发银行")
		var wrongTarget string
		err := serializer.Deserialize(csvData, &wrongTarget)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be *StructuredData")
	})

	t.Run("invalid CSV format", func(t *testing.T) {
		csvData := []byte("invalid csv data")
		sd := NewStructuredData(StockDataSchema)
		err := serializer.Deserialize(csvData, sd)
		assert.Error(t, err)
	})

	t.Run("inconsistent schema in multiple", func(t *testing.T) {
		// 创建不同schema的数据
		schema1 := &DataSchema{Name: "schema1", Fields: map[string]*FieldDefinition{}}
		schema2 := &DataSchema{Name: "schema2", Fields: map[string]*FieldDefinition{}}

		dataList := []*StructuredData{
			{Schema: schema1, Values: map[string]interface{}{}},
			{Schema: schema2, Values: map[string]interface{}{}},
		}

		_, err := serializer.SerializeMultiple(dataList)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "inconsistent schema")
	})
}

func TestSerializationFormat_String(t *testing.T) {
	tests := []struct {
		format   SerializationFormat
		expected string
	}{
		{FormatCSV, "csv"},
		{FormatJSON, "json"},
		{SerializationFormat(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.format.String())
		})
	}
}

func TestStructuredDataSerializer_ParseCSVHeaders(t *testing.T) {
	serializer := NewStructuredDataSerializer(FormatCSV)

	tests := []struct {
		name     string
		headers  []string
		expected []string
	}{
		{
			name:     "headers with descriptions",
			headers:  []string{"股票代码(symbol)", "股票名称(name)", "当前价格(price)"},
			expected: []string{"symbol", "name", "price"},
		},
		{
			name:     "headers without descriptions",
			headers:  []string{"symbol", "name", "price"},
			expected: []string{"symbol", "name", "price"},
		},
		{
			name:     "mixed headers",
			headers:  []string{"股票代码(symbol)", "name", "当前价格(price)"},
			expected: []string{"symbol", "name", "price"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializer.parseCSVHeaders(tt.headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStructuredDataSerializer_ParseAndValidateCSVHeaders(t *testing.T) {
	serializer := NewStructuredDataSerializer(FormatCSV)

	t.Run("valid headers", func(t *testing.T) {
		headers := []string{"股票代码(symbol)", "股票名称(name)", "当前价格(price)"}
		fieldMapping, err := serializer.parseAndValidateCSVHeaders(headers, StockDataSchema)
		require.NoError(t, err)
		assert.Equal(t, []string{"symbol", "name", "price"}, fieldMapping)
	})

	t.Run("unknown field in headers", func(t *testing.T) {
		headers := []string{"股票代码(symbol)", "unknown_field", "当前价格(price)"}
		_, err := serializer.parseAndValidateCSVHeaders(headers, StockDataSchema)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown fields in CSV header")
		assert.Contains(t, err.Error(), "unknown_field")
	})

	t.Run("empty field names", func(t *testing.T) {
		headers := []string{"股票代码(symbol)", "", "当前价格(price)"}
		fieldMapping, err := serializer.parseAndValidateCSVHeaders(headers, StockDataSchema)
		require.NoError(t, err)
		assert.Equal(t, []string{"symbol", "", "price"}, fieldMapping)
	})
}

func TestStructuredDataSerializer_DeserializeMultipleCSV(t *testing.T) {
	serializer := NewStructuredDataSerializer(FormatCSV)

	// 创建测试CSV数据
	csvData := `股票代码(symbol),股票名称(name),当前价格(price),涨跌额(change),成交量(volume),数据时间(timestamp)
600000,浦发银行,10.50,0.15,1250000,2025-08-24 18:30:00
000001,平安银行,12.80,-0.05,980000,2025-08-24 18:35:00
600036,招商银行,45.20,1.20,2100000,2025-08-24 18:40:00`

	// 反序列化
	dataList, err := serializer.DeserializeMultiple([]byte(csvData), StockDataSchema)
	require.NoError(t, err)
	require.Len(t, dataList, 3)

	// 验证第一条数据
	sd1 := dataList[0]
	symbol, err := sd1.GetField("symbol")
	require.NoError(t, err)
	assert.Equal(t, "600000", symbol)

	name, err := sd1.GetField("name")
	require.NoError(t, err)
	assert.Equal(t, "浦发银行", name)

	price, err := sd1.GetField("price")
	require.NoError(t, err)
	assert.Equal(t, 10.50, price)

	// 验证第二条数据
	sd2 := dataList[1]
	symbol2, err := sd2.GetField("symbol")
	require.NoError(t, err)
	assert.Equal(t, "000001", symbol2)

	change2, err := sd2.GetField("change")
	require.NoError(t, err)
	assert.Equal(t, -0.05, change2)

	// 验证第三条数据
	sd3 := dataList[2]
	volume3, err := sd3.GetField("volume")
	require.NoError(t, err)
	assert.Equal(t, int64(2100000), volume3)
}

func TestStructuredDataSerializer_DeserializeMultipleJSON(t *testing.T) {
	serializer := NewStructuredDataSerializer(FormatJSON)

	// 创建测试JSON数据 - 注意volume需要是整数类型
	jsonData := `[
		{
			"values": {
				"symbol": "600000",
				"name": "浦发银行",
				"price": 10.50,
				"change": 0.15
			},
			"timestamp": "2025-08-24 18:30:00"
		},
		{
			"values": {
				"symbol": "000001",
				"name": "平安银行",
				"price": 12.80,
				"change": -0.05
			},
			"timestamp": "2025-08-24 18:35:00"
		}
	]`

	// 反序列化
	dataList, err := serializer.DeserializeMultiple([]byte(jsonData), StockDataSchema)
	require.NoError(t, err)
	require.Len(t, dataList, 2)

	// 验证第一条数据
	sd1 := dataList[0]
	assert.Equal(t, "600000", sd1.Values["symbol"])
	assert.Equal(t, "浦发银行", sd1.Values["name"])
	assert.Equal(t, 10.50, sd1.Values["price"])

	// 验证第二条数据
	sd2 := dataList[1]
	assert.Equal(t, "000001", sd2.Values["symbol"])
	assert.Equal(t, -0.05, sd2.Values["change"])
}

func TestStructuredDataSerializer_CSVDeserializationErrorHandling(t *testing.T) {
	serializer := NewStructuredDataSerializer(FormatCSV)

	t.Run("invalid CSV format", func(t *testing.T) {
		invalidCSV := `invalid,csv,format
missing,data`
		_, err := serializer.DeserializeMultiple([]byte(invalidCSV), StockDataSchema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read CSV data")
	})

	t.Run("unknown field in CSV", func(t *testing.T) {
		csvWithUnknownField := `symbol,unknown_field,price
600000,test,10.50`
		_, err := serializer.DeserializeMultiple([]byte(csvWithUnknownField), StockDataSchema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown fields in CSV header")
	})

	t.Run("invalid data type", func(t *testing.T) {
		csvWithInvalidType := `股票代码(symbol),当前价格(price)
600000,invalid_price`
		_, err := serializer.DeserializeMultiple([]byte(csvWithInvalidType), StockDataSchema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse value")
	})

	t.Run("empty CSV", func(t *testing.T) {
		emptyCSV := ``
		_, err := serializer.DeserializeMultiple([]byte(emptyCSV), StockDataSchema)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must contain at least header and one data row")
	})
}

func TestStructuredDataSerializer_TimeZoneHandling(t *testing.T) {
	serializer := NewStructuredDataSerializer(FormatCSV)

	// 创建包含时间的CSV数据
	csvData := `股票代码(symbol),数据时间(timestamp)
600000,2025-08-24 18:30:00`

	// 反序列化
	dataList, err := serializer.DeserializeMultiple([]byte(csvData), StockDataSchema)
	require.NoError(t, err)
	require.Len(t, dataList, 1)

	// 获取时间字段
	timestamp, err := dataList[0].GetField("timestamp")
	require.NoError(t, err)

	parsedTime, ok := timestamp.(time.Time)
	require.True(t, ok)

	// 验证时区是上海时区
	expectedTime := time.Date(2025, 8, 24, 18, 30, 0, 0, serializer.timezone)
	assert.True(t, parsedTime.Equal(expectedTime))
}

// 辅助函数：创建测试用的 StructuredData
func createTestStructuredData(t *testing.T) *StructuredData {
	sd := NewStructuredData(StockDataSchema)

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
	err = sd.SetField("timestamp", time.Date(2025, 8, 24, 18, 30, 0, 0, time.UTC))
	require.NoError(t, err)

	return sd
}

// 辅助函数：创建第二个测试用的 StructuredData
func createTestStructuredData2(t *testing.T) *StructuredData {
	sd := NewStructuredData(StockDataSchema)

	err := sd.SetField("symbol", "000001")
	require.NoError(t, err)
	err = sd.SetField("name", "平安银行")
	require.NoError(t, err)
	err = sd.SetField("price", 12.80)
	require.NoError(t, err)
	err = sd.SetField("change", -0.05)
	require.NoError(t, err)
	err = sd.SetField("change_percent", -0.39)
	require.NoError(t, err)
	err = sd.SetField("volume", int64(980000))
	require.NoError(t, err)
	err = sd.SetField("timestamp", time.Date(2025, 8, 24, 18, 30, 0, 0, time.UTC))
	require.NoError(t, err)

	return sd
}
