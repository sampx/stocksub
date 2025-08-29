# 提供商示例 (Provider Examples)

这个目录包含了展示如何使用不同类型数据提供商的示例代码。这些示例演示了重构后的提供商模块的使用方法，包括基础提供商、装饰器模式，以及不同的配置选项。

## 目录结构

```
examples/provider/
├── README.md                    # 本文件
├── realtime_stock/             # 实时股票数据提供商示例
│   └── main.go
├── historical_data/            # 历史数据提供商示例
│   └── main.go
└── index_data/                 # 指数数据提供商示例
    └── main.go
```

## 示例说明

### 1. 实时股票数据提供商示例 (`realtime_stock/`)

展示如何使用实时股票数据提供商：

- **基础功能演示**：创建腾讯提供商，检查健康状态和符号支持
- **数据获取**：演示 `FetchStockData()` 和 `FetchStockDataWithRaw()` 方法
- **装饰器使用**：展示如何应用默认和生产环境装饰器配置
- **配置对比**：比较不同环境下的装饰器设置

**运行示例**：
```bash
cd examples/provider/realtime_stock
go run main.go
```

**主要特性**：
- 腾讯数据源集成
- 频率控制装饰器
- 熔断器装饰器
- 错误处理演示

### 2. 历史数据提供商示例 (`historical_data/`)

展示如何使用历史数据提供商接口：

- **模拟提供商**：使用自定义的模拟历史数据提供商
- **时间序列数据**：生成和处理历史K线数据
- **装饰器应用**：演示历史数据提供商的装饰器使用
- **数据分析**：基本的历史数据统计分析

**运行示例**：
```bash
cd examples/provider/historical_data
go run main.go
```

**主要特性**：
- 多时间周期支持（1d, 1w, 1M）
- 历史数据生成算法
- 统计分析功能
- 装饰器链应用

### 3. 指数数据提供商示例 (`index_data/`)

展示如何使用指数数据提供商接口：

- **指数数据模拟**：支持主要中国股指（上证、深证、创业板等）
- **市场分析**：整体市场表现分析
- **定时获取**：演示定时获取指数数据
- **错误处理**：各种异常情况的处理

**运行示例**：
```bash
cd examples/provider/index_data
go run main.go
```

**主要特性**：
- 多指数支持
- 市场分析算法
- 成交量/成交额统计
- 实时数据模拟

## 装饰器配置说明

### 默认配置 (`DefaultDecoratorConfig`)
适用于开发和测试环境：
- 频率控制：200ms 最小间隔
- 熔断器：较宽松的设置

### 生产环境配置 (`ProductionDecoratorConfig`)
适用于生产环境：
- 实时数据：5秒最小间隔
- 历史数据：10秒最小间隔
- 更严格的熔断器设置

### 监控配置 (`MonitoringDecoratorConfig`)
适用于长期监控场景：
- 更长的请求间隔
- 更高的失败容忍度

## 接口层次结构

```
Provider (基础接口)
├── RealtimeStockProvider (实时股票数据)
├── HistoricalProvider (历史数据)
└── RealtimeIndexProvider (实时指数数据)

装饰器支持：
├── FrequencyControlProvider (频率控制)
├── CircuitBreakerProvider (熔断器)
└── ConfigurableDecoratorChain (可配置装饰器链)
```

## 使用模式

### 1. 基础使用
```go
// 创建提供商
provider := tencent.NewClient()

// 获取数据
data, err := provider.FetchStockData(ctx, symbols)
```

### 2. 装饰器使用
```go
// 应用装饰器
decoratedProvider, err := decorators.CreateDecoratedProvider(
    provider, 
    decorators.ProductionDecoratorConfig(),
)

// 通过装饰器获取数据
data, err := decoratedProvider.(provider.RealtimeStockProvider).FetchStockData(ctx, symbols)
```

### 3. 自定义配置
```go
// 自定义装饰器配置
customConfig := provider.ProviderDecoratorConfig{
    All: []provider.DecoratorConfig{
        {
            Type:     provider.FrequencyControlType,
            Enabled:  true,
            Priority: 1,
            Config: map[string]interface{}{
                "min_interval_ms": 1000,
                "max_retries":     5,
            },
        },
    },
}
```

## 环境要求

- Go 1.19+
- 网络连接（实时股票示例需要访问腾讯API）
- 依赖包：
  - `stocksub/pkg/core`
  - `stocksub/pkg/provider`
  - `stocksub/pkg/provider/decorators`
  - `stocksub/pkg/provider/tencent`

## 故障排除

### 常见问题

1. **网络连接失败**
   - 检查网络连接
   - 确认防火墙设置
   - 验证API端点可访问性

2. **编译错误**
   - 确保所有依赖包已正确安装
   - 检查Go模块路径
   - 验证import路径正确

3. **数据获取失败**
   - 检查股票代码格式
   - 确认提供商支持该代码
   - 查看错误日志详情

### 调试模式
设置环境变量启用调试输出：
```bash
export DEBUG=1
go run main.go
```

## 扩展开发

### 创建自定义提供商
1. 实现相应的接口（`RealtimeStockProvider`、`HistoricalProvider` 等）
2. 添加必要的配置和初始化逻辑
3. 实现数据获取和解析逻辑
4. 添加错误处理和健康检查

### 创建自定义装饰器
1. 实现 `Decorator` 接口
2. 添加装饰器工厂函数
3. 在配置系统中注册新的装饰器类型
4. 编写相应的单元测试

## 相关文档

- [设计文档](../../.spec-workflow/specs/provider-module-refactor/design.md)
- [API参考](../../pkg/provider/interfaces.go)
- [装饰器文档](../../pkg/provider/decorators/)