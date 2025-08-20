package config

import (
	"errors"
	"time"
)

// Config 主配置结构
type Config struct {
	// 提供商配置
	Provider ProviderConfig `json:"provider"`

	// 订阅器配置
	Subscriber SubscriberConfig `json:"subscriber"`

	// 日志配置
	Logger LoggerConfig `json:"logger"`
}

// ProviderConfig 数据提供商配置
type ProviderConfig struct {
	Name       string        `json:"name"`        // 提供商名称 ("tencent")
	Timeout    time.Duration `json:"timeout"`     // 请求超时时间
	MaxRetries int           `json:"max_retries"` // 最大重试次数
	RateLimit  time.Duration `json:"rate_limit"`  // 请求间隔限制
	UserAgent  string        `json:"user_agent"`  // 用户代理
	BatchSize  int           `json:"batch_size"`  // 批处理大小
}

// SubscriberConfig 订阅器配置
type SubscriberConfig struct {
	MaxSubscriptions   int           `json:"max_subscriptions"`    // 最大订阅数
	DefaultInterval    time.Duration `json:"default_interval"`     // 默认订阅间隔
	MinInterval        time.Duration `json:"min_interval"`         // 最小订阅间隔
	MaxInterval        time.Duration `json:"max_interval"`         // 最大订阅间隔
	EventChannelBuffer int           `json:"event_channel_buffer"` // 事件通道缓冲区大小
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level      string `json:"level"`       // 日志级别 (debug, info, warn, error)
	Output     string `json:"output"`      // 输出方式 (console, file)
	Filename   string `json:"filename"`    // 日志文件名
	MaxSize    int    `json:"max_size"`    // 最大文件大小(MB)
	MaxBackups int    `json:"max_backups"` // 最大备份数
	MaxAge     int    `json:"max_age"`     // 最大保存天数
}

// Default 返回默认配置
func Default() *Config {
	return &Config{
		Provider: ProviderConfig{
			Name:       "tencent",
			Timeout:    15 * time.Second,
			MaxRetries: 3,
			RateLimit:  200 * time.Millisecond,
			UserAgent:  "StockSub/1.0",
			BatchSize:  50,
		},
		Subscriber: SubscriberConfig{
			MaxSubscriptions:   100,
			DefaultInterval:    5 * time.Second,
			MinInterval:        1 * time.Second,
			MaxInterval:        1 * time.Hour,
			EventChannelBuffer: 1000,
		},
		Logger: LoggerConfig{
			Level:      "info",
			Output:     "console",
			Filename:   "stocksub.log",
			MaxSize:    10,
			MaxBackups: 5,
			MaxAge:     30,
		},
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Provider.Name == "" {
		return errors.New("provider name cannot be empty")
	}

	if c.Provider.Timeout <= 0 {
		return errors.New("provider timeout must be positive")
	}

	if c.Provider.MaxRetries < 0 {
		return errors.New("provider max_retries cannot be negative")
	}

	if c.Provider.RateLimit < 0 {
		return errors.New("provider rate_limit cannot be negative")
	}

	if c.Subscriber.MaxSubscriptions <= 0 {
		return errors.New("max_subscriptions must be positive")
	}

	if c.Subscriber.MinInterval <= 0 {
		return errors.New("min_interval must be positive")
	}

	if c.Subscriber.MaxInterval <= c.Subscriber.MinInterval {
		return errors.New("max_interval must be greater than min_interval")
	}

	if c.Subscriber.EventChannelBuffer <= 0 {
		return errors.New("event_channel_buffer must be positive")
	}

	return nil
}

// SetProviderTimeout 设置提供商超时时间
func (c *Config) SetProviderTimeout(timeout time.Duration) *Config {
	c.Provider.Timeout = timeout
	return c
}

// SetRateLimit 设置请求频率限制
func (c *Config) SetRateLimit(limit time.Duration) *Config {
	c.Provider.RateLimit = limit
	return c
}

// SetDefaultInterval 设置默认订阅间隔
func (c *Config) SetDefaultInterval(interval time.Duration) *Config {
	c.Subscriber.DefaultInterval = interval
	return c
}

// SetMaxSubscriptions 设置最大订阅数
func (c *Config) SetMaxSubscriptions(max int) *Config {
	c.Subscriber.MaxSubscriptions = max
	return c
}

// SetLogLevel 设置日志级别
func (c *Config) SetLogLevel(level string) *Config {
	c.Logger.Level = level
	return c
}
