package subscriber

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager 订阅管理器，提供高级订阅管理功能
type Manager struct {
	subscriber *DefaultSubscriber
	config     *ManagerConfig
	stats      *Statistics
	statsMu    sync.RWMutex
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	AutoRestart         bool          // 自动重启失败的订阅
	HealthCheckInterval time.Duration // 健康检查间隔
	MaxFailures         int           // 最大失败次数
	FailureWindow       time.Duration // 失败窗口时间
}

// Statistics 统计信息
type Statistics struct {
	TotalSubscriptions  int                  `json:"total_subscriptions"`
	ActiveSubscriptions int                  `json:"active_subscriptions"`
	TotalDataPoints     int64                `json:"total_data_points"`
	TotalErrors         int64                `json:"total_errors"`
	SubscriptionStats   map[string]*SubStats `json:"subscription_stats"`
	ProviderStats       *ProviderStats       `json:"provider_stats"`
	StartTime           time.Time            `json:"start_time"`
	LastUpdateTime      time.Time            `json:"last_update_time"`
}

// SubStats 单个订阅统计
type SubStats struct {
	Symbol          string        `json:"symbol"`
	SubscribedAt    time.Time     `json:"subscribed_at"`
	LastDataTime    time.Time     `json:"last_data_time"`
	DataPointCount  int64         `json:"data_point_count"`
	ErrorCount      int64         `json:"error_count"`
	AverageInterval time.Duration `json:"average_interval"`
	LastError       string        `json:"last_error,omitempty"`
	LastErrorTime   time.Time     `json:"last_error_time,omitempty"`
	IsHealthy       bool          `json:"is_healthy"`
}

// ProviderStats 提供商统计
type ProviderStats struct {
	Name            string        `json:"name"`
	TotalRequests   int64         `json:"total_requests"`
	SuccessfulReqs  int64         `json:"successful_requests"`
	FailedRequests  int64         `json:"failed_requests"`
	AverageLatency  time.Duration `json:"average_latency"`
	LastRequestTime time.Time     `json:"last_request_time"`
}

// NewManager 创建订阅管理器
func NewManager(subscriber *DefaultSubscriber) *Manager {
	config := &ManagerConfig{
		AutoRestart:         true,
		HealthCheckInterval: 30 * time.Second,
		MaxFailures:         5,
		FailureWindow:       5 * time.Minute,
	}

	stats := &Statistics{
		SubscriptionStats: make(map[string]*SubStats),
		ProviderStats:     &ProviderStats{},
		StartTime:         time.Now(),
	}

	return &Manager{
		subscriber: subscriber,
		config:     config,
		stats:      stats,
	}
}

// Start 启动管理器
func (m *Manager) Start(ctx context.Context) error {
	// 启动订阅器
	if err := m.subscriber.Start(ctx); err != nil {
		return fmt.Errorf("failed to start subscriber: %w", err)
	}

	// 启动统计收集
	go m.runStatisticsCollector(ctx)

	// 启动健康检查
	go m.runHealthChecker(ctx)

	// 启动事件处理
	go m.runEventProcessor(ctx)

	log.Printf("[Manager] Started with config: AutoRestart=%v, HealthCheckInterval=%v",
		m.config.AutoRestart, m.config.HealthCheckInterval)

	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	return m.subscriber.Stop()
}

// Subscribe 订阅股票（增强版）
func (m *Manager) Subscribe(symbol string, interval time.Duration, callback CallbackFunc) error {
	err := m.subscriber.Subscribe(symbol, interval, callback)
	if err != nil {
		return err
	}

	// 初始化统计信息
	m.statsMu.Lock()
	m.stats.SubscriptionStats[symbol] = &SubStats{
		Symbol:       symbol,
		SubscribedAt: time.Now(),
		IsHealthy:    true,
	}
	m.stats.TotalSubscriptions++
	m.stats.ActiveSubscriptions++
	m.statsMu.Unlock()

	log.Printf("[Manager] Successfully subscribed to %s with interval %v", symbol, interval)
	return nil
}

// Unsubscribe 取消订阅（增强版）
func (m *Manager) Unsubscribe(symbol string) error {
	err := m.subscriber.Unsubscribe(symbol)
	if err != nil {
		return err
	}

	// 清理统计信息
	m.statsMu.Lock()
	delete(m.stats.SubscriptionStats, symbol)
	if m.stats.ActiveSubscriptions > 0 {
		m.stats.ActiveSubscriptions--
	}
	m.statsMu.Unlock()

	log.Printf("[Manager] Successfully unsubscribed from %s", symbol)
	return nil
}

// SubscribeBatch 批量订阅
func (m *Manager) SubscribeBatch(requests []SubscribeRequest) error {
	var errors []error

	for _, req := range requests {
		if err := m.Subscribe(req.Symbol, req.Interval, req.Callback); err != nil {
			errors = append(errors, fmt.Errorf("failed to subscribe %s: %w", req.Symbol, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch subscription failed: %v", errors)
	}

	return nil
}

// UnsubscribeBatch 批量取消订阅
func (m *Manager) UnsubscribeBatch(symbols []string) error {
	var errors []error

	for _, symbol := range symbols {
		if err := m.Unsubscribe(symbol); err != nil {
			errors = append(errors, fmt.Errorf("failed to unsubscribe %s: %w", symbol, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch unsubscription failed: %v", errors)
	}

	return nil
}

// GetStatistics 获取统计信息
func (m *Manager) GetStatistics() Statistics {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()

	// 深拷贝统计信息
	stats := *m.stats
	stats.SubscriptionStats = make(map[string]*SubStats)
	for k, v := range m.stats.SubscriptionStats {
		statsCopy := *v
		stats.SubscriptionStats[k] = &statsCopy
	}

	if m.stats.ProviderStats != nil {
		providerStats := *m.stats.ProviderStats
		stats.ProviderStats = &providerStats
	}

	return stats
}

// GetSubscriptions 获取订阅列表
func (m *Manager) GetSubscriptions() []Subscription {
	return m.subscriber.GetSubscriptions()
}

// runStatisticsCollector 运行统计收集器
func (m *Manager) runStatisticsCollector(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.updateStatistics()
		}
	}
}

// runHealthChecker 运行健康检查
func (m *Manager) runHealthChecker(ctx context.Context) {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// runEventProcessor 运行事件处理器
func (m *Manager) runEventProcessor(ctx context.Context) {
	eventChan := m.subscriber.GetEventChannel()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			m.processEvent(event)
		}
	}
}

// processEvent 处理事件
func (m *Manager) processEvent(event UpdateEvent) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	stats, exists := m.stats.SubscriptionStats[event.Symbol]
	if !exists {
		return
	}

	switch event.Type {
	case EventTypeData:
		stats.DataPointCount++
		stats.LastDataTime = event.Time
		stats.IsHealthy = true
		m.stats.TotalDataPoints++

	case EventTypeError:
		stats.ErrorCount++
		stats.IsHealthy = false
		if event.Error != nil {
			stats.LastError = event.Error.Error()
			stats.LastErrorTime = event.Time
		}
		m.stats.TotalErrors++

		// 自动重启逻辑
		if m.config.AutoRestart && stats.ErrorCount >= int64(m.config.MaxFailures) {
			go m.attemptRestart(event.Symbol)
		}
	}

	m.stats.LastUpdateTime = event.Time
}

// updateStatistics 更新统计信息
func (m *Manager) updateStatistics() {
	subscriptions := m.subscriber.GetSubscriptions()

	m.statsMu.Lock()
	m.stats.ActiveSubscriptions = len(subscriptions)
	m.statsMu.Unlock()
}

// performHealthCheck 执行健康检查
func (m *Manager) performHealthCheck() {
	m.statsMu.RLock()
	unhealthySymbols := make([]string, 0)

	for symbol, stats := range m.stats.SubscriptionStats {
		// 检查是否长时间没有数据更新
		if time.Since(stats.LastDataTime) > 2*m.config.HealthCheckInterval {
			stats.IsHealthy = false
			unhealthySymbols = append(unhealthySymbols, symbol)
		}
	}
	m.statsMu.RUnlock()

	if len(unhealthySymbols) > 0 {
		log.Printf("[Manager] Health check found %d unhealthy subscriptions: %v",
			len(unhealthySymbols), unhealthySymbols)
	}
}

// attemptRestart 尝试重启订阅
func (m *Manager) attemptRestart(symbol string) {
	log.Printf("[Manager] Attempting to restart subscription for %s", symbol)

	// 获取当前订阅信息
	subscriptions := m.subscriber.GetSubscriptions()
	var targetSub *Subscription
	for _, sub := range subscriptions {
		if sub.Symbol == symbol {
			targetSub = &sub
			break
		}
	}

	if targetSub == nil {
		log.Printf("[Manager] Cannot restart %s: subscription not found", symbol)
		return
	}

	// 取消并重新订阅
	if err := m.subscriber.Unsubscribe(symbol); err != nil {
		log.Printf("[Manager] Failed to unsubscribe %s for restart: %v", symbol, err)
		return
	}

	time.Sleep(1 * time.Second) // 短暂等待

	if err := m.subscriber.Subscribe(symbol, targetSub.Interval, targetSub.Callback); err != nil {
		log.Printf("[Manager] Failed to restart subscription for %s: %v", symbol, err)
		return
	}

	// 重置错误计数
	m.statsMu.Lock()
	if stats, exists := m.stats.SubscriptionStats[symbol]; exists {
		stats.ErrorCount = 0
		stats.IsHealthy = true
		stats.LastError = ""
	}
	m.statsMu.Unlock()

	log.Printf("[Manager] Successfully restarted subscription for %s", symbol)
}

// SubscribeRequest 订阅请求
type SubscribeRequest struct {
	Symbol   string
	Interval time.Duration
	Callback CallbackFunc
}
