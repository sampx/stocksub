package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConfig_Default_ReturnsValidConfig 测试默认配置是否正确
func TestConfig_Default_ReturnsValidConfig(t *testing.T) {
	cfg := Default()

	// 验证默认配置值
	assert.Equal(t, "tencent", cfg.Provider.Name)
	assert.Equal(t, 15*time.Second, cfg.Provider.Timeout)
	assert.Equal(t, 3, cfg.Provider.MaxRetries)
	assert.Equal(t, 200*time.Millisecond, cfg.Provider.RateLimit)
	assert.Equal(t, "StockSub/1.0", cfg.Provider.UserAgent)
	assert.Equal(t, 50, cfg.Provider.BatchSize)

	assert.Equal(t, 100, cfg.Subscriber.MaxSubscriptions)
	assert.Equal(t, 5*time.Second, cfg.Subscriber.DefaultInterval)
	assert.Equal(t, 1*time.Second, cfg.Subscriber.MinInterval)
	assert.Equal(t, 1*time.Hour, cfg.Subscriber.MaxInterval)
	assert.Equal(t, 1000, cfg.Subscriber.EventChannelBuffer)

	assert.Equal(t, "info", cfg.Logger.Level)
	assert.Equal(t, "console", cfg.Logger.Output)
	assert.Equal(t, "stocksub.log", cfg.Logger.Filename)
	assert.Equal(t, 10, cfg.Logger.MaxSize)
	assert.Equal(t, 5, cfg.Logger.MaxBackups)
	assert.Equal(t, 30, cfg.Logger.MaxAge)
}

// TestConfig_Validate_WithInvalidConfig_ReturnsError 测试配置验证功能
func TestConfig_Validate_WithInvalidConfig_ReturnsError(t *testing.T) {
	// 测试有效的默认配置
	cfg := Default()
	assert.NoError(t, cfg.Validate(), "默认配置应该是有效的")

	// 测试提供商名称为空的情况
	cfg = Default()
	cfg.Provider.Name = ""
	assert.Error(t, cfg.Validate(), "提供商名称为空时应该返回错误")

	// 测试提供商超时时间小于等于0的情况
	cfg = Default()
	cfg.Provider.Timeout = 0
	assert.Error(t, cfg.Validate(), "提供商超时时间小于等于0时应该返回错误")

	cfg.Provider.Timeout = -1 * time.Second
	assert.Error(t, cfg.Validate(), "提供商超时时间为负数时应该返回错误")

	// 测试提供商最大重试次数为负数的情况
	cfg = Default()
	cfg.Provider.MaxRetries = -1
	assert.Error(t, cfg.Validate(), "提供商最大重试次数为负数时应该返回错误")

	// 测试提供商请求频率限制为负数的情况
	cfg = Default()
	cfg.Provider.RateLimit = -1 * time.Second
	assert.Error(t, cfg.Validate(), "提供商请求频率限制为负数时应该返回错误")

	// 测试最大订阅数小于等于0的情况
	cfg = Default()
	cfg.Subscriber.MaxSubscriptions = 0
	assert.Error(t, cfg.Validate(), "最大订阅数小于等于0时应该返回错误")

	cfg.Subscriber.MaxSubscriptions = -1
	assert.Error(t, cfg.Validate(), "最大订阅数为负数时应该返回错误")

	// 测试最小订阅间隔小于等于0的情况
	cfg = Default()
	cfg.Subscriber.MinInterval = 0
	assert.Error(t, cfg.Validate(), "最小订阅间隔小于等于0时应该返回错误")

	cfg.Subscriber.MinInterval = -1 * time.Second
	assert.Error(t, cfg.Validate(), "最小订阅间隔为负数时应该返回错误")

	// 测试最大订阅间隔小于等于最小订阅间隔的情况
	cfg = Default()
	cfg.Subscriber.MaxInterval = cfg.Subscriber.MinInterval
	assert.Error(t, cfg.Validate(), "最大订阅间隔小于等于最小订阅间隔时应该返回错误")

	cfg.Subscriber.MaxInterval = cfg.Subscriber.MinInterval - 1*time.Second
	assert.Error(t, cfg.Validate(), "最大订阅间隔小于最小订阅间隔时应该返回错误")

	// 测试事件通道缓冲区大小小于等于0的情况
	cfg = Default()
	cfg.Subscriber.EventChannelBuffer = 0
	assert.Error(t, cfg.Validate(), "事件通道缓冲区大小小于等于0时应该返回错误")

	cfg.Subscriber.EventChannelBuffer = -1
	assert.Error(t, cfg.Validate(), "事件通道缓冲区大小为负数时应该返回错误")
}

// TestConfig_SetProviderTimeout_WithValidTimeout_ReturnsSameConfig 测试设置提供商超时时间的方法
func TestConfig_SetProviderTimeout_WithValidTimeout_ReturnsSameConfig(t *testing.T) {
	cfg := Default()
	newTimeout := 30 * time.Second
	result := cfg.SetProviderTimeout(newTimeout)

	// 验证返回的是同一个对象（支持链式调用）
	assert.Equal(t, cfg, result, "应该返回同一个配置对象以支持链式调用")

	// 验证超时时间已更新
	assert.Equal(t, newTimeout, cfg.Provider.Timeout, "提供商超时时间应该被正确更新")
}

// TestConfig_SetRateLimit_WithValidRateLimit_ReturnsSameConfig 测试设置请求频率限制的方法
func TestConfig_SetRateLimit_WithValidRateLimit_ReturnsSameConfig(t *testing.T) {
	cfg := Default()
	newRateLimit := 500 * time.Millisecond
	result := cfg.SetRateLimit(newRateLimit)

	// 验证返回的是同一个对象（支持链式调用）
	assert.Equal(t, cfg, result, "应该返回同一个配置对象以支持链式调用")

	// 验证请求频率限制已更新
	assert.Equal(t, newRateLimit, cfg.Provider.RateLimit, "提供商请求频率限制应该被正确更新")
}

// TestConfig_SetDefaultInterval_WithValidInterval_ReturnsSameConfig 测试设置默认订阅间隔的方法
func TestConfig_SetDefaultInterval_WithValidInterval_ReturnsSameConfig(t *testing.T) {
	cfg := Default()
	newInterval := 10 * time.Second
	result := cfg.SetDefaultInterval(newInterval)

	// 验证返回的是同一个对象（支持链式调用）
	assert.Equal(t, cfg, result, "应该返回同一个配置对象以支持链式调用")

	// 验证默认订阅间隔已更新
	assert.Equal(t, newInterval, cfg.Subscriber.DefaultInterval, "默认订阅间隔应该被正确更新")
}

// TestConfig_SetMaxSubscriptions_WithValidMax_ReturnsSameConfig 测试设置最大订阅数的方法
func TestConfig_SetMaxSubscriptions_WithValidMax_ReturnsSameConfig(t *testing.T) {
	cfg := Default()
	newMax := 200
	result := cfg.SetMaxSubscriptions(newMax)

	// 验证返回的是同一个对象（支持链式调用）
	assert.Equal(t, cfg, result, "应该返回同一个配置对象以支持链式调用")

	// 验证最大订阅数已更新
	assert.Equal(t, newMax, cfg.Subscriber.MaxSubscriptions, "最大订阅数应该被正确更新")
}

// TestConfig_SetLogLevel_WithValidLevel_ReturnsSameConfig 测试设置日志级别的方法
func TestConfig_SetLogLevel_WithValidLevel_ReturnsSameConfig(t *testing.T) {
	cfg := Default()
	newLevel := "debug"
	result := cfg.SetLogLevel(newLevel)

	// 验证返回的是同一个对象（支持链式调用）
	assert.Equal(t, cfg, result, "应该返回同一个配置对象以支持链式调用")

	// 验证日志级别已更新
	assert.Equal(t, newLevel, cfg.Logger.Level, "日志级别应该被正确更新")
}
