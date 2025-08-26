package subscriber

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSchema(t *testing.T) {
	tests := []struct {
		name    string
		schema  *DataSchema
		wantErr bool
		errCode ErrorCode
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
		errCode   ErrorCode
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
		errCode   ErrorCode
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
