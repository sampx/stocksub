package decorators

import (
	"context"
	"fmt"
	"stocksub/pkg/core"
	"stocksub/pkg/provider"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreakerProvider 熔断器装饰器
// 使用 sony/gobreaker 提供熔断功能
type CircuitBreakerProvider struct {
	provider.RealtimeStockProvider
	*provider.BaseDecorator
	// 熔断器组件
	cb     *gobreaker.CircuitBreaker
	config *CircuitBreakerConfig

	// 统计信息
	mu    sync.RWMutex
	stats CircuitBreakerStats
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Name        string        `yaml:"name"`          // 熔断器名称
	MaxRequests uint32        `yaml:"max_requests"`  // 半开状态下的最大请求数
	Interval    time.Duration `yaml:"interval"`      // 统计窗口时间
	Timeout     time.Duration `yaml:"timeout"`       // 熔断器打开后的超时时间
	ReadyToTrip uint32        `yaml:"ready_to_trip"` // 触发熔断的失败次数阈值
	Enabled     bool          `yaml:"enabled"`       // 是否启用熔断器
}

// CircuitBreakerStats 熔断器统计信息
type CircuitBreakerStats struct {
	TotalRequests     int64     `json:"total_requests"`
	SuccessfulRequest int64     `json:"successful_requests"`
	FailedRequests    int64     `json:"failed_requests"`
	LastFailure       time.Time `json:"last_failure"`
}

// DefaultCircuitBreakerConfig 默认熔断器配置
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Name:        "StockProvider",
		MaxRequests: 5,                // 半开状态允许5个请求
		Interval:    60 * time.Second, // 60秒统计窗口
		Timeout:     30 * time.Second, // 熔断30秒
		ReadyToTrip: 5,                // 5次失败触发熔断
		Enabled:     true,             // 默认启用
	}
}

// NewCircuitBreakerProvider 创建熔断器装饰器
func NewCircuitBreakerProvider(stockProvider provider.RealtimeStockProvider, config *CircuitBreakerConfig) *CircuitBreakerProvider {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

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
			fmt.Printf("熔断器 %s 状态从 %v 变更为 %v\n", name, from, to)
		},
	}

	return &CircuitBreakerProvider{
		RealtimeStockProvider: stockProvider,
		BaseDecorator:         provider.NewBaseDecorator(stockProvider),
		cb:                    gobreaker.NewCircuitBreaker(settings),
		config:                config,
		stats:                 CircuitBreakerStats{},
	}
}

// IsHealthy 检查健康状态
func (c *CircuitBreakerProvider) IsHealthy() bool {
	if !c.config.Enabled {
		return c.RealtimeStockProvider.IsHealthy()
	}

	// 熔断器打开状态视为不健康
	state := c.cb.State()
	return state != gobreaker.StateOpen && c.RealtimeStockProvider.IsHealthy()
}

// Name 返回装饰器名称
func (c *CircuitBreakerProvider) Name() string {
	return fmt.Sprintf("CircuitBreaker(%s)", c.RealtimeStockProvider.Name())
}

func (c *CircuitBreakerProvider) GetRateLimit() time.Duration {
	return c.RealtimeStockProvider.GetRateLimit()
}

// FetchStockData 实现带熔断器的股票数据获取
func (c *CircuitBreakerProvider) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	if !c.config.Enabled {
		// 如果熔断器未启用，直接调用基础提供商
		return c.RealtimeStockProvider.FetchStockData(ctx, symbols)
	}

	// 更新统计信息
	c.mu.Lock()
	c.stats.TotalRequests++
	c.mu.Unlock()

	// 通过熔断器执行请求
	result, err := c.cb.Execute(func() (interface{}, error) {
		return c.RealtimeStockProvider.FetchStockData(ctx, symbols)
	})

	// 处理结果和错误统计
	c.handleResult(err)

	if err != nil {
		return nil, err
	}

	// 类型断言转换结果
	data, ok := result.([]core.StockData)
	if !ok {
		err := fmt.Errorf("熔断器返回数据类型错误")
		c.handleResult(err)
		return nil, err
	}

	return data, nil
}

// FetchStockDataWithRaw 实现带熔断器的股票数据获取（包含原始数据）
func (c *CircuitBreakerProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	if !c.config.Enabled {
		return c.RealtimeStockProvider.FetchStockDataWithRaw(ctx, symbols)
	}

	c.mu.Lock()
	c.stats.TotalRequests++
	c.mu.Unlock()

	// 定义包装结果结构
	type Result struct {
		Data []core.StockData
		Raw  string
	}

	// 通过熔断器执行请求
	result, err := c.cb.Execute(func() (interface{}, error) {
		data, raw, err := c.RealtimeStockProvider.FetchStockDataWithRaw(ctx, symbols)
		if err != nil {
			return nil, err
		}
		return Result{Data: data, Raw: raw}, nil
	})

	c.handleResult(err)

	if err != nil {
		return nil, "", err
	}

	// 类型断言转换结果
	res, ok := result.(Result)
	if !ok {
		err := fmt.Errorf("熔断器返回数据类型错误")
		c.handleResult(err)
		return nil, "", err
	}

	return res.Data, res.Raw, nil
}

// handleResult 处理请求结果和更新统计信息
func (c *CircuitBreakerProvider) handleResult(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil {
		c.stats.FailedRequests++
		c.stats.LastFailure = time.Now()
	} else {
		c.stats.SuccessfulRequest++
	}
}

// GetState 获取熔断器当前状态
func (c *CircuitBreakerProvider) GetState() gobreaker.State {
	return c.cb.State()
}

// GetCounts 获取熔断器计数信息
func (c *CircuitBreakerProvider) GetCounts() gobreaker.Counts {
	return c.cb.Counts()
}

// GetStatus 获取熔断器状态信息
func (c *CircuitBreakerProvider) GetStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	counts := c.cb.Counts()
	state := c.cb.State()

	return map[string]interface{}{
		"decorator_type": "CircuitBreaker",
		"base_provider":  c.RealtimeStockProvider.Name(),
		"enabled":        c.config.Enabled,
		"state":          state.String(),
		"counts": map[string]interface{}{
			"requests":              counts.Requests,
			"total_successes":       counts.TotalSuccesses,
			"total_failures":        counts.TotalFailures,
			"consecutive_successes": counts.ConsecutiveSuccesses,
			"consecutive_failures":  counts.ConsecutiveFailures,
		},
		"stats": map[string]interface{}{
			"total_requests":      c.stats.TotalRequests,
			"successful_requests": c.stats.SuccessfulRequest,
			"failed_requests":     c.stats.FailedRequests,
			"last_failure":        c.stats.LastFailure,
		},
		"config": map[string]interface{}{
			"name":          c.config.Name,
			"max_requests":  c.config.MaxRequests,
			"interval":      c.config.Interval.String(),
			"timeout":       c.config.Timeout.String(),
			"ready_to_trip": c.config.ReadyToTrip,
		},
	}
}

// SetEnabled 设置是否启用熔断器
func (c *CircuitBreakerProvider) SetEnabled(enabled bool) {
	c.config.Enabled = enabled
}

// Reset 重置熔断器状态（测试用）
func (c *CircuitBreakerProvider) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 重置统计信息
	c.stats = CircuitBreakerStats{}

	// 注意：gobreaker 不提供重置方法，这里只能重置我们自己的统计
	// 如果需要完全重置，需要重新创建 CircuitBreaker 实例
}

// IsOpen 检查熔断器是否处于打开状态
func (c *CircuitBreakerProvider) IsOpen() bool {
	return c.cb.State() == gobreaker.StateOpen
}

// IsHalfOpen 检查熔断器是否处于半开状态
func (c *CircuitBreakerProvider) IsHalfOpen() bool {
	return c.cb.State() == gobreaker.StateHalfOpen
}

// IsClosed 检查熔断器是否处于关闭状态
func (c *CircuitBreakerProvider) IsClosed() bool {
	return c.cb.State() == gobreaker.StateClosed
}

// --- Historical Provider Support ---

// CircuitBreakerForHistoricalProvider 是为历史数据提供商设计的熔断器
type CircuitBreakerForHistoricalProvider struct {
	provider.HistoricalProvider
	*provider.BaseDecorator
	cb     *gobreaker.CircuitBreaker
	config *CircuitBreakerConfig
}

// NewCircuitBreakerForHistoricalProvider 创建一个新的历史数据熔断器装饰器
func NewCircuitBreakerForHistoricalProvider(p provider.HistoricalProvider, config *CircuitBreakerConfig) provider.Provider {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
		config.Name = "HistoricalProvider" // 为历史提供商使用不同的名称
	}

	settings := gobreaker.Settings{
		Name:        config.Name,
		MaxRequests: config.MaxRequests,
		Interval:    config.Interval,
		Timeout:     config.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= config.ReadyToTrip
		},
	}

	return &CircuitBreakerForHistoricalProvider{
		HistoricalProvider: p,
		BaseDecorator:      provider.NewBaseDecorator(p),
		cb:                 gobreaker.NewCircuitBreaker(settings),
		config:             config,
	}
}

func (c *CircuitBreakerForHistoricalProvider) GetRateLimit() time.Duration {
	return c.HistoricalProvider.GetRateLimit()
}

func (c *CircuitBreakerForHistoricalProvider) IsHealthy() bool {
	return c.HistoricalProvider.IsHealthy()
}

func (c *CircuitBreakerForHistoricalProvider) Name() string {
	return fmt.Sprintf("CircuitBreaker(%s)", c.HistoricalProvider.Name())
}

// FetchHistoricalData 实现带熔断的历史数据获取
func (c *CircuitBreakerForHistoricalProvider) FetchHistoricalData(ctx context.Context, symbol string, start, end time.Time, period string) ([]core.HistoricalData, error) {
	if !c.config.Enabled {
		return c.HistoricalProvider.FetchHistoricalData(ctx, symbol, start, end, period)
	}

	result, err := c.cb.Execute(func() (interface{}, error) {
		return c.HistoricalProvider.FetchHistoricalData(ctx, symbol, start, end, period)
	})

	if err != nil {
		return nil, err
	}

	data, ok := result.([]core.HistoricalData)
	if !ok {
		return nil, fmt.Errorf("熔断器返回历史数据类型错误")
	}

	return data, nil
}
