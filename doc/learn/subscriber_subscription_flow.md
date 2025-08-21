# 订阅机制代码解读与业务说明（subscriber.go）

适用范围：
- 文件：`pkg/subscriber/subscriber.go`
- 相关类型/接口：`pkg/subscriber/types.go`（`Provider`、`Subscription`、`UpdateEvent` 等）

目标：
- 用业务语言解释每个函数的作用和数据流
- 用浅显比喻理解“锁、通道、goroutine、context”等并发机制
- 帮助你能自信地阅读、修改与优化这部分代码

---

## 一、总体业务图景

- 你告知系统“订阅哪些股票（symbol）、多久更新一次（interval）、拿到数据后怎么处理（callback）”
- 系统后台每秒响一次“统一秒表”（ticker），根据各自的更新频率判断谁“到点”，把到点的股票合并为一批，向数据提供方（Provider）批量拉取
- 拉到数据后，逐只股票调用你提供的回调函数，并向“事件通道”投递一条消息，便于旁路监控
- 系统支持优雅停止、抓取超时、回调异常保护、事件通道背压（满了丢弃）

关键词（比喻）：
- goroutine = 小任务（小工人）
- RWMutex 读写锁 = 清单门禁（看清单可并行，改清单要排队）
- channel 通道 = 消息栏（贴纸条，别人来取）
- context = 通行证/收工令（超时或取消时统一收工）
- ticker = 秒表（固定节拍器）

---

## 二、核心结构体与字段（DefaultSubscriber）

- `provider Provider`
  - 数据源实现（如腾讯行情）。必须实现接口：`FetchData`（批量拉数据）、`IsSymbolSupported`（是否支持该代码）等
- `subscriptions map[string]*Subscription`
  - 订阅清单：key 是股票代码，value 包含 `Interval`（更新频率）、`Callback`（回调）、`Active`（是否激活）
- `subsMu sync.RWMutex`
  - 保护订阅清单的“门禁”，防止多人同时改坏数据
  - “读多写少”的场景下，用读写锁提高并发读性能
- `eventChan chan UpdateEvent`
  - 事件消息栏。订阅/取消/有数据/错误等事件都会尝试贴进来
  - 非阻塞投递：满则丢弃，保证主流程不被拖慢
- `ctx context.Context` + `cancel context.CancelFunc` + `wg sync.WaitGroup`
  - 生命周期控制：开始、停止、优雅退出
- `minInterval/maxInterval/maxSubs`
  - 业务约束：最小/最大订阅周期、最大订阅数量
- `log *logrus.Entry`
  - 结构化日志，便于排查问题

---

## 三、对外 API 与业务流程

### 1) Subscribe(symbol, interval, callback)
业务含义：
- 注册或更新一个股票订阅：多久抓一次，抓到后交给谁处理

关键点：
- 校验输入（symbol 非空、callback 非空、interval 在范围内、provider 支持该股票）
- 在“门禁”保护下修改订阅清单（避免并发问题）
- 已存在则更新（Upsert 语义），否则新增；都标记为 Active
- 尝试往事件栏发一条“订阅成功”的消息（满了就丢弃，不阻塞）

你可以修改/优化的常见点：
- 更新时是否保留上次抓取时间（目前 lastFetchTime 只在调度循环里维护，独立于订阅清单）

### 2) Unsubscribe(symbol)
业务含义：
- 从订阅清单里移除该股票，并发“取消订阅”事件

关键点：
- 门禁保护下删除订阅
- 事件消息“取消订阅”为最佳努力（满则丢）

### 3) Start(ctx)
业务含义：
- 启动后台调度循环（runSubscriptions）

关键点：
- 基于传入 `ctx` 创建可取消的上下文
- 新开一个小任务（goroutine）跑 `runSubscriptions`
- 后台开始按固定 1 秒节拍调度

### 4) Stop()
业务含义：
- 优雅停止后台循环并关闭事件通道

顺序重要：
- 先 `cancel()` 发“收工令”
- `wg.Wait()` 等待后台循环确实退出
- `close(eventChan)` 告知“以后不会再有事件了”

---

## 四、主循环：runSubscriptions（调度心脏）

核心逻辑（每 1 秒一次）：
1. 建一个固定 1 秒的秒表（ticker）
2. 维护一张“本地备忘录”`lastFetchTime[code]`，记录每只股票“上次计划/执行抓取的时间”
3. 秒表每响一次：
   - 遍历订阅清单（读锁保护下读取）
   - 对每个激活的订阅，比较“现在时间 - 上次抓取时间”是否已达到订阅的 `Interval`
   - 达到就加入“本次待抓队列 symbolsToFetch”，并立即把 `lastFetchTime` 更新为“现在”
     - 这样做能避免在抓取尚未完成期间，被下一次秒表重复选中，造成重复抓取
   - 如果本次队列非空，异步启动一次批量抓取 `fetchAndNotify(symbolsToFetch)`

为什么用“统一 1 秒秒表 + 各自 Interval 判断”：
- 简单可靠，易读易维护
- 不需要为每一个订阅独立建一个定时器，避免大量小定时器带来的复杂度与资源消耗
- 最小生效间隔受限于秒表周期（当前为 1 秒）

你可以考虑的优化：
- 把 `ticker` 从固定 1 秒改成 `s.minInterval`，让系统配置更直观
- 做更智能的调度（如最小堆或时间轮），减少无效 tick

---

## 五、批量抓取与分发：fetchAndNotify

业务含义：
- 对“到点”的一批股票向 Provider 批量请求数据，并把结果分发给各自的回调

关键点：
- 创建 30 秒超时的上下文，防止外部服务卡死拖垮系统
- 若抓取失败：这批里的每个股票都要收到一个“错误事件”（便于上游知道这次失败了）
- 抓取成功：把返回的切片按 symbol 建成 `dataMap`，方便 O(1) 查询
- 读锁保护下检查订阅是否还存在且仍然激活
- 对每只股票：
  - 如果拿到了数据：异步执行它的回调（每只股票开一个小任务，互不影响）
  - 如果没拿到：异步发错误事件（“无数据”）

你可以考虑的优化：
- 根据 Provider 限流/并发能力，增加批次并行度或限制并发
- 增加“缺失数据”的重试/补偿策略

---

## 六、回调与事件：notifyCallback / notifyError

### notifyCallback
- 保护回调：即使你的回调代码崩溃（panic），也不会影响主引擎（`recover` 兜底）
- 如果回调返回错误，会发一条“错误事件”
- 最后尝试发一条“数据事件”（如果消息栏满了就丢）

### notifyError
- 尝试发“错误事件”，消息栏满时也丢弃

“丢弃事件”的业务考虑：
- 事件是“旁路监控”，不应该拖慢核心抓取与分发流程
- 如果你不希望丢事件，可以改为阻塞发送或做本地堆积，但要注意可能反向拖慢主流程

---

## 七、事件系统（UpdateEvent）

四类事件：
- `EventTypeSubscribed` / `EventTypeUnsubscribed`：订阅/取消订阅
- `EventTypeData`：成功拿到数据
- `EventTypeError`：抓取或回调出错

消费方式：
- 通过 `GetEventChannel()` 拿到只读通道，单独开小任务读取处理
- 注意：这是“最佳努力”，通道满了会丢事件

---

## 八、并发模型（用业务话术理解）

- “清单门禁”（RWMutex）
  - 看清单：大家可以同时看（读锁）
  - 改清单：要排队，避免冲突（写锁）
- “小任务”（goroutine）
  - 让不同股票的回调互不阻塞
  - 让批量抓取不阻塞下一次调度 tick
- “消息栏”（channel）
  - 放事件的队列。为了不拖慢主线，满了会丢
- “收工令/超时”（context）
  - 统一控制何时停止，抓取超时后尽快失败返回

---

## 九、常见修改点与建议

- 调整节拍：`ticker := time.NewTicker(1 * time.Second)` 可替换为 `time.NewTicker(s.minInterval)`
- 事件可靠性：把事件发送从非阻塞改为阻塞或带重试（但要评估对延迟的影响）
- 订阅扩展：新增“暂停/恢复”功能（当前用 `Active` 字段可以近似实现）
- 抓取策略：根据 Provider 的限流做批次拆分、合并或节流
- 可观测性：为 `fetchAndNotify` 增加更多维度的统计（成功率、延迟分布、批次大小等）

---

## 十、最小实践示例（阅读思路）

下面给出一个“如何使用”的最小示意，便于你把数据流转起来（示意代码，仅用于理解，不会写入项目）。

```go
// 创建 Provider（示意），然后创建 Subscriber
prov := tencent.NewProvider(...)
sub := subscriber.NewSubscriber(prov)

// 启动后台调度
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
_ = sub.Start(ctx)

// 订阅两只股票，每 3 秒与 5 秒更新一次
_ = sub.Subscribe("600000", 3*time.Second, func(data subscriber.StockData) error {
    // 你的业务处理
    fmt.Println("600000 =>", data.Price)
    return nil
})
_ = sub.Subscribe("000001", 5*time.Second, func(data subscriber.StockData) error {
    fmt.Println("000001 =>", data.Price)
    return nil
})

// 单独消费事件
go func() {
    for ev := range sub.GetEventChannel() {
        fmt.Printf("EVENT: %v %s\n", ev.Type, ev.Symbol)
    }
}()

// ...运行一段时间后
// _ = sub.Unsubscribe("600000")
// _ = sub.Stop()
```

阅读建议：
- 对照本文件的函数顺序依次跳读：Subscribe -> Start -> runSubscriptions -> fetchAndNotify -> notifyCallback/notifyError -> Stop
- 结合日志输出理解真实时序（建议开 debug 日志）

---

## 十一、代码片段与关键语句对照（你关心的区域）

- 固定 1 秒节拍与“到点判断”
```go
ticker := time.NewTicker(1 * time.Second) // 创建统一秒表（你可以考虑改为 s.minInterval）
...
lastFetchTime := make(map[string]time.Time) // 本地备忘录：记录上次抓取时间
...
for symbol, sub := range s.subscriptions {
    if !sub.Active { continue } // 非激活跳过
    lastFetch, exists := lastFetchTime[symbol]
    if !exists || now.Sub(lastFetch) >= sub.Interval {
        symbolsToFetch = append(symbolsToFetch, symbol) // 到点，加入本次批次
        lastFetchTime[symbol] = now // 立即更新时间，避免重复入队
    }
}
if len(symbolsToFetch) > 0 {
    go s.fetchAndNotify(symbolsToFetch) // 异步抓取与分发
}
```

- 批量抓取与分发
```go
ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second) // 30 秒超时兜底
data, err := s.provider.FetchData(ctx, symbols)            // Provider 批量抓取
...
dataMap := make(map[string]StockData) // 按 symbol 建索引
for _, stock := range data {
    dataMap[stock.Symbol] = stock
}
...
if stockData, found := dataMap[symbol]; found {
    go s.notifyCallback(sub, stockData) // 每只股票独立回调
} else {
    go s.notifyError(symbol, fmt.Errorf("no data received for symbol %s", symbol))
}
```

- 回调保护与事件投递
```go
defer func() { if r := recover(); r != nil { ... } }() // 回调崩溃不影响主流程
if err := sub.Callback(data); err != nil {
    s.notifyError(sub.Symbol, err) // 回调报错 -> 错误事件
}
select {
case s.eventChan <- UpdateEvent{ Type: EventTypeData, ... }: // 最佳努力
default: /* 通道满则丢弃 */ 
}
```

---

如需我把本说明扩展到 `types.go`（事件类型、Provider/Subscription 定义）或 Provider 实现（如腾讯数据源）的代码阅读指南，请告诉我具体路径与优先级。