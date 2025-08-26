package core

import "time"

// Provider 数据提供商基础接口
// 所有数据提供商都必须实现此接口
type Provider interface {
	// Name 返回提供商名称，用于标识和日志记录
	Name() string

	// GetRateLimit 获取请求频率限制
	// 返回两次请求之间的最小间隔时间
	GetRateLimit() time.Duration

	// IsHealthy 检查提供商健康状态
	// 返回 true 表示提供商可以正常工作
	IsHealthy() bool
}

// Configurable 可配置接口
// 支持动态配置的提供商可以实现此接口
type Configurable interface {
	// SetRateLimit 设置请求频率限制
	SetRateLimit(limit time.Duration)

	// SetTimeout 设置请求超时时间
	SetTimeout(timeout time.Duration)

	// SetMaxRetries 设置最大重试次数
	SetMaxRetries(retries int)
}

// Closable 可关闭接口
// 需要清理资源的提供商应实现此接口
type Closable interface {
	// Close 关闭提供商，清理资源
	Close() error
}
