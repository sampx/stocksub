package subscriber

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// isValidFieldType 验证字段类型是否匹配
func isValidFieldType(value interface{}, expectedType FieldType) bool {
	if value == nil {
		return true // nil 值总是有效的（可选字段）
	}

	switch expectedType {
	case FieldTypeString:
		_, ok := value.(string)
		return ok
	case FieldTypeInt:
		switch value.(type) {
		case int, int8, int16, int32, int64:
			return true
		default:
			return false
		}
	case FieldTypeFloat64:
		switch value.(type) {
		case float32, float64:
			return true
		default:
			return false
		}
	case FieldTypeBool:
		_, ok := value.(bool)
		return ok
	case FieldTypeTime:
		_, ok := value.(time.Time)
		return ok
	default:
		return false
	}
}

// ValidateSchema 验证数据模式定义的完整性
func ValidateSchema(schema *DataSchema) error {
	if schema == nil {
		return NewStructuredDataError(ErrSchemaNotFound, "", "schema cannot be nil")
	}

	if schema.Name == "" {
		return NewStructuredDataError(ErrSchemaNotFound, "", "schema name cannot be empty")
	}

	if len(schema.Fields) == 0 {
		return NewStructuredDataError(ErrSchemaNotFound, "", "schema must have at least one field")
	}

	// 验证字段顺序是否包含所有字段
	if len(schema.FieldOrder) > 0 {
		fieldOrderMap := make(map[string]bool)
		for _, fieldName := range schema.FieldOrder {
			fieldOrderMap[fieldName] = true
			if _, exists := schema.Fields[fieldName]; !exists {
				return NewStructuredDataError(ErrFieldNotFound, fieldName, "field in order not found in schema fields")
			}
		}

		// 检查是否有字段没有在顺序中
		for fieldName := range schema.Fields {
			if !fieldOrderMap[fieldName] {
				return NewStructuredDataError(ErrFieldNotFound, fieldName, "field not specified in field order")
			}
		}
	}

	// 验证每个字段定义
	for fieldName, fieldDef := range schema.Fields {
		if err := ValidateFieldDefinition(fieldName, fieldDef); err != nil {
			return err
		}
	}

	return nil
}

// ValidateFieldDefinition 验证字段定义的正确性
func ValidateFieldDefinition(fieldName string, fieldDef *FieldDefinition) error {
	if fieldDef == nil {
		return NewStructuredDataError(ErrFieldNotFound, fieldName, "field definition cannot be nil")
	}

	if fieldDef.Name == "" {
		return NewStructuredDataError(ErrInvalidFieldType, fieldName, "field name cannot be empty")
	}

	if fieldDef.Name != fieldName {
		return NewStructuredDataError(ErrInvalidFieldType, fieldName, "field name mismatch")
	}

	// 验证字段类型
	if fieldDef.Type < FieldTypeString || fieldDef.Type > FieldTypeTime {
		return NewStructuredDataError(ErrInvalidFieldType, fieldName, "invalid field type")
	}

	// 验证默认值类型
	if fieldDef.DefaultValue != nil {
		if !isValidFieldType(fieldDef.DefaultValue, fieldDef.Type) {
			return NewStructuredDataError(ErrInvalidFieldType, fieldName, "default value type mismatch")
		}
	}

	// 验证必填字段是否有默认值
	if fieldDef.Required && fieldDef.DefaultValue == nil {
		// 注意：必填字段可以没有默认值，这样可以强制用户提供值
	}

	return nil
}

// ValidateFieldValue 验证单个字段值
func ValidateFieldValue(fieldName string, value interface{}, fieldDef *FieldDefinition) error {
	if fieldDef == nil {
		return NewStructuredDataError(ErrFieldNotFound, fieldName, "field definition not found")
	}

	// 检查必填字段
	if fieldDef.Required && value == nil {
		return NewStructuredDataError(ErrRequiredFieldMissing, fieldName, "required field missing")
	}

	// 如果值为 nil 且不是必填字段，则有效
	if value == nil {
		return nil
	}

	// 类型验证
	if !isValidFieldType(value, fieldDef.Type) {
		return NewStructuredDataError(ErrInvalidFieldType, fieldName, fmt.Sprintf("expected %s, got %T", fieldDef.Type.String(), value))
	}

	// 范围验证（数值类型）
	if err := validateValueRange(fieldName, value, fieldDef); err != nil {
		return err
	}

	// 自定义验证
	if fieldDef.Validator != nil {
		if err := fieldDef.Validator(value); err != nil {
			return NewStructuredDataError(ErrFieldValidationFailed, fieldName, err.Error())
		}
	}

	return nil
}

// validateValueRange 验证数值范围（可扩展用于不同字段类型的范围验证）
func validateValueRange(fieldName string, value interface{}, fieldDef *FieldDefinition) error {
	switch fieldDef.Type {
	case FieldTypeFloat64:
		if f, ok := value.(float64); ok {
			// 检查是否为 NaN 或无穷大
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return NewStructuredDataError(ErrInvalidFieldType, fieldName, "value cannot be NaN or Infinity")
			}
			// 可以添加更多的范围检查
			if fieldName == "price" && f < 0 {
				return NewStructuredDataError(ErrFieldValidationFailed, fieldName, "price cannot be negative")
			}
		}
	case FieldTypeInt:
		// 可以添加整数范围验证
		if fieldName == "volume" || strings.Contains(fieldName, "volume") {
			var intVal int64
			switch v := value.(type) {
			case int:
				intVal = int64(v)
			case int64:
				intVal = v
			case int32:
				intVal = int64(v)
			}
			if intVal < 0 {
				return NewStructuredDataError(ErrFieldValidationFailed, fieldName, "volume cannot be negative")
			}
		}
	case FieldTypeString:
		if str, ok := value.(string); ok {
			// 字符串长度验证
			if fieldName == "symbol" && (len(str) < 2 || len(str) > 10) {
				return NewStructuredDataError(ErrFieldValidationFailed, fieldName, "symbol length must be between 2 and 10 characters")
			}
			if fieldName == "name" && len(str) > 50 {
				return NewStructuredDataError(ErrFieldValidationFailed, fieldName, "name cannot exceed 50 characters")
			}
		}
	}

	return nil
}
