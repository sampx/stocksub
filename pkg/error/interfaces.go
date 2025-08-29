package error

import (
	"fmt"
	"time"
)

// ErrorCode 错误代码类型
type ErrorCode string

// BaseError 基础错误类型
type BaseError struct {
	Code      ErrorCode              `json:"code"`              // 错误的分类代码
	Message   string                 `json:"message"`           // 人类可读的错误信息
	Cause     error                  `json:"-"`                 // 导致此错误的原始错误
	Context   map[string]interface{} `json:"context,omitempty"` // 额外的上下文信息
	Timestamp time.Time              `json:"timestamp"`         // 错误发生的时间戳
	Stack     []string               `json:"stack,omitempty"`   // 错误发生时的调用栈
}

// NewError 创建新的基础错误
func NewError(code ErrorCode, message string) *BaseError {
	return &BaseError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// Error 实现 error 接口
func (e *BaseError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 支持错误包装
func (e *BaseError) Unwrap() error {
	return e.Cause
}

// Is 支持错误比较
func (e *BaseError) Is(target error) bool {
	if t, ok := target.(*BaseError); ok {
		return e.Code == t.Code
	}
	return false
}

// WithContext 为错误附加一个键值对形式的上下文信息。
func (e *BaseError) WithContext(key string, value interface{}) *BaseError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithStack 为错误附加调用栈信息。
func (e *BaseError) WithStack(stack []string) *BaseError {
	e.Stack = stack
	return e
}

// WrapError 包装现有错误
func WrapError(code ErrorCode, message string, cause error) *BaseError {
	return &BaseError{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}
