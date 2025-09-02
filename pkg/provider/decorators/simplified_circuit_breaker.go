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

	"github.com/sony/gobreaker"
)

// SimplifiedCircuitBreakerProvider 简化熔断器装饰器
// 专门为股票API设计，集成熔断器和交易时段感知
type SimplifiedCircuitBreakerProvider struct {
	provider.RealtimeStockProvider
	*provider.BaseDecorator

	// 熔断器组件
	cb     *gobreaker.CircuitBreaker
	config *SimplifiedCircuitBreakerConfig

	// 智能限流器（用于交易时段感知）
	limiter    *limiter.IntelligentLimiter
	marketTime *timing.MarketTime

	// 统计信息
	mu    sync.RWMutex
	stats SimplifiedCircuitBreakerStats
}

// SimplifiedCircuitBreakerConfig 简化熔断器配置
type SimplifiedCircuitBreakerConfig struct {
	Name        string        `yaml:"name"`          // 熔断器名称
	MaxRequests uint32        `yaml:"max_requests"`  // 半开状态下的最大请求数
	Interval    time.Duration `yaml:"interval"`      // 统计窗口时间
	Timeout     time.Duration `yaml:"timeout"`       // 熔断器打开后的超时时间
	ReadyToTrip uint32        `yaml:"ready_to_trip"` // 触发熔断的失败次数阈值
	Enabled     bool          `yaml:"enabled"`       // 是否启用熔断器

	// 交易时段相关
	MarketTimeAware bool `yaml:"market_time_aware"` // 是否启用交易时段感知
}

// SimplifiedCircuitBreakerStats 简化熔断器统计信息
type SimplifiedCircuitBreakerStats struct {
	TotalRequests     int64     `json:"total_requests"`
	SuccessfulRequest int64     `json:"successful_requests"`
	FailedRequests    int64     `json:"failed_requests"`
	LastFailure       time.Time `json:"last_failure"`
	IPBanCount        int64     `json:"ip_ban_count"` // IP封禁统计
}

// DefaultSimplifiedCircuitBreakerConfig 默认简化熔断器配置
func DefaultSimplifiedCircuitBreakerConfig() *SimplifiedCircuitBreakerConfig {
	return &SimplifiedCircuitBreakerConfig{
		Name:            "SimplifiedStockProvider",
		MaxRequests:     5,                // 半开状态允许5个请求
		Interval:        60 * time.Second, // 60秒统计窗口
		Timeout:         30 * time.Second, // 熔断30秒
		ReadyToTrip:     5,                // 5次失败触发熔断
		Enabled:         true,             // 默认启用
		MarketTimeAware: true,             // 默认启用交易时段感知
	}
}

// NewSimplifiedCircuitBreakerProvider 创建简化熔断器装饰器
func NewSimplifiedCircuitBreakerProvider(stockProvider provider.RealtimeStockProvider, config *SimplifiedCircuitBreakerConfig) *SimplifiedCircuitBreakerProvider {
	if config == nil {
		config = DefaultSimplifiedCircuitBreakerConfig()
	}

	// 创建市场时间组件
	marketTime := timing.DefaultMarketTime()

	// 创建 gobreaker 设置
	settings := gobreaker.Settings{
		Name:        config.Name,
		MaxRequests: config.MaxRequests,
		Interval:    config.Interval,
		Timeout:     config.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// 当连续失败次数达到阈值时触发熔断
			return counts.ConsecutiveFailures >= config.ReadyToTrip
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			// 状态变更回调
			fmt.Printf("简化熔断器 %s 状态从 %v 变更为 %v\n", name, from, to)
		},
	}

	return &SimplifiedCircuitBreakerProvider{
		RealtimeStockProvider: stockProvider,
		BaseDecorator:         provider.NewBaseDecorator(stockProvider),
		cb:                    gobreaker.NewCircuitBreaker(settings),
		config:                config,
		limiter:               limiter.NewIntelligentLimiter(marketTime),
		marketTime:            marketTime,
		stats:                 SimplifiedCircuitBreakerStats{},
	}
}

// Name 返回装饰器名称
func (s *SimplifiedCircuitBreakerProvider) Name() string {
	return fmt.Sprintf("SimplifiedCircuitBreaker(%s)", s.RealtimeStockProvider.Name())
}

// GetRateLimit 返回频率限制
func (s *SimplifiedCircuitBreakerProvider) GetRateLimit() time.Duration {
	return s.RealtimeStockProvider.GetRateLimit()
}

// IsHealthy 检查健康状态
func (s *SimplifiedCircuitBreakerProvider) IsHealthy() bool {
	if !s.config.Enabled {
		return s.RealtimeStockProvider.IsHealthy()
	}

	// 熔断器打开状态视为不健康
	state := s.cb.State()
	isHealthy := state != gobreaker.StateOpen && s.RealtimeStockProvider.IsHealthy()

	// 如果启用交易时段感知，检查是否在交易时段
	if s.config.MarketTimeAware && !s.isInTradingWindow() {
		isHealthy = false
	}

	return isHealthy
}

// FetchStockData 实现带简化熔断器的股票数据获取
func (s *SimplifiedCircuitBreakerProvider) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	if !s.config.Enabled {
		// 如果熔断器未启用，直接调用基础提供商
		return s.RealtimeStockProvider.FetchStockData(ctx, symbols)
	}

	// 检查交易时段（如果启用）
	if s.config.MarketTimeAware && !s.isInTradingWindow() {
		return nil, fmt.Errorf("当前不在交易时段内，简化熔断器阻止请求")
	}

	// 更新统计信息
	s.mu.Lock()
	s.stats.TotalRequests++
	s.mu.Unlock()

	// 通过熔断器执行请求
	result, err := s.cb.Execute(func() (interface{}, error) {
		return s.RealtimeStockProvider.FetchStockData(ctx, symbols)
	})

	// 处理结果和错误统计
	s.handleResult(err)

	if err != nil {
		return nil, err
	}

	// 类型断言转换结果
	data, ok := result.([]core.StockData)
	if !ok {
		err := fmt.Errorf("简化熔断器返回数据类型错误")
		s.handleResult(err)
		return nil, err
	}

	return data, nil
}

// FetchStockDataWithRaw 实现带简化熔断器的股票数据获取（包含原始数据）
func (s *SimplifiedCircuitBreakerProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	if !s.config.Enabled {
		return s.RealtimeStockProvider.FetchStockDataWithRaw(ctx, symbols)
	}

	if s.config.MarketTimeAware && !s.isInTradingWindow() {
		return nil, "", fmt.Errorf("当前不在交易时段内，简化熔断器阻止请求")
	}

	s.mu.Lock()
	s.stats.TotalRequests++
	s.mu.Unlock()

	// 定义包装结果结构
	type Result struct {
		Data []core.StockData
		Raw  string
	}

	// 通过熔断器执行请求
	result, err := s.cb.Execute(func() (interface{}, error) {
		data, raw, err := s.RealtimeStockProvider.FetchStockDataWithRaw(ctx, symbols)
		if err != nil {
			return nil, err
		}
		return Result{Data: data, Raw: raw}, nil
	})

	s.handleResult(err)

	if err != nil {
		return nil, "", err
	}

	// 类型断言转换结果
	res, ok := result.(Result)
	if !ok {
		err := fmt.Errorf("简化熔断器返回数据类型错误")
		s.handleResult(err)
		return nil, "", err
	}

	return res.Data, res.Raw, nil
}

// handleResult 处理请求结果和更新统计信息
func (s *SimplifiedCircuitBreakerProvider) handleResult(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		s.stats.FailedRequests++
		s.stats.LastFailure = time.Now()

		// 检查是否为IP封禁错误
		if s.isIPBanError(err) {
			s.stats.IPBanCount++
		}
	} else {
		s.stats.SuccessfulRequest++
	}
}

// isInTradingWindow 检查是否在交易时段窗口内
func (s *SimplifiedCircuitBreakerProvider) isInTradingWindow() bool {
	now := s.marketTime.Now()

	// 如果不在交易日，直接返回false
	if !s.marketTime.IsTradingDay(now) {
		return false
	}

	// 获取交易时间边界
	tradingStart := time.Date(now.Year(), now.Month(), now.Day(), 9, 13, 30, 0, now.Location())
	tradingEnd := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 10, 0, now.Location())

	return now.After(tradingStart) && now.Before(tradingEnd)
}

// isIPBanError 检测是否为IP封禁错误
func (s *SimplifiedCircuitBreakerProvider) isIPBanError(err error) bool {
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

// GetState 获取熔断器当前状态
func (s *SimplifiedCircuitBreakerProvider) GetState() gobreaker.State {
	return s.cb.State()
}

// GetCounts 获取熔断器计数信息
func (s *SimplifiedCircuitBreakerProvider) GetCounts() gobreaker.Counts {
	return s.cb.Counts()
}

// GetStatus 获取简化熔断器状态信息
func (s *SimplifiedCircuitBreakerProvider) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := s.cb.Counts()
	state := s.cb.State()

	return map[string]interface{}{
		"decorator_type":    "SimplifiedCircuitBreaker",
		"base_provider":     s.RealtimeStockProvider.Name(),
		"enabled":           s.config.Enabled,
		"state":             state.String(),
		"market_time_aware": s.config.MarketTimeAware,
		"in_trading_window": s.isInTradingWindow(),
		"counts": map[string]interface{}{
			"requests":              counts.Requests,
			"total_successes":       counts.TotalSuccesses,
			"total_failures":        counts.TotalFailures,
			"consecutive_successes": counts.ConsecutiveSuccesses,
			"consecutive_failures":  counts.ConsecutiveFailures,
		},
		"stats": map[string]interface{}{
			"total_requests":      s.stats.TotalRequests,
			"successful_requests": s.stats.SuccessfulRequest,
			"failed_requests":     s.stats.FailedRequests,
			"ip_ban_count":        s.stats.IPBanCount,
			"last_failure":        s.stats.LastFailure,
		},
		"config": map[string]interface{}{
			"name":          s.config.Name,
			"max_requests":  s.config.MaxRequests,
			"interval":      s.config.Interval.String(),
			"timeout":       s.config.Timeout.String(),
			"ready_to_trip": s.config.ReadyToTrip,
		},
	}
}

// SetEnabled 设置是否启用简化熔断器
func (s *SimplifiedCircuitBreakerProvider) SetEnabled(enabled bool) {
	s.config.Enabled = enabled
}

// Reset 重置简化熔断器状态（测试用）
func (s *SimplifiedCircuitBreakerProvider) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 重置统计信息
	s.stats = SimplifiedCircuitBreakerStats{}

	// 注意：gobreaker 不提供重置方法，这里只能重置我们自己的统计
	// 如果需要完全重置，需要重新创建 CircuitBreaker 实例
}

// IsOpen 检查熔断器是否处于打开状态
func (s *SimplifiedCircuitBreakerProvider) IsOpen() bool {
	return s.cb.State() == gobreaker.StateOpen
}

// IsHalfOpen 检查熔断器是否处于半开状态
func (s *SimplifiedCircuitBreakerProvider) IsHalfOpen() bool {
	return s.cb.State() == gobreaker.StateHalfOpen
}

// IsClosed 检查熔断器是否处于关闭状态
func (s *SimplifiedCircuitBreakerProvider) IsClosed() bool {
	return s.cb.State() == gobreaker.StateClosed
}
