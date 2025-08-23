package core

import (
	"errors"
	"fmt"
	"time"
)

// ErrorCode 是一个字符串类型，用于表示 testkit 框架中所有预定义的错误类别。
type ErrorCode string

// 标准错误代码常量定义了 testkit 中可能出现的各种错误。
const (
	// ErrCacheTimeout 表示缓存操作超时。
	ErrCacheTimeout ErrorCode = "CACHE_TIMEOUT"
	// ErrCacheMiss 表示在缓存中未找到请求的条目。
	ErrCacheMiss ErrorCode = "CACHE_MISS"
	// ErrCacheFull 表示缓存已满，无法添加新条目。
	ErrCacheFull ErrorCode = "CACHE_FULL"
	// ErrCacheCorrupted 表示缓存数据已损坏。
	ErrCacheCorrupted ErrorCode = "CACHE_CORRUPTED"

	// ErrStorageFull 表示存储空间已满。
	ErrStorageFull ErrorCode = "STORAGE_FULL"
	// ErrStorageIO 表示发生了存储I/O错误。
	ErrStorageIO ErrorCode = "STORAGE_IO"
	// ErrStorageCorrupted 表示持久化存储的数据已损坏。
	ErrStorageCorrupted ErrorCode = "STORAGE_CORRUPTED"
	// ErrStoragePermission 表示存储权限不足。
	ErrStoragePermission ErrorCode = "STORAGE_PERMISSION"

	// ErrSerializeFailed 表示序列化操作失败。
	ErrSerializeFailed ErrorCode = "SERIALIZE_FAILED"
	// ErrDeserializeFailed 表示反序列化操作失败。
	ErrDeserializeFailed ErrorCode = "DESERIALIZE_FAILED"
	// ErrInvalidFormat 表示数据格式无效。
	ErrInvalidFormat ErrorCode = "INVALID_FORMAT"

	// ErrProviderError 表示数据提供者返回了一个通用错误。
	ErrProviderError ErrorCode = "PROVIDER_ERROR"
	// ErrProviderTimeout 表示数据提供者操作超时。
	ErrProviderTimeout ErrorCode = "PROVIDER_TIMEOUT"
	// ErrProviderNotFound 表示数据提供者未找到请求的资源。
	ErrProviderNotFound ErrorCode = "PROVIDER_NOT_FOUND"
	// ErrProviderAuth 表示数据提供者认证失败。
	ErrProviderAuth ErrorCode = "PROVIDER_AUTH"

	// ErrConcurrencyLimit 表示超出了并发限制。
	ErrConcurrencyLimit ErrorCode = "CONCURRENCY_LIMIT"
	// ErrDeadlock 表示检测到死锁。
	ErrDeadlock ErrorCode = "DEADLOCK"
	// ErrRaceCondition 表示检测到竞态条件。
	ErrRaceCondition ErrorCode = "RACE_CONDITION"

	// ErrConfigInvalid 表示配置无效。
	ErrConfigInvalid ErrorCode = "CONFIG_INVALID"
	// ErrConfigMissing 表示缺少必要的配置项。
	ErrConfigMissing ErrorCode = "CONFIG_MISSING"

	// ErrSystemResource 表示系统资源不足。
	ErrSystemResource ErrorCode = "SYSTEM_RESOURCE"
	// ErrSystemShutdown 表示系统正在关闭。
	ErrSystemShutdown ErrorCode = "SYSTEM_SHUTDOWN"
	// ErrInternalError 表示发生了未知的内部错误。
	ErrInternalError ErrorCode = "INTERNAL_ERROR"
	// ErrResourceClosed 表示尝试访问已关闭的资源。
	ErrResourceClosed ErrorCode = "RESOURCE_CLOSED"
)

// TestKitError 是 testkit 框架的自定义错误类型。
// 它包含了错误代码、消息、可选的原始错误(cause)和附加上下文信息。
type TestKitError struct {
	Code      ErrorCode              `json:"code"`              // 错误的分类代码
	Message   string                 `json:"message"`           // 人类可读的错误信息
	Cause     error                  `json:"-"`                 // 导致此错误的原始错误
	Context   map[string]interface{} `json:"context,omitempty"` // 额外的上下文信息
	Timestamp time.Time              `json:"timestamp"`         // 错误发生的时间戳
	Stack     []string               `json:"stack,omitempty"`   // 错误发生时的调用栈
}

// Error 实现了 Go 内置的 error 接口。
func (e *TestKitError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %s)", e.Code, e.Message, e.Cause.Error())
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 实现了 Go 1.13+ 的错误包装接口，允许访问被包装的原始错误(Cause)。
func (e *TestKitError) Unwrap() error {
	return e.Cause
}

// Is 实现了错误判断接口，用于判断一个错误是否与目标错误具有相同的错误代码。
func (e *TestKitError) Is(target error) bool {
	var tkErr *TestKitError
	if errors.As(target, &tkErr) {
		return e.Code == tkErr.Code
	}
	return false
}

// WithContext 为错误附加一个键值对形式的上下文信息。
func (e *TestKitError) WithContext(key string, value interface{}) *TestKitError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithStack 为错误附加调用栈信息。
func (e *TestKitError) WithStack(stack []string) *TestKitError {
	e.Stack = stack
	return e
}

// NewTestKitError 创建一个新的 TestKitError。
func NewTestKitError(code ErrorCode, message string) *TestKitError {
	return &TestKitError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// WrapError 将一个已有的 error 包装成一个新的 TestKitError。
func WrapError(code ErrorCode, message string, cause error) *TestKitError {
	return &TestKitError{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// 预定义的常用错误实例
var (
	ErrCacheMissNotFound    = NewTestKitError(ErrCacheMiss, "cache entry not found")
	ErrCacheTimeoutExceeded = NewTestKitError(ErrCacheTimeout, "cache operation timeout")
	ErrStorageQuotaExceeded = NewTestKitError(ErrStorageFull, "storage quota exceeded")
	ErrSerializationFailed  = NewTestKitError(ErrSerializeFailed, "data serialization failed")
	ErrConcurrencyExceeded  = NewTestKitError(ErrConcurrencyLimit, "concurrency limit exceeded")
	ErrInvalidConfiguration = NewTestKitError(ErrConfigInvalid, "invalid configuration")
	ErrShutdownInProgress   = NewTestKitError(ErrSystemShutdown, "system shutdown in progress")
)

// ErrorHandler 定义了错误处理器的行为，用于实现复杂的错误处理逻辑，如重试。
type ErrorHandler interface {
	HandleError(ctx interface{}, err error) error
	ShouldRetry(err error) bool
	GetRetryDelay(attempt int, err error) time.Duration
}

// RetryConfig 定义了重试操作的配置参数。
type RetryConfig struct {
	MaxAttempts    int           `json:"max_attempts"`    // 最大重试次数
	InitialDelay   time.Duration `json:"initial_delay"`   // 初始延迟
	MaxDelay       time.Duration `json:"max_delay"`       // 最大延迟
	BackoffFactor  float64       `json:"backoff_factor"`  // 退避因子，用于指数退避算法
	RetryableCodes []ErrorCode   `json:"retryable_codes"` // 可重试的错误代码列表
}

// DefaultRetryConfig 是一个默认的重试配置实例。
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:   3,
	InitialDelay:  100 * time.Millisecond,
	MaxDelay:      5 * time.Second,
	BackoffFactor: 2.0,
	RetryableCodes: []ErrorCode{
		ErrCacheTimeout,
		ErrProviderTimeout,
		ErrStorageIO,
		ErrSystemResource,
	},
}

// IsRetryable 判断一个错误根据默认配置是否是可重试的。
func IsRetryable(err error) bool {
	var tkErr *TestKitError
	if errors.As(err, &tkErr) {
		for _, code := range DefaultRetryConfig.RetryableCodes {
			if tkErr.Code == code {
				return true
			}
		}
	}
	return false
}

// CalculateRetryDelay 根据默认配置和当前重试次数计算下一次重试的延迟时间（指数退避）。
func CalculateRetryDelay(attempt int, config RetryConfig) time.Duration {
	if attempt <= 0 {
		return config.InitialDelay
	}

	delay := float64(config.InitialDelay)
	for i := 0; i < attempt; i++ {
		delay *= config.BackoffFactor
	}

	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	return time.Duration(delay)
}
