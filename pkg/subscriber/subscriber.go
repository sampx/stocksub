package subscriber

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stocksub/pkg/core"
	"stocksub/pkg/logger"
	"stocksub/pkg/provider"

	"github.com/sirupsen/logrus"
)

// DefaultSubscriber 默认订阅器实现
type DefaultSubscriber struct {
	provider      provider.RealtimeStockProvider
	subscriptions map[string]*Subscription
	subsMu        sync.RWMutex
	eventChan     chan UpdateEvent
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	maxSubs       int
	minInterval   time.Duration
	maxInterval   time.Duration
	log           *logrus.Entry
}

// NewSubscriber 创建新的订阅器
func NewSubscriber(provider provider.RealtimeStockProvider) *DefaultSubscriber {
	return &DefaultSubscriber{
		provider:      provider,
		subscriptions: make(map[string]*Subscription),
		eventChan:     make(chan UpdateEvent, 1000),
		maxSubs:       100,
		minInterval:   1 * time.Second,
		maxInterval:   1 * time.Hour,
		log:           logger.WithComponent("Subscriber"),
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
		s.log.Infof("Updated subscription for %s with interval %v", symbol, interval)
	} else {
		s.subscriptions[symbol] = &Subscription{
			Symbol:   symbol,
			Interval: interval,
			Callback: callback,
			Active:   true,
		}
		s.log.Infof("Added subscription for %s with interval %v", symbol, interval)
	}

	// 发送订阅成功事件
	select {
	case s.eventChan <- UpdateEvent{
		Type:   EventTypeSubscribed,
		Symbol: symbol,
		Time:   time.Now(),
	}:
	default:
		s.log.Infof("Warning: event channel full, dropping subscription event for %s", symbol)
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
	s.log.Infof("Removed subscription for %s", symbol)

	// 发送取消订阅事件
	select {
	case s.eventChan <- UpdateEvent{
		Type:   EventTypeUnsubscribed,
		Symbol: symbol,
		Time:   time.Now(),
	}:
	default:
		s.log.Infof("Warning: event channel full, dropping unsubscription event for %s", symbol)
	}

	return nil
}

// Start 启动订阅器
func (s *DefaultSubscriber) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.wg.Add(1)
	go s.runSubscriptions()

	s.log.Infof("Started with provider: %s", s.provider.Name())
	return nil
}

// Stop 停止订阅器
func (s *DefaultSubscriber) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	close(s.eventChan)
	s.log.Infof("Stopped")
	return nil
}

func (s *DefaultSubscriber) GetSubscriptions() []Subscription {
	// 获取读锁：允许多个 goroutine 同时读取，但阻止写操作
	// 这确保在遍历 subscriptions map 时不会发生并发修改
	s.subsMu.RLock()
	// defer 确保函数退出时释放读锁，即使发生 panic 也能正确释放
	defer s.subsMu.RUnlock()

	// 预分配切片容量：使用 make([]Subscription, 0, len(s.subscriptions))
	// - 初始长度为 0，避免创建零值元素
	// - 容量为当前订阅数量，避免后续 append 操作引起的动态扩容
	// - 这是 Go 中高效切片操作的最佳实践
	subs := make([]Subscription, 0, len(s.subscriptions))

	// 遍历所有订阅：range 操作在读锁保护下是安全的
	// 注意：这里遍历的是 map 的值，Go 的 map 遍历顺序是随机的
	for _, sub := range s.subscriptions {
		// 值复制：append(subs, *sub) 创建 Subscription 结构体的副本
		// 使用指针解引用 (*sub) 而不是直接传递指针，原因：
		// 1. 防止外部代码通过返回的指针修改内部状态
		// 2. 避免内存泄漏（外部持有内部指针可能阻止 GC）
		// 3. 提供快照语义：返回调用时刻的状态副本
		subs = append(subs, *sub)
	}

	// 返回订阅副本切片：外部可以安全地遍历、排序或修改返回的切片
	// 而不会影响订阅器的内部状态
	return subs
}

// SetProvider 设置数据提供商
func (s *DefaultSubscriber) SetProvider(provider provider.RealtimeStockProvider) {
	s.provider = provider
	s.log.Infof("Provider changed to: %s", provider.Name())
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

// runSubscriptions 是订阅服务的核心运行方法，负责管理所有股票数据的订阅和更新
func (s *DefaultSubscriber) runSubscriptions() {
	// defer 关键字：确保函数退出时执行 s.wg.Done()
	// WaitGroup 是 Go 的同步原语，用于等待一组 goroutine 完成
	// Done() 方法会将计数器减1，告知主程序此 goroutine 已完成
	defer s.wg.Done()

	s.log.Infof("runSubscriptions started")

	// time.NewTicker 创建一个定时器，每1秒触发一次
	// Ticker 是 Go 中用于定期执行任务的机制，类似于定时器
	// 这里使用1秒固定间隔作为检查周期，而不是每个订阅的具体间隔
	ticker := time.NewTicker(1 * time.Second) // 使用固定的1秒间隔

	// defer 确保函数退出时停止 ticker，防止 goroutine 泄漏
	// 这是 Go 中资源管理的最佳实践
	defer ticker.Stop()

	// map 数据结构：记录每个股票代码的最后获取时间
	// key: 股票代码(string), value: 最后获取时间(time.Time)
	lastFetchTime := make(map[string]time.Time)

	s.log.Infof("Starting main subscription loop with 1s ticker")

	// 无限循环：这是 Go 中事件驱动编程的常见模式
	for {
		// select 语句：Go 的多路复用机制，类似于 switch 但用于 channel 操作
		// 它会阻塞等待，直到某个 case 可以执行
		select {
		// 监听上下文取消信号
		// context.Context 是 Go 中用于控制 goroutine 生命周期的标准方式
		// s.ctx.Done() 返回一个 channel，当上下文被取消时会关闭
		case <-s.ctx.Done():
			s.log.Infof("Context done, stopping subscription loop")
			return // 退出函数，结束此 goroutine

		// 监听定时器触发
		// ticker.C 是一个 time channel，每1秒会收到一个时间值
		case <-ticker.C:
			// time.Now() 获取当前时间
			now := time.Now()
			s.log.Debugf("Ticker fired at %v", now.Format("15:04:05.000"))

			// === 核心业务逻辑：检查哪些订阅需要更新数据 ===

			// sync.RWMutex 读写锁：允许多个读操作并发，但写操作独占
			// RLock() 获取读锁，用于安全地遍历 subscriptions map
			s.subsMu.RLock()

			// 声明切片存储需要获取数据的股票代码
			// []string 是字符串切片，Go 中的动态数组
			var symbolsToFetch []string

			// range 遍历 map：for key, value := range map
			// s.subscriptions 存储所有的订阅信息
			for symbol, sub := range s.subscriptions {
				// 检查订阅是否激活
				if !sub.Active {
					continue // 跳过未激活的订阅
				}

				// 检查是否需要获取新数据
				lastFetch, exists := lastFetchTime[symbol]

				// 条件判断：
				// 1. 如果从未获取过数据 (!exists)
				// 2. 或者距离上次获取的时间 >= 订阅间隔 (now.Sub(lastFetch) >= sub.Interval)
				// 则需要获取新数据
				if !exists || now.Sub(lastFetch) >= sub.Interval {
					// append() 是 Go 内置函数，用于向切片添加元素
					symbolsToFetch = append(symbolsToFetch, symbol)
					// 更新最后获取时间 ??
					lastFetchTime[symbol] = now
				}
			}
			// RUnlock() 释放读锁，允许其他 goroutine 进行读写操作
			s.subsMu.RUnlock()

			// 如果有需要获取数据的股票
			if len(symbolsToFetch) > 0 {
				s.log.Infof("Need to fetch data for symbols: %v", symbolsToFetch)

				// go 关键字：启动新的 goroutine（轻量级线程）
				// 这是 Go 的核心特性 - 并发编程
				// 异步执行数据获取，不阻塞主循环
				go s.fetchAndNotify(symbolsToFetch)
			} else {
				s.log.Infof("No symbols need updating at this time")
			}
		}
	}
}

// fetchAndNotify 获取股票数据并通知相关订阅者
//
// 功能说明：
//
//	这是订阅系统的核心数据处理方法，负责：
//	1. 调用数据提供商 API 批量获取多个股票的实时数据
//	2. 将获取的数据分发给对应的订阅回调函数
//	3. 处理数据获取和回调执行过程中的各种错误情况
//	4. 通过事件通道发送相应的事件通知
//
// 参数说明：
//
//	symbols []string - 需要获取数据的股票代码列表，格式如 ["600000", "000001", "300750"]
//
// 设计要点：
//   - 使用超时控制防止 API 调用无限等待
//   - 批量获取减少 API 调用次数，提高效率
//   - 异步回调避免阻塞主订阅循环
//   - 错误隔离确保单个股票的错误不影响其他股票
func (s *DefaultSubscriber) fetchAndNotify(symbols []string) {
	s.log.Infof("Starting fetchAndNotify for symbols: %v", symbols)

	// === 第一步：设置超时上下文 ===
	// context.WithTimeout 创建一个带超时的上下文，继承自 s.ctx
	// 30秒超时是为了防止 API 调用无限等待，保证系统响应性
	// 如果数据提供商响应慢或网络有问题，会在30秒后自动取消请求
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	// defer 确保函数退出时取消上下文，释放相关资源
	// 这是 Go 中资源管理的最佳实践
	defer cancel()

	// === 第二步：调用数据提供商 API 获取数据 ===
	// 记录开始时间，用于计算 API 调用耗时（性能监控）
	start := time.Now()

	// 调用提供商的 FetchData 方法批量获取股票数据
	// 这里是多态调用：s.provider 实现了 Provider 接口
	// 具体可能是 TencentProvider、YahooProvider 等不同实现
	data, err := s.provider.FetchStockData(ctx, symbols)

	// 计算 API 调用总耗时，用于性能分析和调试
	elapsed := time.Since(start)

	// === 第三步：处理 API 调用错误 ===
	if err != nil {
		// 记录错误日志，包含耗时信息便于分析超时问题
		s.log.Infof("Fetch data error after %v: %v", elapsed, err)

		// 错误传播：为每个请求的股票代码发送错误通知
		// 这确保了所有等待数据的订阅者都能收到错误信息
		// 而不是静默失败
		for _, symbol := range symbols {
			s.notifyError(symbol, err)
		}
		return // 提前返回，不继续处理数据
	}

	// 记录成功日志，包含性能信息：耗时、数据条数、请求股票数
	// 这对于监控系统性能和排查问题很有帮助
	s.log.Infof("Fetch data completed in %v, received %d records for %d symbols", elapsed, len(data), len(symbols))

	// === 第四步：组织数据结构 ===
	// 将切片数据转换为 map 结构，key 是股票代码，value 是股票数据
	// 这样可以通过 O(1) 时间复杂度快速查找特定股票的数据
	// 而不需要遍历整个切片（O(n) 复杂度）
	dataMap := make(map[string]core.StockData)
	for _, stock := range data {
		// 使用股票代码作为 key 建立索引
		dataMap[stock.Symbol] = stock
	}

	// === 第五步：分发数据给订阅者 ===
	// 获取读锁，保护对 subscriptions map 的并发访问
	// 这里只需要读锁因为我们只是读取订阅信息，不修改
	s.subsMu.RLock()

	// 遍历本次请求的所有股票代码
	for _, symbol := range symbols {
		// 检查该股票是否还有活跃的订阅
		// 订阅可能在数据获取期间被取消，所以需要重新检查
		if sub, exists := s.subscriptions[symbol]; exists && sub.Active {
			// 检查是否获取到了该股票的数据
			if stockData, found := dataMap[symbol]; found {
				// 异步调用回调函数，避免阻塞当前处理流程
				// 使用 goroutine 确保：
				// 1. 回调函数执行时间长不会影响其他股票的处理
				// 2. 回调函数中的 panic 不会影响当前 goroutine
				// 3. 多个股票的回调可以并发执行，提高效率
				go s.notifyCallback(sub, stockData)
			} else {
				// 数据缺失处理：API 返回成功但没有包含某个股票的数据
				// 这种情况可能发生在：
				// - 股票代码错误或已停牌
				// - 数据提供商暂时无法获取该股票数据
				// - API 响应格式异常
				go s.notifyError(symbol, fmt.Errorf("no data received for symbol %s", symbol))
			}
		}
		// 如果订阅不存在或已停用，则跳过该股票
		// 这是正常情况，不需要记录错误
	}

	// 释放读锁，允许其他 goroutine 进行读写操作
	s.subsMu.RUnlock()
}

// notifyCallback 通知回调函数
// 功能：
//   - 对订阅 sub 执行其回调函数 Callback，传入最新的数据 data。
//   - 保证回调执行的健壮性：从 panic 中恢复并记录日志；如果回调返回 error，记录并通过事件通道发出错误事件。
//   - 在回调完成后，尝试通过事件通道发出一条数据更新事件（非阻塞发送，通道满则丢弃以避免阻塞业务线程）。
//
// 设计要点：
//  1. panic 恢复：任何第三方/业务回调都可能产生 panic；使用 defer + recover 保证不会影响订阅循环或其他 goroutine。
//  2. 错误分流：回调返回的 error 会被转化为 EventTypeError 事件发送（非阻塞），便于统一上报与监控。
//  3. 事件发送策略：采用 select 非阻塞写入 eventChan。若通道已满，优先保证主流程不卡顿，因此静默丢弃。
//     如需“必达”语义，应在上层增加更大的缓冲/专用事件处理器/或重试与丢弃统计。
//  4. 时序说明：本方法通常在独立 goroutine 中调用（见 fetchAndNotify 中的 go s.notifyCallback），
//     因此内部不得产生长时间阻塞操作（例如：同步写满通道）。
func (s *DefaultSubscriber) notifyCallback(sub *Subscription, data core.StockData) {
	// 1) 保护区：确保回调产生的任何 panic 不会蔓延至系统其他部分
	defer func() {
		if r := recover(); r != nil {
			// 这里使用 Infof 记录；若需更高告警等级，可在日志配置中调整
			s.log.Infof("Callback panic for %s: %v", sub.Symbol, r)
		}
	}()

	// 2) 执行业务回调：回调若返回错误，既记录日志也发出错误事件，便于上层统一感知
	// 注意：这里不对错误进行重试，由上层策略（如 Manager）或回调方自行决定
	if err := sub.Callback(data); err != nil {
		s.log.Infof("Callback error for %s: %v", sub.Symbol, err)
		// 非阻塞错误通知：若通道满则丢弃，避免阻塞当前 goroutine
		s.notifyError(sub.Symbol, err)
	}

	// 3) 发送数据更新事件：
	//    - 无论回调是否返回错误，都会尝试发送数据事件（便于消费者同时获得数据与错误上下文）
	//    - 非阻塞发送：当 eventChan 已满时直接丢弃，保障系统整体吞吐不被背压影响
	select {
	case s.eventChan <- UpdateEvent{
		Type:   EventTypeData, // 事件类型：数据更新
		Symbol: sub.Symbol,    // 标的代码
		Data:   &data,         // 本次推送的数据（指针，避免大对象复制）
		Time:   time.Now(),    // 事件时间戳（用于下游统计/排序）
	}:
	default:
		// 事件通道满，丢弃事件（如需观测丢弃比例，可在此处增加计数器或 debug 日志）
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
