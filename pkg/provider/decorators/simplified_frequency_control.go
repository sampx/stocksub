package decorators

import (
	"context"
	"fmt"
	"stocksub/pkg/core"
	"stocksub/pkg/limiter"
	"stocksub/pkg/provider"
	"stocksub/pkg/timing"
	"strings"
	"sync"
	"time"
)

// SimplifiedFrequencyControlProvider 简化频率控制装饰器
// 专门为股票API设计，集成智能限流器和交易时段控制
type SimplifiedFrequencyControlProvider struct {
	provider.RealtimeStockProvider
	*provider.BaseDecorator

	// 智能限流相关组件
	limiter    *limiter.IntelligentLimiter
	marketTime *timing.MarketTime

	// 配置参数
	config *SimplifiedFrequencyControlConfig

	// 运行时状态
	mu          sync.RWMutex
	lastRequest time.Time
	isActive    bool

	// IP封禁状态
	ipBanStatus IPBanStatus
}

// SimplifiedFrequencyControlConfig 简化频率控制配置
type SimplifiedFrequencyControlConfig struct {
	// 基础频率限制
	MinInterval time.Duration `yaml:"min_interval"` // 最小请求间隔
	MaxRetries  int           `yaml:"max_retries"`  // 最大重试次数
	Enabled     bool          `yaml:"enabled"`      // 是否启用

	// 交易时段相关
	MarketTimeAware  bool          `yaml:"market_time_aware"`  // 是否启用交易时段感知
	PreMarketBuffer  time.Duration `yaml:"pre_market_buffer"`  // 交易开始前缓冲时间
	PostMarketBuffer time.Duration `yaml:"post_market_buffer"` // 交易结束后缓冲时间

	// IP封禁处理
	IPBanRetryInterval time.Duration `yaml:"ip_ban_retry_interval"` // IP封禁后重试间隔
	IPBanRetryMax      int           `yaml:"ip_ban_retry_max"`      // IP封禁最大重试次数
}

// IPBanStatus IP封禁状态
type IPBanStatus struct {
	IsBanned     bool      // 是否被封禁
	BanStartTime time.Time // 封禁开始时间
	RetryCount   int       // 重试次数
}

// DefaultSimplifiedFrequencyControlConfig 默认简化频率控制配置
func DefaultSimplifiedFrequencyControlConfig() *SimplifiedFrequencyControlConfig {
	return &SimplifiedFrequencyControlConfig{
		MinInterval:        200 * time.Millisecond, // 默认最小间隔200ms
		MaxRetries:         3,                      // 默认最大重试3次
		Enabled:            true,                   // 默认启用
		MarketTimeAware:    true,                   // 默认启用交易时段感知
		PreMarketBuffer:    5 * time.Minute,        // 交易前5分钟缓冲
		PostMarketBuffer:   10 * time.Minute,       // 交易后10分钟缓冲
		IPBanRetryInterval: 5 * time.Minute,        // IP封禁后5分钟重试
		IPBanRetryMax:      3,                      // IP封禁最大重试3次
	}
}

// NewSimplifiedFrequencyControlProvider 创建简化频率控制装饰器
func NewSimplifiedFrequencyControlProvider(stockProvider provider.RealtimeStockProvider, config *SimplifiedFrequencyControlConfig) *SimplifiedFrequencyControlProvider {
	if config == nil {
		config = DefaultSimplifiedFrequencyControlConfig()
	}

	// 创建市场时间组件
	marketTime := timing.DefaultMarketTime()

	return &SimplifiedFrequencyControlProvider{
		RealtimeStockProvider: stockProvider,
		BaseDecorator:         provider.NewBaseDecorator(stockProvider),
		limiter:               limiter.NewIntelligentLimiter(marketTime),
		marketTime:            marketTime,
		config:                config,
		isActive:              config.Enabled,
		lastRequest:           time.Time{},
		ipBanStatus:           IPBanStatus{},
	}
}

// Name 返回装饰器名称
func (s *SimplifiedFrequencyControlProvider) Name() string {
	return fmt.Sprintf("SimplifiedFrequencyControl(%s)", s.RealtimeStockProvider.Name())
}

// GetRateLimit 返回频率限制
func (s *SimplifiedFrequencyControlProvider) GetRateLimit() time.Duration {
	return s.config.MinInterval
}

// IsHealthy 检查健康状态
func (s *SimplifiedFrequencyControlProvider) IsHealthy() bool {
	// 检查基础提供商健康状态和限流器状态
	return s.RealtimeStockProvider.IsHealthy() && s.limiter.IsSafeToContinue()
}

// FetchStockData 实现带简化频率控制的股票数据获取
func (s *SimplifiedFrequencyControlProvider) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	if !s.isActive {
		// 如果频率控制未激活，直接调用基础提供商
		return s.RealtimeStockProvider.FetchStockData(ctx, symbols)
	}

	// 执行带智能重试的数据获取
	return s.fetchWithSimplifiedRetry(ctx, symbols)
}

// FetchStockDataWithRaw 实现带简化频率控制的股票数据获取（包含原始数据）
func (s *SimplifiedFrequencyControlProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	if !s.isActive {
		return s.RealtimeStockProvider.FetchStockDataWithRaw(ctx, symbols)
	}

	return s.fetchWithRawAndSimplifiedRetry(ctx, symbols)
}

// fetchWithSimplifiedRetry 执行带简化智能重试的数据获取
func (s *SimplifiedFrequencyControlProvider) fetchWithSimplifiedRetry(ctx context.Context, symbols []string) ([]core.StockData, error) {
	// 初始化限流器批次信息
	s.limiter.InitializeBatch(symbols)

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		// 检查是否可以继续
		if !s.canProceed(ctx) {
			return nil, fmt.Errorf("简化频率控制阻止执行")
		}

		// 执行频率控制
		if err := s.enforceSimplifiedFrequencyLimit(ctx); err != nil {
			return nil, err
		}

		// 调用基础提供商获取数据
		data, err := s.RealtimeStockProvider.FetchStockData(ctx, symbols)

		// 处理IP封禁检测
		if err != nil && s.isIPBanError(err) {
			s.handleIPBan()
			continue
		}

		// 将结果转换为字符串数组供限流器分析
		var dataStrings []string
		if err == nil && len(data) > 0 {
			dataStrings = make([]string, len(data))
			for i, d := range data {
				dataStrings[i] = fmt.Sprintf("%s:%.2f", d.Symbol, d.Price)
			}
		}

		// 记录结果并获取下一步行动
		shouldContinue, waitDuration, finalError := s.limiter.RecordResult(err, dataStrings)

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

	return nil, fmt.Errorf("已达到最大重试次数 (%d)", s.config.MaxRetries)
}

// fetchWithRawAndSimplifiedRetry 执行带原始数据和简化智能重试的数据获取
func (s *SimplifiedFrequencyControlProvider) fetchWithRawAndSimplifiedRetry(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	// 初始化限流器批次信息
	s.limiter.InitializeBatch(symbols)

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		if !s.canProceed(ctx) {
			return nil, "", fmt.Errorf("简化频率控制阻止执行")
		}

		if err := s.enforceSimplifiedFrequencyLimit(ctx); err != nil {
			return nil, "", err
		}

		data, raw, err := s.RealtimeStockProvider.FetchStockDataWithRaw(ctx, symbols)

		// 处理IP封禁检测
		if err != nil && s.isIPBanError(err) {
			s.handleIPBan()
			continue
		}

		var dataStrings []string
		if err == nil && len(data) > 0 {
			dataStrings = make([]string, len(data))
			for i, d := range data {
				dataStrings[i] = fmt.Sprintf("%s:%.2f", d.Symbol, d.Price)
			}
		}

		shouldContinue, waitDuration, finalError := s.limiter.RecordResult(err, dataStrings)

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

	return nil, "", fmt.Errorf("已达到最大重试次数 (%d)", s.config.MaxRetries)
}

// canProceed 检查是否可以继续执行请求
func (s *SimplifiedFrequencyControlProvider) canProceed(ctx context.Context) bool {
	// 检查IP封禁状态
	if s.isIPBanned() {
		return false
	}

	// 检查交易时段（如果启用）
	if s.config.MarketTimeAware && !s.isInTradingWindow() {
		return false
	}

	// 如果禁用了交易时段感知，直接返回true（跳过智能限流器的交易时间检查）
	if !s.config.MarketTimeAware {
		return true
	}

	// 检查智能限流器（只有在启用交易时段感知时才检查）
	if s.limiter != nil {
		shouldProceed, _ := s.limiter.ShouldProceed(ctx)
		return shouldProceed
	}

	return true
}

// isInTradingWindow 检查是否在交易时段窗口内
func (s *SimplifiedFrequencyControlProvider) isInTradingWindow() bool {
	// 直接使用marketTime的IsTradingTime方法，它已经实现了完整的交易时间检查
	if !s.marketTime.IsTradingTime() {
		return false
	}

	// 应用缓冲时间检查
	now := s.marketTime.Now()

	// 检查是否在缓冲时间范围内
	tradingStart := time.Date(now.Year(), now.Month(), now.Day(), 9, 13, 30, 0, now.Location())
	tradingEnd := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 10, 0, now.Location())

	// 应用缓冲时间
	tradingStart = tradingStart.Add(-s.config.PreMarketBuffer)
	tradingEnd = tradingEnd.Add(s.config.PostMarketBuffer)

	return now.After(tradingStart) && now.Before(tradingEnd)
}

// isIPBanError 检测是否为IP封禁错误
func (s *SimplifiedFrequencyControlProvider) isIPBanError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	// 常见的IP封禁错误关键词
	ipBanKeywords := []string{
		"ip ban", "banned", "forbidden", "403", "blocked",
		"rate limit exceeded", "too many requests", "throttle",
	}

	for _, keyword := range ipBanKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

// handleIPBan 处理IP封禁情况
func (s *SimplifiedFrequencyControlProvider) handleIPBan() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ipBanStatus.IsBanned = true
	s.ipBanStatus.BanStartTime = time.Now()
	s.ipBanStatus.RetryCount++
}

// isIPBanned 检查是否处于IP封禁状态
func (s *SimplifiedFrequencyControlProvider) isIPBanned() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ipBanStatus.IsBanned {
		return false
	}

	// 检查是否超过重试次数
	if s.ipBanStatus.RetryCount >= s.config.IPBanRetryMax {
		return true // 永久封禁
	}

	// 检查是否过了重试间隔
	elapsed := time.Since(s.ipBanStatus.BanStartTime)
	if elapsed >= s.config.IPBanRetryInterval {
		// 重试间隔已过，重置封禁状态
		s.ipBanStatus.IsBanned = false
		return false
	}

	return true
}

// enforceSimplifiedFrequencyLimit 执行简化频率限制
func (s *SimplifiedFrequencyControlProvider) enforceSimplifiedFrequencyLimit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 计算距离上次请求的时间
	elapsed := time.Since(s.lastRequest)

	// 如果间隔不足，需要等待
	if elapsed < s.config.MinInterval {
		waitTime := s.config.MinInterval - elapsed

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// 等待完成
		}
	}

	// 更新最后请求时间
	s.lastRequest = time.Now()
	return nil
}

// SetEnabled 设置是否启用简化频率控制
func (s *SimplifiedFrequencyControlProvider) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isActive = enabled
}

// GetStatus 获取简化频率控制状态
func (s *SimplifiedFrequencyControlProvider) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limiterStatus := s.limiter.GetStatus()

	return map[string]interface{}{
		"decorator_type":    "SimplifiedFrequencyControl",
		"base_provider":     s.RealtimeStockProvider.Name(),
		"enabled":           s.isActive,
		"min_interval":      s.config.MinInterval.String(),
		"max_retries":       s.config.MaxRetries,
		"last_request":      s.lastRequest,
		"market_time_aware": s.config.MarketTimeAware,
		"in_trading_window": s.isInTradingWindow(),
		"ip_ban_status": map[string]interface{}{
			"is_banned":      s.ipBanStatus.IsBanned,
			"ban_start_time": s.ipBanStatus.BanStartTime,
			"retry_count":    s.ipBanStatus.RetryCount,
		},
		"limiter_status": limiterStatus,
	}
}

// Reset 重置简化频率控制状态（测试用）
func (s *SimplifiedFrequencyControlProvider) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastRequest = time.Time{}
	s.ipBanStatus = IPBanStatus{}
	s.limiter.Reset()
}
