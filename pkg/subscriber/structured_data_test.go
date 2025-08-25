package subscriber_test

import (
	"testing"
	"time"

	"stocksub/pkg/subscriber"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStructuredData(t *testing.T) {
	tests := []struct {
		name   string
		schema *subscriber.DataSchema
	}{
		{
			name:   "with valid schema",
			schema: subscriber.StockDataSchema,
		},
		{
			name: "with custom schema",
			schema: &subscriber.DataSchema{
				Name:        "test_schema",
				Description: "测试模式",
				Fields: map[string]*subscriber.FieldDefinition{
					"id": {
						Name:        "id",
						Type:        subscriber.FieldTypeInt,
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
			sd := subscriber.NewStructuredData(tt.schema)
			
			assert.NotNil(t, sd)
			assert.Equal(t, tt.schema, sd.Schema)
			assert.NotNil(t, sd.Values)
			assert.NotZero(t, sd.Timestamp)
		})
	}
}

func TestStructuredData_SetField(t *testing.T) {
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)

	tests := []struct {
		name      string
		fieldName string
		value     interface{}
		wantErr   bool
		errCode   subscriber.ErrorCode
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
			errCode:   subscriber.ErrFieldNotFound,
		},
		{
			name:      "invalid field type",
			fieldName: "price",
			value:     "not a number",
			wantErr:   true,
			errCode:   subscriber.ErrInvalidFieldType,
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
				if structErr, ok := err.(*subscriber.StructuredDataError); ok {
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
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)

	// 设置一些测试数据
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("price", 10.50))

	tests := []struct {
		name        string
		fieldName   string
		expectValue interface{}
		wantErr     bool
		errCode     subscriber.ErrorCode
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
			errCode:   subscriber.ErrFieldNotFound,
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
			errCode:   subscriber.ErrRequiredFieldMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := sd.GetField(tt.fieldName)

			if tt.wantErr {
				require.Error(t, err)
				if structErr, ok := err.(*subscriber.StructuredDataError); ok {
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
		setup   func() *subscriber.StructuredData
		wantErr bool
		errCode subscriber.ErrorCode
	}{
		{
			name: "valid complete data",
			setup: func() *subscriber.StructuredData {
				sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
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
			setup: func() *subscriber.StructuredData {
				sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
				sd.SetField("symbol", "600000")
				// 缺少 name 字段
				sd.SetField("price", 10.50)
				return sd
			},
			wantErr: true,
			errCode: subscriber.ErrRequiredFieldMissing,
		},
		{
			name: "invalid field type",
			setup: func() *subscriber.StructuredData {
				sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
				sd.Values["symbol"] = "600000"
				sd.Values["name"] = "浦发银行"
				sd.Values["price"] = "invalid_price" // 错误的类型
				return sd
			},
			wantErr: true,
			errCode: subscriber.ErrInvalidFieldType,
		},
		{
			name: "valid with optional fields missing",
			setup: func() *subscriber.StructuredData {
				sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
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
				if structErr, ok := err.(*subscriber.StructuredDataError); ok {
					assert.Equal(t, tt.errCode, structErr.Code)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStockDataToStructuredData(t *testing.T) {
	stockData := subscriber.StockData{
		Symbol:        "600000",
		Name:          "浦发银行",
		Price:         10.50,
		Change:        0.15,
		ChangePercent: 1.45,
		Volume:        1250000,
		Timestamp:     time.Now(),
	}

	sd, err := subscriber.StockDataToStructuredData(stockData)
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
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	timestamp := time.Now()
	
	require.NoError(t, sd.SetField("symbol", "600000"))
	require.NoError(t, sd.SetField("name", "浦发银行"))
	require.NoError(t, sd.SetField("price", 10.50))
	require.NoError(t, sd.SetField("change", 0.15))
	require.NoError(t, sd.SetField("volume", int64(1250000)))
	require.NoError(t, sd.SetField("timestamp", timestamp))

	stockData, err := subscriber.StructuredDataToStockData(sd)
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

func TestFieldType_String(t *testing.T) {
	tests := []struct {
		fieldType subscriber.FieldType
		expected  string
	}{
		{subscriber.FieldTypeString, "string"},
		{subscriber.FieldTypeInt, "int"},
		{subscriber.FieldTypeFloat64, "float64"},
		{subscriber.FieldTypeBool, "bool"},
		{subscriber.FieldTypeTime, "time"},
		{subscriber.FieldType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.fieldType.String())
		})
	}
}

func TestStructuredDataError(t *testing.T) {
	err := subscriber.NewStructuredDataError(
		subscriber.ErrInvalidFieldType,
		"price",
		"invalid value type",
	)

	assert.Contains(t, err.Error(), "INVALID_FIELD_TYPE")
	assert.Contains(t, err.Error(), "price")
	assert.Contains(t, err.Error(), "invalid value type")
	assert.Equal(t, subscriber.ErrInvalidFieldType, err.Code)
	assert.Equal(t, "price", err.Field)
	assert.Equal(t, "invalid value type", err.Message)
}