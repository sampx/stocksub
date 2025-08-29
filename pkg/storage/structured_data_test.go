package storage

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stocksub/pkg/core"
	"stocksub/pkg/error"
)

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

	// Try to convert to core.StockData
	stockData, err := StructuredDataToStockData(sd)
	assert.Error(t, err)
	assert.Nil(t, stockData)
	assert.Contains(t, err.Error(), "SCHEMA_NOT_FOUND")
}

func TestStockDataRoundTripConversion(t *testing.T) {
	// Create original core.StockData
	now := time.Now()
	original := core.StockData{
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

	// Convert back to core.StockData
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

func TestNewStructuredData(t *testing.T) {
	tests := []struct {
		name   string
		schema *DataSchema
	}{
		{
			name:   "with valid schema",
			schema: StockDataSchema,
		},
		{
			name: "with custom schema",
			schema: &DataSchema{
				Name:        "test_schema",
				Description: "测试模式",
				Fields: map[string]*FieldDefinition{
					"id": {
						Name:        "id",
						Type:        FieldTypeInt,
						Description: "ID",
						Required:    true,
					},
				},
				FieldOrder: []string{"id"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sd := NewStructuredData(tt.schema)

			assert.NotNil(t, sd)
			assert.Equal(t, tt.schema, sd.Schema)
			assert.NotNil(t, sd.Values)
			assert.NotZero(t, sd.Timestamp)
		})
	}
}

func TestStructuredData_SetField(t *testing.T) {
	sd := NewStructuredData(StockDataSchema)

	tests := []struct {
		name      string
		fieldName string
		value     interface{}
		wantErr   bool
		errCode   error.ErrorCode
	}{
		{
			name:      "valid string field",
			fieldName: "symbol",
			value:     "600000",
			wantErr:   false,
		},
		{
			name:      "valid float field",
			fieldName: "price",
			value:     10.50,
			wantErr:   false,
		},
		{
			name:      "valid int field",
			fieldName: "volume",
			value:     1000000,
			wantErr:   false,
		},
		{
			name:      "invalid field name",
			fieldName: "invalid_field",
			value:     "test",
			wantErr:   true,
			errCode:   ErrFieldNotFound,
		},
		{
			name:      "invalid field type",
			fieldName: "price",
			value:     "not a number",
			wantErr:   true,
			errCode:   ErrInvalidFieldType,
		},
		{
			name:      "nil value for non-required field",
			fieldName: "change",
			value:     nil,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sd.SetField(tt.fieldName, tt.value)

			if tt.wantErr {
				require.Error(t, err)
				if structErr, ok := err.(*StructuredDataError); ok {
					assert.Equal(t, tt.errCode, structErr.Code)
				}
			} else {
				require.NoError(t, err)
				value, getErr := sd.GetField(tt.fieldName)
				require.NoError(t, getErr)
				assert.Equal(t, tt.value, value)
			}
		})
	}
}

func TestStructuredData_GetField(t *testing.T) {
	sd := NewStructuredData(StockDataSchema)

	// 设置一些测试数据
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("price", 10.50))

	tests := []struct {
		name        string
		fieldName   string
		expectValue interface{}
		wantErr     bool
		errCode     error.ErrorCode
	}{
		{
			name:        "existing field",
			fieldName:   "symbol",
			expectValue: "600000",
			wantErr:     false,
		},
		{
			name:        "existing numeric field",
			fieldName:   "price",
			expectValue: 10.50,
			wantErr:     false,
		},
		{
			name:      "non-existing field",
			fieldName: "invalid_field",
			wantErr:   true,
			errCode:   ErrFieldNotFound,
		},
		{
			name:        "non-existing optional field",
			fieldName:   "change",
			expectValue: nil,
			wantErr:     false,
		},
		{
			name:      "non-existing required field",
			fieldName: "name",
			wantErr:   true,
			errCode:   ErrRequiredFieldMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := sd.GetField(tt.fieldName)

			if tt.wantErr {
				require.Error(t, err)
				if structErr, ok := err.(*StructuredDataError); ok {
					assert.Equal(t, tt.errCode, structErr.Code)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectValue, value)
			}
		})
	}
}

func TestStructuredData_ValidateData(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *StructuredData
		wantErr bool
		errCode error.ErrorCode
	}{
		{
			name: "valid complete data",
			setup: func() *StructuredData {
				sd := NewStructuredData(StockDataSchema)
				sd.SetField("symbol", "600000")
				sd.SetField("name", "浦发银行")
				sd.SetField("price", 10.50)
				sd.SetField("timestamp", time.Now())
				return sd
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			setup: func() *StructuredData {
				sd := NewStructuredData(StockDataSchema)
				sd.SetField("symbol", "600000")
				// 缺少 name 字段
				sd.SetField("price", 10.50)
				return sd
			},
			wantErr: true,
			errCode: ErrRequiredFieldMissing,
		},
		{
			name: "invalid field type",
			setup: func() *StructuredData {
				sd := NewStructuredData(StockDataSchema)
				sd.SetField("symbol", "600000")
				sd.SetField("name", "浦发银行")
				sd.SetField("timestamp", time.Now()) // 提供所有其他必填字段
				// 直接设置错误类型的值来测试验证
				sd.Values["price"] = "invalid_price" // 错误的类型
				return sd
			},
			wantErr: true,
			errCode: ErrInvalidFieldType,
		},
		{
			name: "valid with optional fields missing",
			setup: func() *StructuredData {
				sd := NewStructuredData(StockDataSchema)
				sd.SetField("symbol", "600000")
				sd.SetField("name", "浦发银行")
				sd.SetField("price", 10.50)
				sd.SetField("timestamp", time.Now())
				// 可选字段如 change、volume 等可以缺失
				return sd
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sd := tt.setup()
			err := sd.ValidateData()

			if tt.wantErr {
				require.Error(t, err)
				if structErr, ok := err.(*StructuredDataError); ok {
					assert.Equal(t, tt.errCode, structErr.Code)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStockDataToStructuredData(t *testing.T) {
	stockData := core.StockData{
		Symbol:        "600000",
		Name:          "浦发银行",
		Price:         10.50,
		Change:        0.15,
		ChangePercent: 1.45,
		Volume:        1250000,
		Timestamp:     time.Now(),
	}

	sd, err := StockDataToStructuredData(stockData)
	require.NoError(t, err)
	require.NotNil(t, sd)

	// 验证核心字段
	symbol, err := sd.GetField("symbol")
	require.NoError(t, err)
	assert.Equal(t, "600000", symbol)

	name, err := sd.GetField("name")
	require.NoError(t, err)
	assert.Equal(t, "浦发银行", name)

	price, err := sd.GetField("price")
	require.NoError(t, err)
	assert.Equal(t, 10.50, price)

	volume, err := sd.GetField("volume")
	require.NoError(t, err)
	assert.Equal(t, int64(1250000), volume)

	// 验证时间戳
	assert.Equal(t, stockData.Timestamp, sd.Timestamp)
}

func TestStructuredDataToStockData(t *testing.T) {
	// 创建 StructuredData
	sd := NewStructuredData(StockDataSchema)
	timestamp := time.Now()

	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("change", 0.15))
	require.NoError(t, sd.SetField("volume", int64(1250000)))
	require.NoError(t, sd.SetField("timestamp", timestamp))

	stockData, err := StructuredDataToStockData(sd)
	require.NoError(t, err)
	require.NotNil(t, stockData)

	// 验证转换结果
	assert.Equal(t, "600000", stockData.Symbol)
	assert.Equal(t, "浦发银行", stockData.Name)
	assert.Equal(t, 10.50, stockData.Price)
	assert.Equal(t, 0.15, stockData.Change)
	assert.Equal(t, int64(1250000), stockData.Volume)
	assert.Equal(t, timestamp, stockData.Timestamp)
}

func TestStructuredData_SetFieldSafe(t *testing.T) {
	sd := NewStructuredData(StockDataSchema)

	tests := []struct {
		name      string
		fieldName string
		value     interface{}
		wantErr   bool
		errCode   error.ErrorCode
	}{
		{
			name:      "valid field",
			fieldName: "symbol",
			value:     "600000",
			wantErr:   false,
		},
		{
			name:      "invalid field name",
			fieldName: "nonexistent",
			value:     "test",
			wantErr:   true,
			errCode:   ErrFieldNotFound,
		},
		{
			name:      "invalid price (negative)",
			fieldName: "price",
			value:     -10.5,
			wantErr:   true,
			errCode:   ErrFieldValidationFailed,
		},
		{
			name:      "invalid symbol (too short)",
			fieldName: "symbol",
			value:     "6",
			wantErr:   true,
			errCode:   ErrFieldValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sd.SetFieldSafe(tt.fieldName, tt.value)

			if tt.wantErr {
				require.Error(t, err)
				if structErr, ok := err.(*StructuredDataError); ok {
					assert.Equal(t, tt.errCode, structErr.Code)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStructuredData_ValidateDataComplete(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *StructuredData
		wantErr bool
		errCode error.ErrorCode
	}{
		{
			name: "valid complete data",
			setup: func() *StructuredData {
				sd := NewStructuredData(StockDataSchema)
				sd.SetFieldSafe("symbol", "600000")
				sd.SetFieldSafe("name", "浦发银行")
				sd.SetFieldSafe("price", 10.50)
				sd.SetFieldSafe("timestamp", time.Now())
				return sd
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			setup: func() *StructuredData {
				sd := NewStructuredData(StockDataSchema)
				sd.SetFieldSafe("symbol", "600000")
				// 缺少必填字段 name
				sd.SetFieldSafe("price", 10.50)
				return sd
			},
			wantErr: true,
			errCode: ErrRequiredFieldMissing,
		},
		{
			name: "unknown field in data",
			setup: func() *StructuredData {
				sd := NewStructuredData(StockDataSchema)
				sd.SetFieldSafe("symbol", "600000")
				sd.SetFieldSafe("name", "浦发银行")
				sd.SetFieldSafe("price", 10.50)
				sd.SetFieldSafe("timestamp", time.Now())
				// 添加未知字段
				sd.Values["unknown_field"] = "unknown_value"
				return sd
			},
			wantErr: true,
			errCode: ErrFieldNotFound,
		},
		{
			name: "invalid schema",
			setup: func() *StructuredData {
				invalidSchema := &DataSchema{
					Name:   "", // 无效的空名称
					Fields: map[string]*FieldDefinition{},
				}
				return NewStructuredData(invalidSchema)
			},
			wantErr: true,
			errCode: ErrSchemaNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sd := tt.setup()
			err := sd.ValidateDataComplete()

			if tt.wantErr {
				require.Error(t, err)
				if structErr, ok := err.(*StructuredDataError); ok {
					assert.Equal(t, tt.errCode, structErr.Code)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStructuredData_GetFieldSafe(t *testing.T) {
	sd := NewStructuredData(StockDataSchema)
	sd.SetFieldSafe("symbol", "600000")
	sd.SetFieldSafe("price", 10.50)

	tests := []struct {
		name       string
		fieldName  string
		targetType FieldType
		wantErr    bool
		errCode    error.ErrorCode
		expected   interface{}
	}{
		{
			name:       "valid field with correct type",
			fieldName:  "symbol",
			targetType: FieldTypeString,
			wantErr:    false,
			expected:   "600000",
		},
		{
			name:       "valid field with wrong type",
			fieldName:  "symbol",
			targetType: FieldTypeInt,
			wantErr:    true,
			errCode:    ErrInvalidFieldType,
		},
		{
			name:       "nonexistent field",
			fieldName:  "nonexistent",
			targetType: FieldTypeString,
			wantErr:    true,
			errCode:    ErrFieldNotFound,
		},
		{
			name:       "missing required field",
			fieldName:  "name", // 必填但未设置
			targetType: FieldTypeString,
			wantErr:    true,
			errCode:    ErrRequiredFieldMissing,
		},
		{
			name:       "missing optional field",
			fieldName:  "change", // 可选且未设置
			targetType: FieldTypeFloat64,
			wantErr:    false,
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := sd.GetFieldSafe(tt.fieldName, tt.targetType)

			if tt.wantErr {
				require.Error(t, err)
				if structErr, ok := err.(*StructuredDataError); ok {
					assert.Equal(t, tt.errCode, structErr.Code)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, value)
			}
		})
	}
}

func TestStockDataSchema_Validation(t *testing.T) {
	// 测试预定义的股票数据模式是否有效
	err := ValidateSchema(StockDataSchema)
	require.NoError(t, err)

	// 验证模式中的所有字段定义
	for fieldName, fieldDef := range StockDataSchema.Fields {
		err := ValidateFieldDefinition(fieldName, fieldDef)
		require.NoError(t, err, "Field %s should be valid", fieldName)
	}

	// 验证字段顺序是否完整
	assert.NotEmpty(t, StockDataSchema.FieldOrder)

	fieldOrderMap := make(map[string]bool)
	for _, fieldName := range StockDataSchema.FieldOrder {
		fieldOrderMap[fieldName] = true
		_, exists := StockDataSchema.Fields[fieldName]
		assert.True(t, exists, "Field %s in order should exist in fields", fieldName)
	}

	// 验证所有字段都在顺序中
	for fieldName := range StockDataSchema.Fields {
		assert.True(t, fieldOrderMap[fieldName], "Field %s should be in field order", fieldName)
	}
}

func TestValidateSchema(t *testing.T) {
	tests := []struct {
		name    string
		schema  *DataSchema
		wantErr bool
		errCode error.ErrorCode
	}{
		{
			name:    "nil schema",
			schema:  nil,
			wantErr: true,
			errCode: ErrSchemaNotFound,
		},
		{
			name: "empty schema name",
			schema: &DataSchema{
				Name:   "",
				Fields: map[string]*FieldDefinition{},
			},
			wantErr: true,
			errCode: ErrSchemaNotFound,
		},
		{
			name: "no fields",
			schema: &DataSchema{
				Name:   "test",
				Fields: map[string]*FieldDefinition{},
			},
			wantErr: true,
			errCode: ErrSchemaNotFound,
		},
		{
			name: "valid schema",
			schema: &DataSchema{
				Name:        "test",
				Description: "测试模式",
				Fields: map[string]*FieldDefinition{
					"id": {
						Name:        "id",
						Type:        FieldTypeInt,
						Description: "ID",
						Required:    true,
					},
				},
				FieldOrder: []string{"id"},
			},
			wantErr: false,
		},
		{
			name: "field order mismatch - extra field in order",
			schema: &DataSchema{
				Name:        "test",
				Description: "测试模式",
				Fields: map[string]*FieldDefinition{
					"id": {
						Name:        "id",
						Type:        FieldTypeInt,
						Description: "ID",
						Required:    true,
					},
				},
				FieldOrder: []string{"id", "nonexistent"},
			},
			wantErr: true,
			errCode: ErrFieldNotFound,
		},
		{
			name: "field order mismatch - missing field in order",
			schema: &DataSchema{
				Name:        "test",
				Description: "测试模式",
				Fields: map[string]*FieldDefinition{
					"id": {
						Name:        "id",
						Type:        FieldTypeInt,
						Description: "ID",
						Required:    true,
					},
					"name": {
						Name:        "name",
						Type:        FieldTypeString,
						Description: "名称",
						Required:    false,
					},
				},
				FieldOrder: []string{"id"}, // 缺少 name
			},
			wantErr: true,
			errCode: ErrFieldNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSchema(tt.schema)

			if tt.wantErr {
				require.Error(t, err)
				if structErr, ok := err.(*StructuredDataError); ok {
					assert.Equal(t, tt.errCode, structErr.Code)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateFieldDefinition(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		fieldDef  *FieldDefinition
		wantErr   bool
		errCode   error.ErrorCode
	}{
		{
			name:      "nil field definition",
			fieldName: "test",
			fieldDef:  nil,
			wantErr:   true,
			errCode:   ErrFieldNotFound,
		},
		{
			name:      "empty field name",
			fieldName: "test",
			fieldDef: &FieldDefinition{
				Name: "",
				Type: FieldTypeString,
			},
			wantErr: true,
			errCode: ErrInvalidFieldType,
		},
		{
			name:      "field name mismatch",
			fieldName: "test",
			fieldDef: &FieldDefinition{
				Name: "different",
				Type: FieldTypeString,
			},
			wantErr: true,
			errCode: ErrInvalidFieldType,
		},
		{
			name:      "invalid field type",
			fieldName: "test",
			fieldDef: &FieldDefinition{
				Name: "test",
				Type: FieldType(999),
			},
			wantErr: true,
			errCode: ErrInvalidFieldType,
		},
		{
			name:      "invalid default value type",
			fieldName: "test",
			fieldDef: &FieldDefinition{
				Name:         "test",
				Type:         FieldTypeInt,
				DefaultValue: "string_value", // 应该是 int
			},
			wantErr: true,
			errCode: ErrInvalidFieldType,
		},
		{
			name:      "valid field definition",
			fieldName: "test",
			fieldDef: &FieldDefinition{
				Name:        "test",
				Type:        FieldTypeString,
				Description: "测试字段",
				Required:    true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldDefinition(tt.fieldName, tt.fieldDef)

			if tt.wantErr {
				require.Error(t, err)
				if structErr, ok := err.(*StructuredDataError); ok {
					assert.Equal(t, tt.errCode, structErr.Code)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateFieldValue(t *testing.T) {
	fieldDef := &FieldDefinition{
		Name:        "price",
		Type:        FieldTypeFloat64,
		Description: "价格",
		Required:    true,
	}

	tests := []struct {
		name      string
		fieldName string
		value     interface{}
		fieldDef  *FieldDefinition
		wantErr   bool
		errCode   error.ErrorCode
	}{
		{
			name:      "nil field definition",
			fieldName: "price",
			value:     10.5,
			fieldDef:  nil,
			wantErr:   true,
			errCode:   ErrFieldNotFound,
		},
		{
			name:      "required field missing",
			fieldName: "price",
			value:     nil,
			fieldDef:  fieldDef,
			wantErr:   true,
			errCode:   ErrRequiredFieldMissing,
		},
		{
			name:      "valid value",
			fieldName: "price",
			value:     10.5,
			fieldDef:  fieldDef,
			wantErr:   false,
		},
		{
			name:      "invalid type",
			fieldName: "price",
			value:     "not_a_number",
			fieldDef:  fieldDef,
			wantErr:   true,
			errCode:   ErrInvalidFieldType,
		},
		{
			name:      "NaN value",
			fieldName: "price",
			value:     math.NaN(),
			fieldDef:  fieldDef,
			wantErr:   true,
			errCode:   ErrInvalidFieldType,
		},
		{
			name:      "Infinity value",
			fieldName: "price",
			value:     math.Inf(1),
			fieldDef:  fieldDef,
			wantErr:   true,
			errCode:   ErrInvalidFieldType,
		},
		{
			name:      "negative price",
			fieldName: "price",
			value:     -10.5,
			fieldDef:  fieldDef,
			wantErr:   true,
			errCode:   ErrFieldValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldValue(tt.fieldName, tt.value, tt.fieldDef)

			if tt.wantErr {
				require.Error(t, err)
				if structErr, ok := err.(*StructuredDataError); ok {
					assert.Equal(t, tt.errCode, structErr.Code)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateFieldValue_RangeValidation(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		fieldType FieldType
		value     interface{}
		wantErr   bool
	}{
		// String 字段测试
		{
			name:      "valid symbol",
			fieldName: "symbol",
			fieldType: FieldTypeString,
			value:     "600000",
			wantErr:   false,
		},
		{
			name:      "symbol too short",
			fieldName: "symbol",
			fieldType: FieldTypeString,
			value:     "6",
			wantErr:   true,
		},
		{
			name:      "symbol too long",
			fieldName: "symbol",
			fieldType: FieldTypeString,
			value:     "12345678901",
			wantErr:   true,
		},
		{
			name:      "valid name",
			fieldName: "name",
			fieldType: FieldTypeString,
			value:     "浦发银行",
			wantErr:   false,
		},
		{
			name:      "name too long",
			fieldName: "name",
			fieldType: FieldTypeString,
			value:     "这是一个非常非常非常非常非常非常非常非常非常非常非常非常非常非常长的股票名称",
			wantErr:   true,
		},
		// Int 字段测试
		{
			name:      "valid volume",
			fieldName: "volume",
			fieldType: FieldTypeInt,
			value:     1000000,
			wantErr:   false,
		},
		{
			name:      "negative volume",
			fieldName: "volume",
			fieldType: FieldTypeInt,
			value:     -1000,
			wantErr:   true,
		},
		{
			name:      "valid bid_volume1",
			fieldName: "bid_volume1",
			fieldType: FieldTypeInt,
			value:     int64(5000),
			wantErr:   false,
		},
		{
			name:      "negative bid_volume1",
			fieldName: "bid_volume1",
			fieldType: FieldTypeInt,
			value:     int64(-100),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldDef := &FieldDefinition{
				Name:        tt.fieldName,
				Type:        tt.fieldType,
				Description: "测试字段",
				Required:    false,
			}

			err := ValidateFieldValue(tt.fieldName, tt.value, fieldDef)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
