package storage

import (
	"stocksub/pkg/error"
)

const (
	// ErrStorageFull 表示存储空间已满。
	ErrStorageFull error.ErrorCode = "STORAGE_FULL"
	// ErrStorageIO 表示发生了存储I/O错误。
	ErrStorageIO error.ErrorCode = "STORAGE_IO"
	// ErrStorageCorrupted 表示持久化存储的数据已损坏。
	ErrStorageCorrupted error.ErrorCode = "STORAGE_CORRUPTED"
	// ErrStoragePermission 表示存储权限不足。
	ErrStoragePermission error.ErrorCode = "STORAGE_PERMISSION"
	// ErrSerializeFailed 表示序列化操作失败。
	ErrSerializeFailed error.ErrorCode = "SERIALIZE_FAILED"
	// ErrDeserializeFailed 表示反序列化操作失败。
	ErrDeserializeFailed error.ErrorCode = "DESERIALIZE_FAILED"
	// ErrInvalidFormat 表示数据格式无效。
	ErrInvalidFormat error.ErrorCode = "INVALID_FORMAT"
	// ErrResourceClosed 表示尝试访问已关闭的资源。
	ErrResourceClosed error.ErrorCode = "RESOURCE_CLOSED"

	ErrInvalidFieldType      error.ErrorCode = "INVALID_FIELD_TYPE"
	ErrRequiredFieldMissing  error.ErrorCode = "REQUIRED_FIELD_MISSING"
	ErrFieldValidationFailed error.ErrorCode = "FIELD_VALIDATION_FAILED"
	ErrSchemaNotFound        error.ErrorCode = "SCHEMA_NOT_FOUND"
	ErrCSVHeaderMismatch     error.ErrorCode = "CSV_HEADER_MISMATCH"
	ErrFieldNotFound         error.ErrorCode = "FIELD_NOT_FOUND"
)

var (
	ErrStorageQuotaExceeded = NewStorageError(ErrStorageFull, "storage quota exceeded")
	ErrSerializationFailed  = NewStorageError(ErrSerializeFailed, "data serialization failed")
)

type StorageError struct {
	error.BaseError
}

func NewStorageError(code error.ErrorCode, message string) *StorageError {
	return &StorageError{
		BaseError: *error.NewError(code, message),
	}
}

// StructuredDataError 结构化数据相关错误
type StructuredDataError struct {
	error.BaseError
}

// NewStructuredDataError 创建新的结构化数据错误
func NewStructuredDataError(code error.ErrorCode, field, message string) *StructuredDataError {
	return &StructuredDataError{
		BaseError: *error.NewError(code, message).WithContext("field", field),
	}
}
