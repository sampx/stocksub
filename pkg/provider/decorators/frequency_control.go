package decorators

import (
	"context"
	"fmt"
	"stocksub/pkg/core"
	"stocksub/pkg/limiter"
	"stocksub/pkg/provider"
	"stocksub/pkg/timing"
	"sync"
	"time"
)

// FrequencyControlProvider 频率控制装饰器
// 将 pkg/limiter 的智能限流逻辑适配为装饰器模式
type FrequencyControlProvider struct {
	*RealtimeStockBaseDecorator

	// 智能限流相关组件
	limiter    *limiter.IntelligentLimiter
	marketTime *timing.MarketTime

	// 配置参数
	minInterval time.Duration // 最小请求间隔
	maxRetries  int           // 最大重试次数

	// 运行时状态
	mu          sync.RWMutex
	lastRequest time.Time // 上次请求时间
	isActive    bool      // 是否激活限流
}

// FrequencyControlConfig 频率控制配置
type FrequencyControlConfig struct {
	MinInterval time.Duration `yaml:"min_interval"` // 最小请求间隔
	MaxRetries  int           `yaml:"max_retries"`  // 最大重试次数
	Enabled     bool          `yaml:"enabled"`      // 是否启用
}

// NewFrequencyControlProvider 创建频率控制装饰器
func NewFrequencyControlProvider(stockProvider provider.RealtimeStockProvider, config *FrequencyControlConfig) *FrequencyControlProvider {
	if config == nil {
		config = &FrequencyControlConfig{
			MinInterval: 200 * time.Millisecond, // 默认最小间隔200ms
			MaxRetries:  3,                      // 默认最大重试3次
			Enabled:     true,                   // 默认启用
		}
	}

	// 创建市场时间组件
	marketTime := timing.DefaultMarketTime()

	return &FrequencyControlProvider{
		RealtimeStockBaseDecorator: NewRealtimeStockBaseDecorator(stockProvider),
		limiter:                    limiter.NewIntelligentLimiter(marketTime),
		marketTime:                 marketTime,
		minInterval:                config.MinInterval,
		maxRetries:                 config.MaxRetries,
		isActive:                   config.Enabled,
		lastRequest:                time.Time{},
	}
}

// Name 返回装饰器名称
func (f *FrequencyControlProvider) Name() string {
	return fmt.Sprintf("FrequencyControl(%s)", f.stockProvider.Name())
}

// GetRateLimit 返回频率限制
func (f *FrequencyControlProvider) GetRateLimit() time.Duration {
	return f.minInterval
}

// IsHealthy 检查健康状态
func (f *FrequencyControlProvider) IsHealthy() bool {
	// 检查基础提供商健康状态和限流器状态
	return f.stockProvider.IsHealthy() && f.limiter.IsSafeToContinue()
}

// FetchStockData 实现带频率控制的股票数据获取
func (f *FrequencyControlProvider) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	if !f.isActive {
		// 如果频率控制未激活，直接调用基础提供商
		return f.stockProvider.FetchStockData(ctx, symbols)
	}

	// 初始化限流器批次信息
	f.limiter.InitializeBatch(symbols)

	// 执行带智能重试的数据获取
	return f.fetchWithIntelligentRetry(ctx, symbols)
}

// FetchStockDataWithRaw 实现带频率控制的股票数据获取（包含原始数据）
func (f *FrequencyControlProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	if !f.isActive {
		return f.stockProvider.FetchStockDataWithRaw(ctx, symbols)
	}

	f.limiter.InitializeBatch(symbols)
	return f.fetchWithRawAndRetry(ctx, symbols)
}

// fetchWithIntelligentRetry 执行带智能重试的数据获取
func (f *FrequencyControlProvider) fetchWithIntelligentRetry(ctx context.Context, symbols []string) ([]core.StockData, error) {
	for attempt := 0; attempt <= f.maxRetries; attempt++ {
		// 检查是否可以继续
		shouldProceed, err := f.limiter.ShouldProceed(ctx)
		if !shouldProceed {
			return nil, fmt.Errorf("限流器阻止执行: %w", err)
		}

		// 执行频率控制
		if err := f.enforceFrequencyLimit(ctx); err != nil {
			return nil, err
		}

		// 调用基础提供商获取数据
		data, err := f.stockProvider.FetchStockData(ctx, symbols)

		// 将结果转换为字符串数组供限流器分析
		var dataStrings []string
		if err == nil && len(data) > 0 {
			dataStrings = make([]string, len(data))
			for i, d := range data {
				dataStrings[i] = fmt.Sprintf("%s:%.2f", d.Symbol, d.Price)
			}
		}

		// 记录结果并获取下一步行动
		shouldContinue, waitDuration, finalError := f.limiter.RecordResult(err, dataStrings)

		// 成功情况
		if err == nil {
			return data, nil
		}

		// 不应继续的情况
		if !shouldContinue {
			if finalError != nil {
				return nil, finalError
			}
			return nil, err
		}

		// 需要等待重试
		if waitDuration > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitDuration):
				// 等待结束，继续下一次重试
				continue
			}
		}
	}

	return nil, fmt.Errorf("已达到最大重试次数 (%d)", f.maxRetries)
}

// fetchWithRawAndRetry 执行带原始数据和智能重试的数据获取
func (f *FrequencyControlProvider) fetchWithRawAndRetry(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	for attempt := 0; attempt <= f.maxRetries; attempt++ {
		shouldProceed, err := f.limiter.ShouldProceed(ctx)
		if !shouldProceed {
			return nil, "", fmt.Errorf("限流器阻止执行: %w", err)
		}

		if err := f.enforceFrequencyLimit(ctx); err != nil {
			return nil, "", err
		}

		data, raw, err := f.stockProvider.FetchStockDataWithRaw(ctx, symbols)

		var dataStrings []string
		if err == nil && len(data) > 0 {
			dataStrings = make([]string, len(data))
			for i, d := range data {
				dataStrings[i] = fmt.Sprintf("%s:%.2f", d.Symbol, d.Price)
			}
		}

		shouldContinue, waitDuration, finalError := f.limiter.RecordResult(err, dataStrings)

		if err == nil {
			return data, raw, nil
		}

		if !shouldContinue {
			if finalError != nil {
				return nil, raw, finalError
			}
			return nil, raw, err
		}

		if waitDuration > 0 {
			select {
			case <-ctx.Done():
				return nil, raw, ctx.Err()
			case <-time.After(waitDuration):
				continue
			}
		}
	}

	return nil, "", fmt.Errorf("已达到最大重试次数 (%d)", f.maxRetries)
}

// enforceFrequencyLimit 执行频率限制
func (f *FrequencyControlProvider) enforceFrequencyLimit(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 计算距离上次请求的时间
	elapsed := time.Since(f.lastRequest)

	// 如果间隔不足，需要等待
	if elapsed < f.minInterval {
		waitTime := f.minInterval - elapsed

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// 等待完成
		}
	}

	// 更新最后请求时间
	f.lastRequest = time.Now()
	return nil
}

// SetMinInterval 设置最小请求间隔
func (f *FrequencyControlProvider) SetMinInterval(interval time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.minInterval = interval
}

// SetMaxRetries 设置最大重试次数
func (f *FrequencyControlProvider) SetMaxRetries(retries int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maxRetries = retries
}

// SetEnabled 设置是否启用频率控制
func (f *FrequencyControlProvider) SetEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isActive = enabled
}

// GetStatus 获取频率控制状态
func (f *FrequencyControlProvider) GetStatus() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	status := f.limiter.GetStatus()
	status["decorator_type"] = "FrequencyControl"
	status["min_interval"] = f.minInterval.String()
	status["max_retries"] = f.maxRetries
	status["is_active"] = f.isActive
	status["last_request"] = f.lastRequest
	status["base_provider"] = f.stockProvider.Name()

	return status
}

// Reset 重置频率控制状态（测试用）
func (f *FrequencyControlProvider) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastRequest = time.Time{}
	f.limiter.Reset()
}
