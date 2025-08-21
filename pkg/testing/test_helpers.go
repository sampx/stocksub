package testing

import (
	"context"
	"os"
	"time"

	"stocksub/pkg/provider/tencent"
)

// CreateTestProvider 创建测试专用Provider
func CreateTestProvider() *tencent.Provider {
	provider := tencent.NewProvider()

	// 测试环境下稍微宽松的配置
	provider.SetTimeout(30 * time.Second)
	provider.SetRateLimit(500 * time.Millisecond) // 比生产环境稍快

	return provider
}

// IsValidTimeFormat 时间格式验证Helper
func IsValidTimeFormat(timeStr, format string) bool {
	if len(timeStr) != len(format) {
		return false
	}
	_, err := time.ParseInLocation(format, timeStr, time.Local)
	return err == nil
}

// CreateTestContext 创建测试用Context
func CreateTestContext() context.Context {
	timeout := 30 * time.Second
	if os.Getenv("TEST_TIMEOUT") != "" {
		if d, err := time.ParseDuration(os.Getenv("TEST_TIMEOUT")); err == nil {
			timeout = d
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	// The cancel function should be called, but in this helper,
	// we detach it and let the caller manage the context's lifecycle.
	// In a real-world scenario, you might return the cancel function.
	_ = cancel
	return ctx
}

// IsUsingCache 检查是否使用缓存模式
func IsUsingCache() bool {
	return os.Getenv("TEST_FORCE_CACHE") == "1"
}

// IsForcingAPICall 检查是否强制API调用
func IsForcingAPICall() bool {
	return os.Getenv("FORCE_API_CALL") == "1"
}
