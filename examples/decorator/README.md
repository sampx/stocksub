# 装饰器示例

本目录包含了装饰器模式在股票数据提供商中的各种使用示例。装饰器模式允许在不修改原始提供商代码的情况下，动态地添加功能，如频率控制、熔断器等。

## 目录结构

```
examples/decorator/
├── README.md                    # 本文档
├── frequency_control/           # 频率控制装饰器示例
│   └── main.go
├── circuit_breaker/            # 熔断器装饰器示例
│   └── main.go
├── chain_composition/          # 装饰器链组合示例
│   └── main.go
└── configuration/              # 配置驱动装饰器示例
    └── main.go
```

## 装饰器类型

### 1. 频率控制装饰器 (FrequencyControlProvider)

**用途**: 控制对底层API的请求频率，防止超出API限制。

**特性**:
- 设置最小请求间隔
- 智能重试机制
- 支持动态配置调整
- 集成市场时间感知的智能限流

**适用场景**:
- API有频率限制的提供商
- 需要保护底层服务的场景
- 实时数据获取场景

### 2. 熔断器装饰器 (CircuitBreakerProvider)

**用途**: 当底层服务出现故障时，快速失败并防止雪崩效应。

**特性**:
- 三种状态：关闭、打开、半开
- 可配置的失败阈值和恢复时间
- 详细的统计信息
- 状态变更回调

**适用场景**:
- 外部服务调用
- 网络不稳定的环境
- 微服务架构中的服务保护

## 示例说明

### 频率控制示例 (`frequency_control/main.go`)

展示了如何使用频率控制装饰器：

```bash
cd examples/decorator/frequency_control
go run main.go
```

**主要功能**:
- 基础频率控制演示
- 动态配置调整
- 历史数据提供商的频率控制
- Mock Provider的集成使用

**输出示例**:
```
=== 频率控制装饰器示例 ===
装饰器名称: FrequencyControl(SimpleRealtimeStock)
频率限制: 500ms
健康状态: true

=== 测试频率控制效果 ===
第1次请求开始...
成功获取2条数据，耗时: 501ms
  000001.SZ: ¥12.34 (变化: 0.56, 4.75%)
  000002.SZ: ¥23.45 (变化: -0.12, -0.51%)
```

### 熔断器示例 (`circuit_breaker/main.go`)

展示了熔断器的完整生命周期：

```bash
cd examples/decorator/circuit_breaker
go run main.go
```

**主要功能**:
- 熔断器状态演示（关闭->打开->半开->关闭）
- 失败恢复机制
- 配置调整和敏感度测试
- 统计信息展示

**输出示例**:
```
=== 熔断器装饰器示例 ===
装饰器名称: CircuitBreaker(UnreliableStock)
健康状态: true
熔断器状态: StateClosed

=== 测试熔断器效果 ===
第1次请求...
请求失败: 模拟的API错误 - 失败次数: 1
熔断器状态: StateClosed, 连续失败: 1, 总请求: 1
```

### 装饰器链组合示例 (`chain_composition/main.go`)

展示了如何组合多个装饰器：

```bash
cd examples/decorator/chain_composition
go run main.go
```

**主要功能**:
- 手动装饰器链组合
- 配置驱动的装饰器链
- 不同装饰器顺序的影响
- 复杂多层装饰器结构

**装饰器组合方式**:
1. **手动组合**: `FrequencyControl -> CircuitBreaker -> BaseProvider`
2. **配置驱动**: 使用`ProviderDecoratorConfig`定义
3. **动态组合**: 运行时添加和调整装饰器

### 配置示例 (`configuration/main.go`)

展示了各种配置模式：

```bash
cd examples/decorator/configuration
go run main.go
```

**配置类型**:
- **默认配置**: 开发和原型阶段
- **生产环境配置**: 严格的限制和长超时
- **测试环境配置**: 禁用装饰器便于测试
- **监控环境配置**: 长期监控优化
- **自定义配置**: 特定需求定制
- **文件配置**: YAML/JSON格式加载
- **动态配置**: 运行时调整

## 配置参数说明

### 频率控制配置 (FrequencyControlConfig)

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `min_interval` | `time.Duration` | `200ms` | 最小请求间隔 |
| `max_retries` | `int` | `3` | 最大重试次数 |
| `enabled` | `bool` | `true` | 是否启用频率控制 |

### 熔断器配置 (CircuitBreakerConfig)

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `name` | `string` | `"StockProvider"` | 熔断器名称 |
| `max_requests` | `uint32` | `5` | 半开状态最大请求数 |
| `interval` | `time.Duration` | `60s` | 统计窗口时间 |
| `timeout` | `time.Duration` | `30s` | 熔断超时时间 |
| `ready_to_trip` | `uint32` | `5` | 触发熔断的失败次数阈值 |
| `enabled` | `bool` | `true` | 是否启用熔断器 |

## 装饰器优先级

装饰器按照优先级数字从小到大的顺序应用：

1. **Priority 1**: 频率控制 (最内层)
2. **Priority 2**: 熔断器 (中间层)
3. **Priority 3**: 其他装饰器 (最外层)

## 最佳实践

### 1. 装饰器顺序选择

**推荐顺序**: `FrequencyControl -> CircuitBreaker -> BaseProvider`

**原因**:
- 频率控制在最内层可以有效控制对底层服务的压力
- 熔断器在外层可以快速响应服务故障，避免无意义的等待

### 2. 配置选择指南

| 环境 | 频率控制 | 熔断器 | 适用场景 |
|------|----------|--------|----------|
| 开发 | 宽松 (200ms) | 宽松 (5次失败) | 快速开发迭代 |
| 测试 | 禁用 | 禁用 | 功能验证 |
| 生产 | 严格 (5s) | 严格 (3次失败) | 稳定性优先 |
| 监控 | 中等 (3s) | 宽松 (10次失败) | 长期运行 |

### 3. 错误处理

```go
// 检查装饰器特定的错误
if err != nil {
    if strings.Contains(err.Error(), "circuit breaker is open") {
        // 熔断器打开，服务暂时不可用
        log.Warn("服务熔断中，请稍后重试")
    } else if strings.Contains(err.Error(), "频率限制") {
        // 频率控制限制
        log.Warn("请求频率过高，已限流")
    }
}
```

### 4. 监控和度量

装饰器提供了丰富的状态信息：

```go
// 获取熔断器状态
if cb, ok := provider.(*decorators.CircuitBreakerProvider); ok {
    status := cb.GetStatus()
    fmt.Printf("熔断器状态: %v\n", status)
}

// 获取频率控制状态
if fc, ok := provider.(*decorators.FrequencyControlProvider); ok {
    status := fc.GetStatus()
    fmt.Printf("频率控制状态: %v\n", status)
}
```

## 扩展装饰器

要创建自定义装饰器，需要实现以下接口：

```go
// 1. 实现Provider基础接口
type CustomDecorator struct {
    provider.RealtimeStockProvider
    *provider.BaseDecorator
    // 自定义字段
}

// 2. 实现必要的方法
func (c *CustomDecorator) Name() string {
    return fmt.Sprintf("Custom(%s)", c.RealtimeStockProvider.Name())
}

func (c *CustomDecorator) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
    // 在调用前后添加自定义逻辑
    // ...
    return c.RealtimeStockProvider.FetchStockData(ctx, symbols)
}
```

## 故障排查

### 常见问题

1. **装饰器不生效**
   - 检查`enabled`配置是否为`true`
   - 确认装饰器顺序是否正确

2. **频率控制过于严格**
   - 调整`min_interval`参数
   - 检查智能限流器的市场时间设置

3. **熔断器误触发**
   - 增加`ready_to_trip`阈值
   - 调整`interval`统计窗口

4. **配置不生效**
   - 检查配置文件格式
   - 验证Viper配置键路径

### 调试技巧

1. **启用详细日志**:
   ```go
   // 在装饰器创建时启用详细模式
   config.EnableDetailedLogging = true
   ```

2. **状态监控**:
   ```go
   // 定期输出装饰器状态
   ticker := time.NewTicker(10 * time.Second)
   go func() {
       for range ticker.C {
           fmt.Printf("装饰器状态: %+v\n", provider.GetStatus())
       }
   }()
   ```

## 性能考虑

1. **内存使用**: 每个装饰器会增加少量内存开销
2. **延迟影响**: 装饰器会引入微小的处理延迟
3. **并发安全**: 所有装饰器都是线程安全的

## 参考资料

- [装饰器模式](https://en.wikipedia.org/wiki/Decorator_pattern)
- [熔断器模式](https://martinfowler.com/bliki/CircuitBreaker.html)
- [频率限制最佳实践](https://cloud.google.com/apis/design/design_patterns#rate_limiting)