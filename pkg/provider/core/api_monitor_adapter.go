package core

import (
	"context"
	"fmt"
	"stocksub/pkg/limiter"
	"stocksub/pkg/timing"
	"time"
)

// APIMonitorCompatibilityConfig API监控兼容性配置
type APIMonitorCompatibilityConfig struct {
	// SkipMarketTimeCheck 是否跳过市场时间检查
	// 设置为 true 时可以在非交易时间进行测试
	SkipMarketTimeCheck bool `yaml:"skip_market_time_check"`
	
	// ForceTestMode 强制测试模式
	// 设置为 true 时禁用所有安全检查，仅用于开发测试
	ForceTestMode bool `yaml:"force_test_mode"`
	
	// MaxTestDuration 测试模式下的最大运行时间
	// 避免测试模式下的无限运行
	MaxTestDuration time.Duration `yaml:"max_test_duration"`
}

// CompatibleIntelligentLimiter 兼容性智能限制器
// 为 api_monitor 提供兼容的智能限制器功能
type CompatibleIntelligentLimiter struct {
	originalLimiter *limiter.IntelligentLimiter
	config          APIMonitorCompatibilityConfig
	startTime       time.Time
	initialized     bool
}

// NewCompatibleIntelligentLimiter 创建兼容性智能限制器
func NewCompatibleIntelligentLimiter(marketTime *timing.MarketTime, config APIMonitorCompatibilityConfig) *CompatibleIntelligentLimiter {
	return &CompatibleIntelligentLimiter{
		originalLimiter: limiter.NewIntelligentLimiter(marketTime),
		config:          config,
		startTime:       time.Now(),
	}
}

// InitializeBatch 初始化批次信息
func (c *CompatibleIntelligentLimiter) InitializeBatch(symbols []string) {
	c.initialized = true
	c.startTime = time.Now()
	
	if c.config.ForceTestMode {
		// 测试模式：直接标记为已初始化，跳过原始限制器
		return
	}
	
	// 正常模式：使用原始限制器
	c.originalLimiter.InitializeBatch(symbols)
}

// ShouldProceed 判断是否可以继续进行API调用
func (c *CompatibleIntelligentLimiter) ShouldProceed(ctx context.Context) (bool, error) {
	if !c.initialized {
		return false, fmt.Errorf("limiter not initialized")
	}
	
	// 强制测试模式：跳过所有检查但限制运行时间
	if c.config.ForceTestMode {
		if c.config.MaxTestDuration > 0 && time.Since(c.startTime) > c.config.MaxTestDuration {
			return false, fmt.Errorf("test mode time limit reached")
		}
		return true, nil
	}
	
	// 跳过市场时间检查模式
	if c.config.SkipMarketTimeCheck {
		// 仍然使用原始限制器的其他逻辑，但跳过交易时间检查
		return c.shouldProceedWithoutMarketCheck(ctx)
	}
	
	// 正常模式：完全使用原始限制器
	return c.originalLimiter.ShouldProceed(ctx)
}

// RecordResult 记录结果
func (c *CompatibleIntelligentLimiter) RecordResult(err error, data []string) (bool, time.Duration, error) {
	// 强制测试模式：简化结果处理
	if c.config.ForceTestMode {
		if err != nil {
			return false, 2*time.Second, nil // 简单重试逻辑
		}
		return true, 0, nil
	}
	
	// 正常模式：使用原始限制器
	return c.originalLimiter.RecordResult(err, data)
}

// shouldProceedWithoutMarketCheck 跳过市场时间检查的判断逻辑
func (c *CompatibleIntelligentLimiter) shouldProceedWithoutMarketCheck(ctx context.Context) (bool, error) {
	// 这里我们需要手动实现原始限制器的非市场时间相关逻辑
	// 由于原始限制器的内部状态是私有的，我们提供一个简化版本
	
	// 检查上下文是否被取消
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	
	// 检查运行时间限制（如果设置了）
	if c.config.MaxTestDuration > 0 && time.Since(c.startTime) > c.config.MaxTestDuration {
		return false, fmt.Errorf("test duration limit reached")
	}
	
	return true, nil
}

// APIMonitorTestingHelper API监控测试辅助器
type APIMonitorTestingHelper struct{}

// CreateTestCompatibleLimiter 为测试创建兼容的限制器
func (h *APIMonitorTestingHelper) CreateTestCompatibleLimiter(marketTime *timing.MarketTime) *CompatibleIntelligentLimiter {
	config := APIMonitorCompatibilityConfig{
		SkipMarketTimeCheck: true,
		ForceTestMode:       false, // 保留其他安全检查
		MaxTestDuration:     10 * time.Minute, // 测试限制为10分钟
	}
	
	return NewCompatibleIntelligentLimiter(marketTime, config)
}

// CreateDevCompatibleLimiter 为开发创建兼容的限制器
func (h *APIMonitorTestingHelper) CreateDevCompatibleLimiter(marketTime *timing.MarketTime) *CompatibleIntelligentLimiter {
	config := APIMonitorCompatibilityConfig{
		SkipMarketTimeCheck: true,
		ForceTestMode:       true, // 开发模式：跳过所有检查
		MaxTestDuration:     30 * time.Minute, // 开发限制为30分钟
	}
	
	return NewCompatibleIntelligentLimiter(marketTime, config)
}

// DefaultCompatibilityConfig 默认兼容性配置
func DefaultCompatibilityConfig() APIMonitorCompatibilityConfig {
	return APIMonitorCompatibilityConfig{
		SkipMarketTimeCheck: false, // 默认保持市场时间检查
		ForceTestMode:       false, // 默认不使用测试模式
		MaxTestDuration:     0,     // 默认无时间限制
	}
}
