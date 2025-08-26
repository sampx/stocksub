package subscriber

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		errCode   ErrorCode
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
		errCode     ErrorCode
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
		errCode ErrorCode
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
	stockData := StockData{
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
		errCode   ErrorCode
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
		errCode ErrorCode
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
		errCode    ErrorCode
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
