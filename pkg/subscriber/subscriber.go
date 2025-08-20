package subscriber

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// DefaultSubscriber 默认订阅器实现
type DefaultSubscriber struct {
	provider      Provider
	subscriptions map[string]*Subscription
	subsMu        sync.RWMutex
	eventChan     chan UpdateEvent
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	maxSubs       int
	minInterval   time.Duration
	maxInterval   time.Duration
}

// NewSubscriber 创建新的订阅器
func NewSubscriber(provider Provider) *DefaultSubscriber {
	return &DefaultSubscriber{
		provider:      provider,
		subscriptions: make(map[string]*Subscription),
		eventChan:     make(chan UpdateEvent, 1000),
		maxSubs:       100,
		minInterval:   1 * time.Second,
		maxInterval:   1 * time.Hour,
	}
}

// Subscribe 订阅股票
func (s *DefaultSubscriber) Subscribe(symbol string, interval time.Duration, callback CallbackFunc) error {
	if symbol == "" {
		return fmt.Errorf("symbol cannot be empty")
	}

	if callback == nil {
		return fmt.Errorf("callback cannot be nil")
	}

	if interval < s.minInterval {
		return fmt.Errorf("interval too short, minimum is %v", s.minInterval)
	}

	if interval > s.maxInterval {
		return fmt.Errorf("interval too long, maximum is %v", s.maxInterval)
	}

	if !s.provider.IsSymbolSupported(symbol) {
		return fmt.Errorf("symbol %s is not supported by provider %s", symbol, s.provider.Name())
	}

	s.subsMu.Lock()
	defer s.subsMu.Unlock()

	if len(s.subscriptions) >= s.maxSubs {
		return fmt.Errorf("maximum subscriptions (%d) reached", s.maxSubs)
	}

	// 如果已存在订阅，更新它
	if existing, exists := s.subscriptions[symbol]; exists {
		existing.Interval = interval
		existing.Callback = callback
		existing.Active = true
		log.Printf("[Subscriber] Updated subscription for %s with interval %v", symbol, interval)
	} else {
		s.subscriptions[symbol] = &Subscription{
			Symbol:   symbol,
			Interval: interval,
			Callback: callback,
			Active:   true,
		}
		log.Printf("[Subscriber] Added subscription for %s with interval %v", symbol, interval)
	}

	// 发送订阅成功事件
	select {
	case s.eventChan <- UpdateEvent{
		Type:   EventTypeSubscribed,
		Symbol: symbol,
		Time:   time.Now(),
	}:
	default:
		log.Printf("[Subscriber] Warning: event channel full, dropping subscription event for %s", symbol)
	}

	return nil
}

// Unsubscribe 取消订阅
func (s *DefaultSubscriber) Unsubscribe(symbol string) error {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()

	if _, exists := s.subscriptions[symbol]; !exists {
		return fmt.Errorf("no subscription found for symbol %s", symbol)
	}

	delete(s.subscriptions, symbol)
	log.Printf("[Subscriber] Removed subscription for %s", symbol)

	// 发送取消订阅事件
	select {
	case s.eventChan <- UpdateEvent{
		Type:   EventTypeUnsubscribed,
		Symbol: symbol,
		Time:   time.Now(),
	}:
	default:
		log.Printf("[Subscriber] Warning: event channel full, dropping unsubscription event for %s", symbol)
	}

	return nil
}

// Start 启动订阅器
func (s *DefaultSubscriber) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.wg.Add(1)
	go s.runSubscriptions()

	log.Printf("[Subscriber] Started with provider: %s", s.provider.Name())
	return nil
}

// Stop 停止订阅器
func (s *DefaultSubscriber) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	close(s.eventChan)
	log.Printf("[Subscriber] Stopped")
	return nil
}

// GetSubscriptions 获取当前订阅列表
func (s *DefaultSubscriber) GetSubscriptions() []Subscription {
	s.subsMu.RLock()
	defer s.subsMu.RUnlock()

	subs := make([]Subscription, 0, len(s.subscriptions))
	for _, sub := range s.subscriptions {
		subs = append(subs, *sub)
	}

	return subs
}

// SetProvider 设置数据提供商
func (s *DefaultSubscriber) SetProvider(provider Provider) {
	s.provider = provider
	log.Printf("[Subscriber] Provider changed to: %s", provider.Name())
}

// GetEventChannel 获取事件通道
func (s *DefaultSubscriber) GetEventChannel() <-chan UpdateEvent {
	return s.eventChan
}

// SetMaxSubscriptions 设置最大订阅数
func (s *DefaultSubscriber) SetMaxSubscriptions(max int) {
	s.maxSubs = max
}

// SetIntervalLimits 设置订阅间隔限制
func (s *DefaultSubscriber) SetIntervalLimits(min, max time.Duration) {
	s.minInterval = min
	s.maxInterval = max
}

// runSubscriptions 运行订阅逻辑
func (s *DefaultSubscriber) runSubscriptions() {
	defer s.wg.Done()

	log.Printf("[Subscriber] runSubscriptions started")

	// 创建单个 ticker 用于所有订阅，使用最小间隔
	ticker := time.NewTicker(1 * time.Second) // 使用固定的1秒间隔
	defer ticker.Stop()

	lastFetchTime := make(map[string]time.Time)

	log.Printf("[Subscriber] Starting main subscription loop with 1s ticker")

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("[Subscriber] Context done, stopping subscription loop")
			return

		case <-ticker.C:
			now := time.Now()
			log.Printf("[Subscriber] Ticker fired at %v", now.Format("15:04:05.000"))

			// 检查每个订阅是否需要更新
			s.subsMu.RLock()
			var symbolsToFetch []string

			for symbol, sub := range s.subscriptions {
				if !sub.Active {
					continue
				}

				lastFetch, exists := lastFetchTime[symbol]
				if !exists || now.Sub(lastFetch) >= sub.Interval {
					symbolsToFetch = append(symbolsToFetch, symbol)
					lastFetchTime[symbol] = now
				}
			}
			s.subsMu.RUnlock()

			if len(symbolsToFetch) > 0 {
				log.Printf("[Subscriber] Need to fetch data for symbols: %v", symbolsToFetch)
				go s.fetchAndNotify(symbolsToFetch)
			} else {
				log.Printf("[Subscriber] No symbols need updating at this time")
			}
		}
	}
}

// fetchAndNotify 获取数据并通知
func (s *DefaultSubscriber) fetchAndNotify(symbols []string) {
	log.Printf("[Subscriber] Starting fetchAndNotify for symbols: %v", symbols)

	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	start := time.Now()
	data, err := s.provider.FetchData(ctx, symbols)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("[Subscriber] Fetch data error after %v: %v", elapsed, err)
		for _, symbol := range symbols {
			s.notifyError(symbol, err)
		}
		return
	}

	log.Printf("[Subscriber] Fetch data completed in %v, received %d records for %d symbols", elapsed, len(data), len(symbols))

	// 按symbol组织数据
	dataMap := make(map[string]StockData)
	for _, stock := range data {
		dataMap[stock.Symbol] = stock
	}

	// 通知每个订阅
	s.subsMu.RLock()
	for _, symbol := range symbols {
		if sub, exists := s.subscriptions[symbol]; exists && sub.Active {
			if stockData, found := dataMap[symbol]; found {
				go s.notifyCallback(sub, stockData)
			} else {
				go s.notifyError(symbol, fmt.Errorf("no data received for symbol %s", symbol))
			}
		}
	}
	s.subsMu.RUnlock()
}

// notifyCallback 通知回调函数
func (s *DefaultSubscriber) notifyCallback(sub *Subscription, data StockData) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Subscriber] Callback panic for %s: %v", sub.Symbol, r)
		}
	}()

	if err := sub.Callback(data); err != nil {
		log.Printf("[Subscriber] Callback error for %s: %v", sub.Symbol, err)
		s.notifyError(sub.Symbol, err)
	}

	// 发送数据更新事件
	select {
	case s.eventChan <- UpdateEvent{
		Type:   EventTypeData,
		Symbol: sub.Symbol,
		Data:   &data,
		Time:   time.Now(),
	}:
	default:
		// 事件通道满，丢弃事件
	}
}

// notifyError 通知错误
func (s *DefaultSubscriber) notifyError(symbol string, err error) {
	select {
	case s.eventChan <- UpdateEvent{
		Type:   EventTypeError,
		Symbol: symbol,
		Error:  err,
		Time:   time.Now(),
	}:
	default:
		// 事件通道满，丢弃事件
	}
}
