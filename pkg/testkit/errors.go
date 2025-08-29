package testkit

import (
	"errors"
	"time"

	"stocksub/pkg/cache"
	"stocksub/pkg/error"
	"stocksub/pkg/storage"
)

const (
	// ErrProviderError 表示数据提供者返回了一个通用错误。
	ErrProviderError error.ErrorCode = "PROVIDER_ERROR"
	// ErrProviderTimeout 表示数据提供者操作超时。
	ErrProviderTimeout error.ErrorCode = "PROVIDER_TIMEOUT"
	// ErrProviderNotFound 表示数据提供者未找到请求的资源。
	ErrProviderNotFound error.ErrorCode = "PROVIDER_NOT_FOUND"
	// ErrProviderAuth 表示数据提供者认证失败。
	ErrProviderAuth error.ErrorCode = "PROVIDER_AUTH"
	// ErrConcurrencyLimit 表示超出了并发限制。
	ErrConcurrencyLimit error.ErrorCode = "CONCURRENCY_LIMIT"
	// ErrDeadlock 表示检测到死锁。
	ErrDeadlock error.ErrorCode = "DEADLOCK"
	// ErrRaceCondition 表示检测到竞态条件。
	ErrRaceCondition error.ErrorCode = "RACE_CONDITION"
	// ErrConfigInvalid 表示配置无效。
	ErrConfigInvalid error.ErrorCode = "CONFIG_INVALID"
	// ErrConfigMissing 表示缺少必要的配置项。
	ErrConfigMissing error.ErrorCode = "CONFIG_MISSING"
	// ErrSystemResource 表示系统资源不足。
	ErrSystemResource error.ErrorCode = "SYSTEM_RESOURCE"
	// ErrSystemShutdown 表示系统正在关闭。
	ErrSystemShutdown error.ErrorCode = "SYSTEM_SHUTDOWN"
	// ErrInternalError 表示发生了未知的内部错误。
	ErrInternalError error.ErrorCode = "INTERNAL_ERROR"
)

// 预定义的常用错误实例
var (
	ErrConcurrencyExceeded  = NewTestKitError(ErrConcurrencyLimit, "concurrency limit exceeded")
	ErrInvalidConfiguration = NewTestKitError(ErrConfigInvalid, "invalid configuration")
	ErrShutdownInProgress   = NewTestKitError(ErrSystemShutdown, "system shutdown in progress")
)

// 它包含了错误代码、消息、可选的原始错误(cause)和附加上下文信息。
type TestKitError struct {
	error.BaseError
}

// NewTestKitError 创建一个新的 TestKitError。
func NewTestKitError(code error.ErrorCode, message string) *TestKitError {
	return &TestKitError{
		BaseError: *error.NewError(code, message),
	}
}

// ErrorHandler 定义了错误处理器的行为，用于实现复杂的错误处理逻辑，如重试。
type ErrorHandler interface {
	HandleError(ctx interface{}, err error.BaseError) *error.BaseError
	ShouldRetry(err error.BaseError) bool
	GetRetryDelay(attempt int, err error.BaseError) time.Duration
}

// RetryConfig 定义了重试操作的配置参数。
type RetryConfig struct {
	MaxAttempts    int               `json:"max_attempts"`    // 最大重试次数
	InitialDelay   time.Duration     `json:"initial_delay"`   // 初始延迟
	MaxDelay       time.Duration     `json:"max_delay"`       // 最大延迟
	BackoffFactor  float64           `json:"backoff_factor"`  // 退避因子，用于指数退避算法
	RetryableCodes []error.ErrorCode `json:"retryable_codes"` // 可重试的错误代码列表
}

// DefaultRetryConfig 是一个默认的重试配置实例。
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:   3,
	InitialDelay:  100 * time.Millisecond,
	MaxDelay:      5 * time.Second,
	BackoffFactor: 2.0,
	RetryableCodes: []error.ErrorCode{
		cache.ErrCacheTimeout,
		ErrProviderTimeout,
		storage.ErrStorageIO,
		ErrSystemResource,
	},
}

// IsRetryable 判断一个错误根据默认配置是否是可重试的。
func IsRetryable(err *error.BaseError) bool {
	var tkErr *error.BaseError
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
