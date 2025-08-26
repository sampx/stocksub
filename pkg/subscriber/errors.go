package subscriber

// ErrorCode 错误代码类型
type ErrorCode string

// 错误代码常量
const (
	ErrInvalidFieldType      ErrorCode = "INVALID_FIELD_TYPE"
	ErrRequiredFieldMissing  ErrorCode = "REQUIRED_FIELD_MISSING"
	ErrFieldValidationFailed ErrorCode = "FIELD_VALIDATION_FAILED"
	ErrSchemaNotFound        ErrorCode = "SCHEMA_NOT_FOUND"
	ErrCSVHeaderMismatch     ErrorCode = "CSV_HEADER_MISMATCH"
	ErrFieldNotFound         ErrorCode = "FIELD_NOT_FOUND"
)

// StructuredDataError 结构化数据相关错误
type StructuredDataError struct {
	Code    ErrorCode `json:"code"`
	Field   string    `json:"field"`
	Message string    `json:"message"`
	Cause   error     `json:"cause,omitempty"`
}

// Error 实现 error 接口
func (e *StructuredDataError) Error() string {
	if e.Field != "" {
		return string(e.Code) + ": " + e.Field + " - " + e.Message
	}
	return string(e.Code) + ": " + e.Message
}

// NewStructuredDataError 创建新的结构化数据错误
func NewStructuredDataError(code ErrorCode, field, message string) *StructuredDataError {
	return &StructuredDataError{
		Code:    code,
		Field:   field,
		Message: message,
	}
}
