# 简化装饰器使用指南

StockSub 提供了专门针对股票API业务需求的简化装饰器系统，包括简化频率控制装饰器和简化熔断器装饰器。这些装饰器集成了交易时段控制和IP封禁处理逻辑，为股票数据采集提供了更专业的保护机制。

## 简化频率控制装饰器 (SimplifiedFrequencyControlProvider)

### 功能特性

- **基础频率控制**: 确保请求间隔不小于配置的最小值
- **交易时段感知**: 只在股票交易时间段内允许请求
- **IP封禁检测与处理**: 自动检测IP封禁错误并支持重试机制
- **智能重试**: 结合智能限流器实现更智能的重试策略

### 配置选项

```go
type SimplifiedFrequencyControlConfig struct {
    // 基础频率限制
    MinInterval time.Duration // 最小请求间隔
    MaxRetries  int           // 最大重试次数
    Enabled     bool          // 是否启用

    // 交易时段相关
    MarketTimeAware  bool          // 是否启用交易时段感知
    PreMarketBuffer  time.Duration // 交易开始前缓冲时间
    PostMarketBuffer time.Duration // 交易结束后缓冲时间

    // IP封禁处理
    IPBanRetryInterval time.Duration // IP封禁后重试间隔
    IPBanRetryMax      int           // IP封禁最大重试次数
}
```

### 使用示例

```go
// 创建基础提供商
baseProvider := tencent.NewProvider()

// 创建配置
config := decorators.DefaultSimplifiedFrequencyControlConfig()
config.MinInterval = 200 * time.Millisecond
config.MarketTimeAware = true

// 创建装饰器
decoratedProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, config)

// 使用装饰器
ctx := context.Background()
data, err := decoratedProvider.FetchStockData(ctx, []string{"600000"})
```

## 简化熔断器装饰器 (SimplifiedCircuitBreakerProvider)

### 功能特性

- **熔断保护**: 在连续失败达到阈值时自动熔断
- **自动恢复**: 熔断超时后自动尝试恢复
- **IP封禁统计**: 统计IP封禁事件
- **交易时段感知**: 结合交易时段控制

### 配置选项

```go
type SimplifiedCircuitBreakerConfig struct {
    Name        string        // 熔断器名称
    MaxRequests uint32        // 半开状态下的最大请求数
    Interval    time.Duration // 统计窗口时间
    Timeout     time.Duration // 熔断器打开后的超时时间
    ReadyToTrip uint32        // 触发熔断的失败次数阈值
    Enabled     bool          // 是否启用熔断器

    // 交易时段相关
    MarketTimeAware bool // 是否启用交易时段感知
}
```

### 使用示例

```go
// 创建基础提供商
baseProvider := tencent.NewProvider()

// 创建配置
config := decorators.DefaultSimplifiedCircuitBreakerConfig()
config.ReadyToTrip = 3
config.MarketTimeAware = true

// 创建装饰器
decoratedProvider := decorators.NewSimplifiedCircuitBreakerProvider(baseProvider, config)

// 使用装饰器
ctx := context.Background()
data, err := decoratedProvider.FetchStockData(ctx, []string{"600000"})
```

## 组合使用

可以将简化频率控制装饰器和简化熔断器装饰器组合使用，以提供多层次的保护：

```go
// 创建基础提供商
baseProvider := tencent.NewProvider()

// 创建频率控制装饰器
freqConfig := decorators.DefaultSimplifiedFrequencyControlConfig()
freqProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, freqConfig)

// 创建熔断器装饰器
cbConfig := decorators.DefaultSimplifiedCircuitBreakerConfig()
cbProvider := decorators.NewSimplifiedCircuitBreakerProvider(freqProvider, cbConfig)

// 使用组合装饰器
ctx := context.Background()
data, err := cbProvider.FetchStockData(ctx, []string{"600000"})
```

## 配置模板

### 默认配置

```go
func DefaultSimplifiedConfig() *SimplifiedDecoratorConfig {
    return &SimplifiedDecoratorConfig{
        FrequencyControl: &SimplifiedFrequencyControlConfig{
            MinInterval:        200 * time.Millisecond,
            MaxRetries:         3,
            Enabled:            true,
            MarketTimeAware:    true,
            PreMarketBuffer:    5 * time.Minute,
            PostMarketBuffer:   10 * time.Minute,
            IPBanRetryInterval: 5 * time.Minute,
            IPBanRetryMax:      3,
        },
        CircuitBreaker: &SimplifiedCircuitBreakerConfig{
            Name:            "SimplifiedStockProvider",
            MaxRequests:     5,
            Interval:        60 * time.Second,
            Timeout:         30 * time.Second,
            ReadyToTrip:     5,
            Enabled:         true,
            MarketTimeAware: true,
        },
        Enabled: true,
    }
}
```

### 生产环境配置

```go
func ProductionSimplifiedConfig() *SimplifiedDecoratorConfig {
    return &SimplifiedDecoratorConfig{
        FrequencyControl: &SimplifiedFrequencyControlConfig{
            MinInterval:        1 * time.Second, // 生产环境使用更长的间隔
            MaxRetries:         5,
            Enabled:            true,
            MarketTimeAware:    true,
            PreMarketBuffer:    10 * time.Minute,
            PostMarketBuffer:   15 * time.Minute,
            IPBanRetryInterval: 10 * time.Minute,
            IPBanRetryMax:      5,
        },
        CircuitBreaker: &SimplifiedCircuitBreakerConfig{
            Name:            "ProductionSimplifiedStockProvider",
            MaxRequests:     3,
            Interval:        120 * time.Second,
            Timeout:         60 * time.Second,
            ReadyToTrip:     3,
            Enabled:         true,
            MarketTimeAware: true,
        },
        Enabled: true,
    }
}
```

## 最佳实践

1. **生产环境**: 使用 `ProductionSimplifiedConfig()` 获取生产环境配置
2. **测试环境**: 使用 `TestSimplifiedConfig()` 获取测试环境配置
3. **监控**: 定期检查装饰器的状态信息
4. **组合使用**: 根据业务需求组合使用多个装饰器
5. **配置调整**: 根据实际运行情况调整配置参数

## 相关文档

- [装饰器接口定义](../../pkg/provider/interfaces.go)
- [装饰器实现](../../pkg/provider/decorators/)
- [配置系统](../../pkg/provider/decorators/config.go)
- [装饰器工厂](../../pkg/provider/decorators/factory.go)
- [示例代码](../../examples/decorator/simplified/)