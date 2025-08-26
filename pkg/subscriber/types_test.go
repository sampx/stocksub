package subscriber

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldType_String(t *testing.T) {
	tests := []struct {
		fieldType FieldType
		expected  string
	}{
		{FieldTypeString, "string"},
		{FieldTypeInt, "int"},
		{FieldTypeFloat64, "float64"},
		{FieldTypeBool, "bool"},
		{FieldTypeTime, "time"},
		{FieldType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.fieldType.String())
		})
	}
}

func TestIsValidFieldType(t *testing.T) {
	tests := []struct {
		name         string
		value        interface{}
		expectedType FieldType
		expected     bool
	}{
		// String type tests
		{"string valid", "test", FieldTypeString, true},
		{"string invalid", 123, FieldTypeString, false},

		// Int type tests
		{"int valid", 123, FieldTypeInt, true},
		{"int8 valid", int8(123), FieldTypeInt, true},
		{"int16 valid", int16(123), FieldTypeInt, true},
		{"int32 valid", int32(123), FieldTypeInt, true},
		{"int64 valid", int64(123), FieldTypeInt, true},
		{"int invalid", "123", FieldTypeInt, false},

		// Float64 type tests
		{"float64 valid", 123.45, FieldTypeFloat64, true},
		{"float32 valid", float32(123.45), FieldTypeFloat64, true},
		{"float invalid", "123.45", FieldTypeFloat64, false},

		// Bool type tests
		{"bool valid true", true, FieldTypeBool, true},
		{"bool valid false", false, FieldTypeBool, true},
		{"bool invalid", "true", FieldTypeBool, false},

		// Time type tests
		{"time valid", time.Now(), FieldTypeTime, true},
		{"time invalid", "2023-01-01", FieldTypeTime, false},

		// Nil values (should always be valid)
		{"nil string", nil, FieldTypeString, true},
		{"nil int", nil, FieldTypeInt, true},
		{"nil float64", nil, FieldTypeFloat64, true},
		{"nil bool", nil, FieldTypeBool, true},
		{"nil time", nil, FieldTypeTime, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidFieldType(tt.value, tt.expectedType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStockDataSchema(t *testing.T) {
	// Test schema basic properties
	assert.Equal(t, "stock_data", StockDataSchema.Name)
	assert.Equal(t, "股票行情数据", StockDataSchema.Description)
	assert.NotNil(t, StockDataSchema.Fields)
	assert.NotNil(t, StockDataSchema.FieldOrder)

	// Test required fields exist
	requiredFields := []string{"symbol", "name", "price", "timestamp"}
	for _, fieldName := range requiredFields {
		field, exists := StockDataSchema.Fields[fieldName]
		assert.True(t, exists, "Required field %s should exist", fieldName)
		assert.True(t, field.Required, "Field %s should be required", fieldName)
	}

	// Test field order contains all fields
	assert.Equal(t, len(StockDataSchema.Fields), len(StockDataSchema.FieldOrder))

	// Test all fields in FieldOrder exist in Fields
	for _, fieldName := range StockDataSchema.FieldOrder {
		_, exists := StockDataSchema.Fields[fieldName]
		assert.True(t, exists, "Field %s in FieldOrder should exist in Fields", fieldName)
	}

	// Test specific field definitions
	symbolField := StockDataSchema.Fields["symbol"]
	assert.Equal(t, "symbol", symbolField.Name)
	assert.Equal(t, FieldTypeString, symbolField.Type)
	assert.Equal(t, "股票代码", symbolField.Description)
	assert.Equal(t, "如600000、000001等", symbolField.Comment)
	assert.True(t, symbolField.Required)

	priceField := StockDataSchema.Fields["price"]
	assert.Equal(t, "price", priceField.Name)
	assert.Equal(t, FieldTypeFloat64, priceField.Type)
	assert.Equal(t, "当前价格", priceField.Description)
	assert.True(t, priceField.Required)

	timestampField := StockDataSchema.Fields["timestamp"]
	assert.Equal(t, "timestamp", timestampField.Name)
	assert.Equal(t, FieldTypeTime, timestampField.Type)
	assert.Equal(t, "数据时间", timestampField.Description)
	assert.True(t, timestampField.Required)

	// Test optional fields
	volumeField := StockDataSchema.Fields["volume"]
	assert.Equal(t, "volume", volumeField.Name)
	assert.Equal(t, FieldTypeInt, volumeField.Type)
	assert.Equal(t, "成交量", volumeField.Description)
	assert.False(t, volumeField.Required)
}

func TestStructuredDataToStockData_InvalidSchema(t *testing.T) {
	// Create StructuredData with wrong schema
	wrongSchema := &DataSchema{
		Name:        "wrong_schema",
		Description: "错误的模式",
		Fields:      map[string]*FieldDefinition{},
		FieldOrder:  []string{},
	}

	sd := NewStructuredData(wrongSchema)

	// Try to convert to StockData
	stockData, err := StructuredDataToStockData(sd)
	assert.Error(t, err)
	assert.Nil(t, stockData)
	assert.Contains(t, err.Error(), "SCHEMA_NOT_FOUND")
}

func TestStockDataRoundTripConversion(t *testing.T) {
	// Create original StockData
	now := time.Now()
	original := StockData{
		Symbol:        "688036",
		Name:          "传音控股",
		Price:         85.50,
		Change:        2.30,
		ChangePercent: 2.76,
		MarketCode:    3,
		Volume:        2500000,
		Turnover:      213750000.0,
		Open:          83.20,
		High:          86.00,
		Low:           82.80,
		PrevClose:     83.20,
		BidPrice1:     85.49,
		BidVolume1:    500,
		BidPrice2:     85.48,
		BidVolume2:    800,
		BidPrice3:     85.47,
		BidVolume3:    600,
		BidPrice4:     85.46,
		BidVolume4:    1200,
		BidPrice5:     85.45,
		BidVolume5:    900,
		AskPrice1:     85.51,
		AskVolume1:    400,
		AskPrice2:     85.52,
		AskVolume2:    700,
		AskPrice3:     85.53,
		AskVolume3:    550,
		AskPrice4:     85.54,
		AskVolume4:    1000,
		AskPrice5:     85.55,
		AskVolume5:    800,
		InnerDisc:     1200000,
		OuterDisc:     1300000,
		TurnoverRate:  1.25,
		PE:            25.8,
		PB:            3.2,
		Amplitude:     3.85,
		Circulation:   850.5,
		MarketValue:   1200.8,
		LimitUp:       91.52,
		LimitDown:     74.88,
		Timestamp:     now,
	}

	// Convert to StructuredData
	sd, err := StockDataToStructuredData(original)
	require.NoError(t, err)

	// Convert back to StockData
	converted, err := StructuredDataToStockData(sd)
	require.NoError(t, err)

	// Verify all fields match
	assert.Equal(t, original.Symbol, converted.Symbol)
	assert.Equal(t, original.Name, converted.Name)
	assert.Equal(t, original.Price, converted.Price)
	assert.Equal(t, original.Change, converted.Change)
	assert.Equal(t, original.ChangePercent, converted.ChangePercent)
	assert.Equal(t, original.MarketCode, converted.MarketCode)
	assert.Equal(t, original.Volume, converted.Volume)
	assert.Equal(t, original.Turnover, converted.Turnover)
	assert.Equal(t, original.Open, converted.Open)
	assert.Equal(t, original.High, converted.High)
	assert.Equal(t, original.Low, converted.Low)
	assert.Equal(t, original.PrevClose, converted.PrevClose)

	// Test bid prices and volumes
	assert.Equal(t, original.BidPrice1, converted.BidPrice1)
	assert.Equal(t, original.BidVolume1, converted.BidVolume1)
	assert.Equal(t, original.BidPrice2, converted.BidPrice2)
	assert.Equal(t, original.BidVolume2, converted.BidVolume2)
	assert.Equal(t, original.BidPrice3, converted.BidPrice3)
	assert.Equal(t, original.BidVolume3, converted.BidVolume3)
	assert.Equal(t, original.BidPrice4, converted.BidPrice4)
	assert.Equal(t, original.BidVolume4, converted.BidVolume4)
	assert.Equal(t, original.BidPrice5, converted.BidPrice5)
	assert.Equal(t, original.BidVolume5, converted.BidVolume5)

	// Test ask prices and volumes
	assert.Equal(t, original.AskPrice1, converted.AskPrice1)
	assert.Equal(t, original.AskVolume1, converted.AskVolume1)
	assert.Equal(t, original.AskPrice2, converted.AskPrice2)
	assert.Equal(t, original.AskVolume2, converted.AskVolume2)
	assert.Equal(t, original.AskPrice3, converted.AskPrice3)
	assert.Equal(t, original.AskVolume3, converted.AskVolume3)
	assert.Equal(t, original.AskPrice4, converted.AskPrice4)
	assert.Equal(t, original.AskVolume4, converted.AskVolume4)
	assert.Equal(t, original.AskPrice5, converted.AskPrice5)
	assert.Equal(t, original.AskVolume5, converted.AskVolume5)

	// Test remaining fields
	assert.Equal(t, original.InnerDisc, converted.InnerDisc)
	assert.Equal(t, original.OuterDisc, converted.OuterDisc)
	assert.Equal(t, original.TurnoverRate, converted.TurnoverRate)
	assert.Equal(t, original.PE, converted.PE)
	assert.Equal(t, original.PB, converted.PB)
	assert.Equal(t, original.Amplitude, converted.Amplitude)
	assert.Equal(t, original.Circulation, converted.Circulation)
	assert.Equal(t, original.MarketValue, converted.MarketValue)
	assert.Equal(t, original.LimitUp, converted.LimitUp)
	assert.Equal(t, original.LimitDown, converted.LimitDown)
	assert.Equal(t, original.Timestamp, converted.Timestamp)
}

func TestStockDataSchemaFieldOrder(t *testing.T) {
	// Test that field order contains all expected fields in logical order
	expectedOrder := []string{
		"symbol", "name", "price", "change", "change_percent", "market_code",
		"volume", "turnover", "open", "high", "low", "prev_close",
		"bid_price1", "bid_volume1", "bid_price2", "bid_volume2", "bid_price3", "bid_volume3",
		"bid_price4", "bid_volume4", "bid_price5", "bid_volume5",
		"ask_price1", "ask_volume1", "ask_price2", "ask_volume2", "ask_price3", "ask_volume3",
		"ask_price4", "ask_volume4", "ask_price5", "ask_volume5",
		"inner_disc", "outer_disc",
		"turnover_rate", "pe", "pb", "amplitude", "circulation", "market_value",
		"limit_up", "limit_down", "timestamp",
	}

	assert.Equal(t, expectedOrder, StockDataSchema.FieldOrder)

	// Verify all fields in order exist in schema
	for _, fieldName := range expectedOrder {
		_, exists := StockDataSchema.Fields[fieldName]
		assert.True(t, exists, "Field %s should exist in schema", fieldName)
	}
}
