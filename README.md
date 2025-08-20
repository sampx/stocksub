# StockSub - 股票数据订阅器

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

StockSub 是一个高性能的股票数据订阅服务，支持实时获取A股、港股、美股等多市场股票数据。

## ✨ 特性

- 🚀 **高性能**: 基于Go语言，支持高并发订阅
- 🌐 **多市场**: 支持A股（上证、深证、北交所）、港股、美股
- ⚡ **实时数据**: 灵活的订阅间隔设置（最小1秒）
- 🔄 **智能重试**: 内置指数退避重试机制
- 📊 **统计监控**: 完整的订阅统计和健康监控
- 🛡️ **错误处理**: 自动故障恢复和订阅重启
- 📝 **日志系统**: 结构化日志，支持文件和控制台输出
- 🔌 **模块化**: 清晰的架构设计，易于扩展

## 🚀 快速开始

### 安装

```bash
git clone <repository>
cd stocksub
go mod download
```

### 基本使用

```go
package main

import (
    "context"
    "fmt"
    "time"

    "stocksub/pkg/provider/tencent"
    "stocksub/pkg/subscriber"
)

func main() {
    // 创建提供商和订阅器
    provider := tencent.NewProvider()
    sub := subscriber.NewSubscriber(provider)
    
    // 启动订阅器
    ctx := context.Background()
    sub.Start(ctx)
    
    // 订阅股票数据
    sub.Subscribe("600000", 5*time.Second, func(data subscriber.StockData) error {
        fmt.Printf("%s: ¥%.2f %+.2f (%.2f%%)\n", 
            data.Symbol, data.Price, data.Change, data.ChangePercent)
        return nil
    })
    
    // 保持运行...
}
```

### 高级使用（推荐）

```go
package main

import (
    "context"
    "time"

    "stocksub/pkg/config"
    "stocksub/pkg/logger"
    "stocksub/pkg/provider/tencent" 
    "stocksub/pkg/subscriber"
)

func main() {
    // 初始化配置和日志
    cfg := config.Default().SetDefaultInterval(6 * time.Second)
    logger.Init(cfg.Logger)
    
    // 创建管理器（推荐）
    provider := tencent.NewProvider()
    sub := subscriber.NewSubscriber(provider)
    manager := subscriber.NewManager(sub)
    
    // 启动服务
    ctx := context.Background()
    manager.Start(ctx)
    
    // 批量订阅
    requests := []subscriber.SubscribeRequest{
        {Symbol: "600000", Interval: 6*time.Second, Callback: stockCallback},
        {Symbol: "AAPL", Interval: 6*time.Second, Callback: stockCallback},
    }
    manager.SubscribeBatch(requests)
    
    // 获取统计信息
    stats := manager.GetStatistics()
    logger.Info("订阅数: %d", stats.ActiveSubscriptions)
}

func stockCallback(data subscriber.StockData) error {
    logger.Info("%s: ¥%.2f %+.2f (%.2f%%)", 
        data.Symbol, data.Price, data.Change, data.ChangePercent)
    return nil
}
```

## 🏗️ 架构设计

```
stocksub/
├── cmd/                    # 命令行工具
│   └── stocksub/          # 主程序
├── pkg/                   # 核心库
│   ├── subscriber/        # 订阅器核心
│   │   ├── types.go      # 数据类型定义  
│   │   ├── subscriber.go # 订阅器实现
│   │   └── manager.go    # 管理器（推荐）
│   ├── provider/         # 数据提供商
│   │   └── tencent/      # 腾讯数据源
│   ├── config/           # 配置管理
│   └── logger/           # 日志系统
└── examples/             # 示例代码
```

### 核心组件

1. **Provider（数据提供商）**: 负责从外部API获取股票数据
2. **Subscriber（订阅器）**: 管理股票订阅和数据分发
3. **Manager（管理器）**: 高级功能，包括统计、健康检查、自动重启
4. **Config（配置）**: 统一的配置管理
5. **Logger（日志）**: 结构化日志系统

## 📈 支持的股票市场

| 市场 | 格式示例 | 说明 |
|------|----------|------|
| **A股上证** | `600000`, `601398` | 6开头的6位数字 |
| **A股深证** | `000001`, `300750` | 0/3开头的6位数字 |  
| **A股北交** | `835174`, `832000` | 4/8开头的6位数字 |
| **港股** | `00700`, `03690` | 5位数字 |
| **美股** | `AAPL`, `TSLA` | 1-5位字母 |

## ⚙️ 配置说明

### 基础配置

```go
config := config.Default()
config.SetDefaultInterval(5 * time.Second)    // 默认订阅间隔
config.SetMaxSubscriptions(50)                // 最大订阅数
config.SetRateLimit(200 * time.Millisecond)  // 请求频率限制
```

### 日志配置

```go
config.Logger.Level = "info"        // debug, info, warn, error
config.Logger.Output = "both"       // console, file, both  
config.Logger.Filename = "app.log"  // 日志文件名
```

## 🔧 命令行工具

### 构建

```bash
# 构建主程序
go build -o stocksub ./cmd/stocksub

# 构建示例程序
go build -o simple-example ./examples/simple
go build -o advanced-example ./examples/advanced
```

### 运行

```bash
# 运行主程序
./stocksub

# 运行示例
./simple-example
./advanced-example

# 直接运行
go run ./cmd/stocksub
go run ./examples/simple
```

### 交叉编译

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o stocksub-linux ./cmd/stocksub

# Windows  
GOOS=windows GOARCH=amd64 go build -o stocksub.exe ./cmd/stocksub

# macOS
GOOS=darwin GOARCH=amd64 go build -o stocksub-macos ./cmd/stocksub
```

## 📊 监控和统计

### 获取统计信息

```go
stats := manager.GetStatistics()
fmt.Printf("总订阅: %d\n", stats.TotalSubscriptions)
fmt.Printf("活跃订阅: %d\n", stats.ActiveSubscriptions) 
fmt.Printf("数据点总数: %d\n", stats.TotalDataPoints)
fmt.Printf("错误总数: %d\n", stats.TotalErrors)

// 单个股票统计
for symbol, subStats := range stats.SubscriptionStats {
    fmt.Printf("%s: 数据=%d, 错误=%d, 健康=%v\n", 
        symbol, subStats.DataPointCount, subStats.ErrorCount, subStats.IsHealthy)
}
```

### 事件监控

```go
eventChan := subscriber.GetEventChannel()

for event := range eventChan {
    switch event.Type {
    case subscriber.EventTypeData:
        fmt.Printf("收到数据: %s\n", event.Symbol)
    case subscriber.EventTypeError:
        fmt.Printf("错误: %s - %v\n", event.Symbol, event.Error)
    }
}
```

## 🛠️ API 参考

### 订阅器接口

```go
type Subscriber interface {
    Subscribe(symbol string, interval time.Duration, callback CallbackFunc) error
    Unsubscribe(symbol string) error
    Start(ctx context.Context) error
    Stop() error
    GetSubscriptions() []Subscription
}
```

### 数据结构

```go
type StockData struct {
    Symbol        string    `json:"symbol"`         // 股票代码
    Name          string    `json:"name"`           // 股票名称
    Price         float64   `json:"price"`          // 当前价格
    Change        float64   `json:"change"`         // 涨跌额  
    ChangePercent float64   `json:"change_percent"` // 涨跌幅
    Volume        int64     `json:"volume"`         // 成交量
    Turnover      float64   `json:"turnover"`       // 成交额
    Open          float64   `json:"open"`           // 开盘价
    High          float64   `json:"high"`           // 最高价
    Low           float64   `json:"low"`            // 最低价
    PrevClose     float64   `json:"prev_close"`     // 昨收价
    MarketCap     float64   `json:"market_cap"`     // 市值
    PE            float64   `json:"pe"`             // 市盈率
    PB            float64   `json:"pb"`             // 市净率
    Timestamp     time.Time `json:"timestamp"`      // 时间戳
}
```

## 📋 最佳实践

1. **使用Manager**: 推荐使用 `Manager` 而不是直接使用 `Subscriber`
2. **合理间隔**: 订阅间隔建议设置为3-10秒，避免过于频繁
3. **批量操作**: 使用 `SubscribeBatch` 进行批量订阅
4. **错误处理**: 在回调函数中妥善处理错误
5. **监控统计**: 定期检查统计信息和健康状态
6. **优雅退出**: 使用 `context.Context` 进行优雅关闭

## 🔍 故障排除

### 常见问题

1. **订阅失败**
   ```
   Error: symbol XXX is not supported
   ```
   检查股票代码格式是否正确

2. **数据获取失败**
   ```  
   Error: HTTP request failed
   ```
   检查网络连接或增加重试次数

3. **频率限制**
   ```
   Warning: rate limit exceeded
   ```
   增加订阅间隔或减少并发订阅数

### 调试模式

```go
config.SetLogLevel("debug")  // 启用调试日志
```

## 🚧 性能指标

- **内存占用**: < 50MB (100个订阅)
- **CPU占用**: < 5% (正常负载)
- **网络延迟**: 200-500ms (取决于订阅间隔)
- **支持订阅**: 最多1000个股票同时订阅
- **数据延迟**: 1-3秒 (腾讯接口延迟)

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件