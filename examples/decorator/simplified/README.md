# 简化装饰器示例

这个目录包含了简化频率控制和熔断器装饰器的使用示例。简化装饰器提供了更直观、更易用的接口，适用于大多数常见场景。

## 文件说明

- `comprehensive_example.go` - 综合示例，演示所有简化装饰器的功能

## 运行示例

```bash
# 运行综合示例
go run examples/decorator/simplified/comprehensive_example.go
```

## 简化装饰器完整用法

### 1. 创建基础提供商

首先，需要创建一个基础股票数据提供商，可以是腾讯、新浪等具体实现：

```go
// 使用腾讯数据提供商
baseProvider := tencent.NewClient()

// 或者使用新浪数据提供商
// baseProvider := sina.NewClient()

// 也可以创建模拟提供商用于测试
// baseProvider := NewMockStockProvider(0)
```

### 2. 创建简化频率控制装饰器

```go
// 1. 创建默认配置
freqConfig := decorators.DefaultSimplifiedFrequencyControlConfig()

// 2. 根据需求调整配置
freqConfig.MinInterval = 200 * time.Millisecond          // 最小请求间隔
freqConfig.MaxRetries = 3                                // 最大重试次数
freqConfig.MarketTimeAware = true                        // 启用交易时段感知
freqConfig.PreMarketBuffer = 5 * time.Minute             // 开盘前缓冲时间
freqConfig.PostMarketBuffer = 10 * time.Minute           // 收盘后缓冲时间
freqConfig.IPBanRetryInterval = 5 * time.Minute          // IP封禁后重试间隔
freqConfig.IPBanRetryMax = 3                             // IP封禁后最大重试次数

// 3. 创建频率控制装饰器
freqProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, freqConfig)
```

### 3. 创建简化熔断器装饰器

```go
// 1. 创建默认配置
cbConfig := decorators.DefaultSimplifiedCircuitBreakerConfig()

// 2. 根据需求调整配置
cbConfig.Name = "StockProvider"                         // 熔断器名称
cbConfig.MaxRequests = 5                                // 半开状态最大请求数
cbConfig.Interval = 60 * time.Second                   // 统计窗口时间
cbConfig.Timeout = 30 * time.Second                      // 熔断器超时时间
cbConfig.ReadyToTrip = 3                                // 触发熔断失败次数
cbConfig.MarketTimeAware = true                         // 启用交易时段感知

// 3. 创建熔断器装饰器
cbProvider := decorators.NewSimplifiedCircuitBreakerProvider(baseProvider, cbConfig)
```

### 4. 组合使用装饰器

可以将多个装饰器组合使用，形成多层保护机制：

```go
// 创建基础提供商
baseProvider := tencent.NewClient()

// 创建频率控制装饰器
freqConfig := decorators.DefaultSimplifiedFrequencyControlConfig()
freqConfig.MinInterval = 200 * time.Millisecond
freqProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, freqConfig)

// 创建熔断器装饰器，以频率控制装饰器为基础
cbConfig := decorators.DefaultSimplifiedCircuitBreakerConfig()
cbConfig.ReadyToTrip = 3
cbProvider := decorators.NewSimplifiedCircuitBreakerProvider(freqProvider, cbConfig)

// 现在cbProvider是经过频率控制和熔断器双重保护的提供商
```

### 5. 使用装饰器获取数据

```go
ctx := context.Background()
symbols := []string{"600000", "000001"} // 浦发银行、平安银行

// 获取股票数据
stockData, err := cbProvider.FetchStockData(ctx, symbols)
if err != nil {
    // 处理错误
    log.Printf("获取股票数据失败: %v", err)
    return
}

// 使用数据
for _, stock := range stockData {
    fmt.Printf("%s: %.2f
", stock.Symbol, stock.Price)
}

// 获取股票数据和原始响应
stockData, raw, err := cbProvider.FetchStockDataWithRaw(ctx, symbols)
if err != nil {
    log.Printf("获取股票数据失败: %v", err)
    return
}

// 使用原始数据进行调试
fmt.Printf("原始响应: %s\n", raw)
```

### 6. 监控装饰器状态

```go
// 获取装饰器状态
status := cbProvider.GetStatus()

// 解析状态信息
if freqStatus, ok := status["frequency_control"].(map[string]interface{}); ok {
    fmt.Printf("频率控制状态: 最小间隔=%v, IP封禁=%v\n", 
        freqStatus["min_interval"], freqStatus["ip_ban_status"])
}

if cbStatus, ok := status["circuit_breaker"].(map[string]interface{}); ok {
    fmt.Printf("熔断器状态: 状态=%v, 失败次数=%v\n", 
        cbStatus["state"], cbStatus["failure_count"])
}

// 获取熔断器状态
state := cbProvider.GetState()
fmt.Printf("熔断器当前状态: %v\n", state)
```

### 7. 使用装饰器工厂创建

装饰器工厂提供了更便捷的创建方式：

```go
// 创建装饰器工厂
factory := decorators.NewDecoratorFactory()

// 创建简化装饰器配置
type SimplifiedDecoratorConfig struct {
    Enabled bool
    FrequencyControl *SimplifiedFrequencyControlConfig
    CircuitBreaker *SimplifiedCircuitBreakerConfig
}

// 设置配置
config := &SimplifiedDecoratorConfig{
    Enabled: true,
    FrequencyControl: &SimplifiedFrequencyControlConfig{
        MinInterval: 200 * time.Millisecond,
        MaxRetries: 3,
        MarketTimeAware: true,
    },
    CircuitBreaker: &SimplifiedCircuitBreakerConfig{
        ReadyToTrip: 3,
        Timeout: 30 * time.Second,
        MarketTimeAware: true,
    },
}

// 使用工厂创建装饰器
decoratedProvider, err := factory.CreateSimplifiedDecorators(baseProvider, config)
if err != nil {
    log.Fatal(err)
}
```

### 8. 从配置文件加载

可以从配置文件（如YAML）加载装饰器配置：

```yaml
# config.yaml
provider:
  decorators:
    enabled: true
    frequency_control:
      min_interval: "200ms"
      max_retries: 3
      market_time_aware: true
      pre_market_buffer: "5m"
      post_market_buffer: "10m"
      ip_ban_retry_interval: "5m"
      ip_ban_retry_max: 3
    circuit_breaker:
      name: "StockProvider"
      max_requests: 5
      interval: "60s"
      timeout: "30s"
      ready_to_trip: 3
      market_time_aware: true
```

```go
// 使用Viper加载配置
v := viper.New()
v.SetConfigFile("config.yaml")
if err := v.ReadInConfig(); err != nil {
    log.Fatal(err)
}

// 使用工厂从配置创建装饰器
factory := decorators.NewDecoratorFactory()
decoratedProvider, err := factory.CreateFromViper(baseProvider, v, "provider.decorators")
if err != nil {
    log.Fatal(err)
}
```

### 9. 预定义配置

简化装饰器提供了一些预定义配置，方便快速使用：

```go
// 生产环境配置
productionConfig := decorators.ProductionSimplifiedConfig()
productionProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, productionConfig.FrequencyControl)

// 测试环境配置
testConfig := decorators.TestSimplifiedConfig()
testProvider := decorators.NewSimplifiedCircuitBreakerProvider(baseProvider, testConfig.CircuitBreaker)

// 快速创建组合装饰器
combinedProvider := decorators.NewSimplifiedCircuitBreakerProvider(
    decorators.NewSimplifiedFrequencyControlProvider(baseProvider, productionConfig.FrequencyControl),
    productionConfig.CircuitBreaker,
)
```

## 示例内容

### 1. 简化频率控制装饰器演示

展示如何使用 `SimplifiedFrequencyControlProvider`：

- 自动频率控制，确保请求间隔不小于最小值
- 交易时段感知，只在交易时间段内允许请求
- IP封禁检测和处理，支持重试机制
- 灵活的配置选项和运行时调整

### 2. 简化熔断器装饰器演示

展示如何使用 `SimplifiedCircuitBreakerProvider`：

- 熔断保护，在连续失败时自动熔断
- 自动恢复，熔断超时后自动尝试恢复
- IP封禁事件统计和跟踪
- 交易时段感知功能

### 3. 组合使用演示

展示如何将频率控制和熔断器组合使用：

- 频率控制 + 熔断器协同工作
- 多层保护机制
- 完整的状态监控

### 4. 配置系统演示

展示如何使用配置系统创建装饰器：

- 配置驱动的装饰器创建
- 预定义配置模板
- 灵活的配置选项

## 主要特性

### 频率控制装饰器

```go
config := decorators.DefaultSimplifiedFrequencyControlConfig()
config.MinInterval = 200 * time.Millisecond
config.MarketTimeAware = true

decoratedProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, config)
```

**特性：**
- ✅ 最小请求间隔控制
- ✅ 交易时段感知
- ✅ IP封禁自动检测和处理
- ✅ 智能重试机制
- ✅ 详细状态监控

### 熔断器装饰器

```go
config := decorators.DefaultSimplifiedCircuitBreakerConfig()
config.ReadyToTrip = 3
config.MarketTimeAware = true

decoratedProvider := decorators.NewSimplifiedCircuitBreakerProvider(baseProvider, config)
```

**特性：**
- ✅ 熔断保护机制
- ✅ 自动恢复功能
- ✅ IP封禁统计
- ✅ 交易时段感知
- ✅ 完整状态信息

## 配置选项

### 频率控制配置

```go
type SimplifiedFrequencyControlConfig struct {
    MinInterval        time.Duration // 最小请求间隔
    MaxRetries         int          // 最大重试次数
    Enabled            bool         // 是否启用
    MarketTimeAware    bool         // 交易时段感知
    PreMarketBuffer    time.Duration // 交易前缓冲时间
    PostMarketBuffer   time.Duration // 交易后缓冲时间
    IPBanRetryInterval time.Duration // IP封禁重试间隔
    IPBanRetryMax      int          // IP封禁最大重试次数
}
```

### 熔断器配置

```go
type SimplifiedCircuitBreakerConfig struct {
    Name             string        // 熔断器名称
    MaxRequests      uint32        // 半开状态最大请求数
    Interval         time.Duration // 统计窗口时间
    Timeout          time.Duration // 熔断器超时时间
    ReadyToTrip      uint32        // 触发熔断失败次数
    Enabled          bool          // 是否启用
    MarketTimeAware  bool          // 交易时段感知
}
```

## 使用建议

1. **生产环境**：使用 `ProductionSimplifiedConfig()` 获取生产环境配置
2. **测试环境**：使用 `TestSimplifiedConfig()` 获取测试环境配置
3. **自定义配置**：根据具体需求调整配置参数
4. **监控告警**：定期检查装饰器的状态信息
5. **组合使用**：根据业务需求组合使用多个装饰器
6. **错误处理**：实现适当的错误处理逻辑，特别是对IP封禁等特定错误
7. **性能监控**：监控请求延迟、成功率等指标，及时调整配置

## 常见问题与解决方案

### Q: 如何处理IP封禁错误？

A: 简化频率控制装饰器内置了IP封禁检测和处理机制，当检测到IP封禁时，会自动等待并重试。可以通过配置 `IPBanRetryInterval` 和 `IPBanRetryMax` 调整重试策略。

### Q: 交易时段感知如何工作？

A: 当启用 `MarketTimeAware` 后，装饰器会根据当前时间判断是否在交易时段内，只在交易时段内或缓冲时间内允许请求。交易时段默认为工作日的9:30-15:00。

### Q: 如何监控装饰器状态？

A: 使用 `GetStatus()` 方法获取装饰器的详细状态信息，包括请求统计、错误类型、熔断器状态等。可以定期记录这些信息用于监控和告警。

## 相关文档

- [装饰器接口定义](../../pkg/provider/interfaces.go)
- [装饰器实现](../../pkg/provider/decorators/)
- [配置系统](../../pkg/provider/decorators/config.go)
- [装饰器工厂](../../pkg/provider/decorators/factory.go)
- [腾讯股票数据提供商](../../pkg/provider/tencent/)
- [新浪股票数据提供商](../../pkg/provider/sina/)