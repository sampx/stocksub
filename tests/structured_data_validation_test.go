package tests

import (
	"math"
	"testing"
	"time"

	"stocksub/pkg/subscriber"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSchema(t *testing.T) {
	tests := []struct {
		name    string
		schema  *subscriber.DataSchema
		wantErr bool
		errCode subscriber.ErrorCode
	}{
		{
			name:    "nil schema",
			schema:  nil,
			wantErr: true,
			errCode: subscriber.ErrSchemaNotFound,
		},
		{
			name: "empty schema name",
			schema: &subscriber.DataSchema{
				Name:   "",
				Fields: map[string]*subscriber.FieldDefinition{},
			},
			wantErr: true,
			errCode: subscriber.ErrSchemaNotFound,
		},
		{
			name: "no fields",
			schema: &subscriber.DataSchema{
				Name:   "test",
				Fields: map[string]*subscriber.FieldDefinition{},
			},
			wantErr: true,
			errCode: subscriber.ErrSchemaNotFound,
		},
		{
			name: "valid schema",
			schema: &subscriber.DataSchema{
				Name:        "test",
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
			wantErr: false,
		},
		{
			name: "field order mismatch - extra field in order",
			schema: &subscriber.DataSchema{
				Name:        "test",
				Description: "测试模式",
				Fields: map[string]*subscriber.FieldDefinition{
					"id": {
						Name:        "id",
						Type:        subscriber.FieldTypeInt,
						Description: "ID",
						Required:    true,
					},
				},
				FieldOrder: []string{"id", "nonexistent"},
			},
			wantErr: true,
			errCode: subscriber.ErrFieldNotFound,
		},
		{
			name: "field order mismatch - missing field in order",
			schema: &subscriber.DataSchema{
				Name:        "test",
				Description: "测试模式",
				Fields: map[string]*subscriber.FieldDefinition{
					"id": {
						Name:        "id",
						Type:        subscriber.FieldTypeInt,
						Description: "ID",
						Required:    true,
					},
					"name": {
						Name:        "name",
						Type:        subscriber.FieldTypeString,
						Description: "名称",
						Required:    false,
					},
				},
				FieldOrder: []string{"id"}, // 缺少 name
			},
			wantErr: true,
			errCode: subscriber.ErrFieldNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := subscriber.ValidateSchema(tt.schema)

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

func TestValidateFieldDefinition(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		fieldDef  *subscriber.FieldDefinition
		wantErr   bool
		errCode   subscriber.ErrorCode
	}{
		{
			name:      "nil field definition",
			fieldName: "test",
			fieldDef:  nil,
			wantErr:   true,
			errCode:   subscriber.ErrFieldNotFound,
		},
		{
			name:      "empty field name",
			fieldName: "test",
			fieldDef: &subscriber.FieldDefinition{
				Name: "",
				Type: subscriber.FieldTypeString,
			},
			wantErr: true,
			errCode: subscriber.ErrInvalidFieldType,
		},
		{
			name:      "field name mismatch",
			fieldName: "test",
			fieldDef: &subscriber.FieldDefinition{
				Name: "different",
				Type: subscriber.FieldTypeString,
			},
			wantErr: true,
			errCode: subscriber.ErrInvalidFieldType,
		},
		{
			name:      "invalid field type",
			fieldName: "test",
			fieldDef: &subscriber.FieldDefinition{
				Name: "test",
				Type: subscriber.FieldType(999),
			},
			wantErr: true,
			errCode: subscriber.ErrInvalidFieldType,
		},
		{
			name:      "invalid default value type",
			fieldName: "test",
			fieldDef: &subscriber.FieldDefinition{
				Name:         "test",
				Type:         subscriber.FieldTypeInt,
				DefaultValue: "string_value", // 应该是 int
			},
			wantErr: true,
			errCode: subscriber.ErrInvalidFieldType,
		},
		{
			name:      "valid field definition",
			fieldName: "test",
			fieldDef: &subscriber.FieldDefinition{
				Name:        "test",
				Type:        subscriber.FieldTypeString,
				Description: "测试字段",
				Required:    true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := subscriber.ValidateFieldDefinition(tt.fieldName, tt.fieldDef)

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

func TestValidateFieldValue(t *testing.T) {
	fieldDef := &subscriber.FieldDefinition{
		Name:        "price",
		Type:        subscriber.FieldTypeFloat64,
		Description: "价格",
		Required:    true,
	}

	tests := []struct {
		name      string
		fieldName string
		value     interface{}
		fieldDef  *subscriber.FieldDefinition
		wantErr   bool
		errCode   subscriber.ErrorCode
	}{
		{
			name:      "nil field definition",
			fieldName: "price",
			value:     10.5,
			fieldDef:  nil,
			wantErr:   true,
			errCode:   subscriber.ErrFieldNotFound,
		},
		{
			name:      "required field missing",
			fieldName: "price",
			value:     nil,
			fieldDef:  fieldDef,
			wantErr:   true,
			errCode:   subscriber.ErrRequiredFieldMissing,
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
			errCode:   subscriber.ErrInvalidFieldType,
		},
		{
			name:      "NaN value",
			fieldName: "price",
			value:     math.NaN(),
			fieldDef:  fieldDef,
			wantErr:   true,
			errCode:   subscriber.ErrInvalidFieldType,
		},
		{
			name:      "Infinity value",
			fieldName: "price",
			value:     math.Inf(1),
			fieldDef:  fieldDef,
			wantErr:   true,
			errCode:   subscriber.ErrInvalidFieldType,
		},
		{
			name:      "negative price",
			fieldName: "price",
			value:     -10.5,
			fieldDef:  fieldDef,
			wantErr:   true,
			errCode:   subscriber.ErrFieldValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := subscriber.ValidateFieldValue(tt.fieldName, tt.value, tt.fieldDef)

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

func TestValidateFieldValue_RangeValidation(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		fieldType subscriber.FieldType
		value     interface{}
		wantErr   bool
	}{
		// String 字段测试
		{
			name:      "valid symbol",
			fieldName: "symbol",
			fieldType: subscriber.FieldTypeString,
			value:     "600000",
			wantErr:   false,
		},
		{
			name:      "symbol too short",
			fieldName: "symbol",
			fieldType: subscriber.FieldTypeString,
			value:     "6",
			wantErr:   true,
		},
		{
			name:      "symbol too long",
			fieldName: "symbol",
			fieldType: subscriber.FieldTypeString,
			value:     "12345678901",
			wantErr:   true,
		},
		{
			name:      "valid name",
			fieldName: "name",
			fieldType: subscriber.FieldTypeString,
			value:     "浦发银行",
			wantErr:   false,
		},
		{
			name:      "name too long",
			fieldName: "name",
			fieldType: subscriber.FieldTypeString,
			value:     "这是一个非常非常非常非常非常非常非常非常非常非常非常非常非常非常长的股票名称",
			wantErr:   true,
		},
		// Int 字段测试
		{
			name:      "valid volume",
			fieldName: "volume",
			fieldType: subscriber.FieldTypeInt,
			value:     1000000,
			wantErr:   false,
		},
		{
			name:      "negative volume",
			fieldName: "volume",
			fieldType: subscriber.FieldTypeInt,
			value:     -1000,
			wantErr:   true,
		},
		{
			name:      "valid bid_volume1",
			fieldName: "bid_volume1",
			fieldType: subscriber.FieldTypeInt,
			value:     int64(5000),
			wantErr:   false,
		},
		{
			name:      "negative bid_volume1",
			fieldName: "bid_volume1",
			fieldType: subscriber.FieldTypeInt,
			value:     int64(-100),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldDef := &subscriber.FieldDefinition{
				Name:        tt.fieldName,
				Type:        tt.fieldType,
				Description: "测试字段",
				Required:    false,
			}

			err := subscriber.ValidateFieldValue(tt.fieldName, tt.value, fieldDef)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStructuredData_SetFieldSafe(t *testing.T) {
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)

	tests := []struct {
		name      string
		fieldName string
		value     interface{}
		wantErr   bool
		errCode   subscriber.ErrorCode
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
			errCode:   subscriber.ErrFieldNotFound,
		},
		{
			name:      "invalid price (negative)",
			fieldName: "price",
			value:     -10.5,
			wantErr:   true,
			errCode:   subscriber.ErrFieldValidationFailed,
		},
		{
			name:      "invalid symbol (too short)",
			fieldName: "symbol",
			value:     "6",
			wantErr:   true,
			errCode:   subscriber.ErrFieldValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sd.SetFieldSafe(tt.fieldName, tt.value)

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

func TestStructuredData_ValidateDataComplete(t *testing.T) {
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
			setup: func() *subscriber.StructuredData {
				sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
				sd.SetFieldSafe("symbol", "600000")
				// 缺少必填字段 name
				sd.SetFieldSafe("price", 10.50)
				return sd
			},
			wantErr: true,
			errCode: subscriber.ErrRequiredFieldMissing,
		},
		{
			name: "unknown field in data",
			setup: func() *subscriber.StructuredData {
				sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
				sd.SetFieldSafe("symbol", "600000")
				sd.SetFieldSafe("name", "浦发银行")
				sd.SetFieldSafe("price", 10.50)
				sd.SetFieldSafe("timestamp", time.Now())
				// 添加未知字段
				sd.Values["unknown_field"] = "unknown_value"
				return sd
			},
			wantErr: true,
			errCode: subscriber.ErrFieldNotFound,
		},
		{
			name: "invalid schema",
			setup: func() *subscriber.StructuredData {
				invalidSchema := &subscriber.DataSchema{
					Name:   "", // 无效的空名称
					Fields: map[string]*subscriber.FieldDefinition{},
				}
				return subscriber.NewStructuredData(invalidSchema)
			},
			wantErr: true,
			errCode: subscriber.ErrSchemaNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sd := tt.setup()
			err := sd.ValidateDataComplete()

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

func TestStructuredData_GetFieldSafe(t *testing.T) {
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)
	sd.SetFieldSafe("symbol", "600000")
	sd.SetFieldSafe("price", 10.50)

	tests := []struct {
		name       string
		fieldName  string
		targetType subscriber.FieldType
		wantErr    bool
		errCode    subscriber.ErrorCode
		expected   interface{}
	}{
		{
			name:       "valid field with correct type",
			fieldName:  "symbol",
			targetType: subscriber.FieldTypeString,
			wantErr:    false,
			expected:   "600000",
		},
		{
			name:       "valid field with wrong type",
			fieldName:  "symbol",
			targetType: subscriber.FieldTypeInt,
			wantErr:    true,
			errCode:    subscriber.ErrInvalidFieldType,
		},
		{
			name:       "nonexistent field",
			fieldName:  "nonexistent",
			targetType: subscriber.FieldTypeString,
			wantErr:    true,
			errCode:    subscriber.ErrFieldNotFound,
		},
		{
			name:       "missing required field",
			fieldName:  "name", // 必填但未设置
			targetType: subscriber.FieldTypeString,
			wantErr:    true,
			errCode:    subscriber.ErrRequiredFieldMissing,
		},
		{
			name:       "missing optional field",
			fieldName:  "change", // 可选且未设置
			targetType: subscriber.FieldTypeFloat64,
			wantErr:    false,
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := sd.GetFieldSafe(tt.fieldName, tt.targetType)

			if tt.wantErr {
				require.Error(t, err)
				if structErr, ok := err.(*subscriber.StructuredDataError); ok {
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
	err := subscriber.ValidateSchema(subscriber.StockDataSchema)
	require.NoError(t, err)

	// 验证模式中的所有字段定义
	for fieldName, fieldDef := range subscriber.StockDataSchema.Fields {
		err := subscriber.ValidateFieldDefinition(fieldName, fieldDef)
		require.NoError(t, err, "Field %s should be valid", fieldName)
	}

	// 验证字段顺序是否完整
	assert.NotEmpty(t, subscriber.StockDataSchema.FieldOrder)
	
	fieldOrderMap := make(map[string]bool)
	for _, fieldName := range subscriber.StockDataSchema.FieldOrder {
		fieldOrderMap[fieldName] = true
		_, exists := subscriber.StockDataSchema.Fields[fieldName]
		assert.True(t, exists, "Field %s in order should exist in fields", fieldName)
	}

	// 验证所有字段都在顺序中
	for fieldName := range subscriber.StockDataSchema.Fields {
		assert.True(t, fieldOrderMap[fieldName], "Field %s should be in field order", fieldName)
	}
}