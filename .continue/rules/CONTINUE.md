# StockSub 项目开发指南

## 1. 项目概述

StockSub 是一个高性能的股票数据订阅服务，支持实时获取A股、港股、美股等多市场股票数据。

### 关键技术
- **语言**: Go 1.23+
- **核心依赖**: 
  - github.com/sirupsen/logrus (日志系统)
  - golang.org/x/text (字符编码转换)
- **架构模式**: 事件驱动、发布订阅模式

### 高级架构
```
stocksub/
├── cmd/                    # 命令行入口
│   └── stocksub/          # 主程序
├── pkg/                   # 核心库
│   ├── subscriber/        # 订阅器核心
│   ├── provider/          # 数据提供商
│   │   └── tencent/      # 腾讯数据源实现
│   ├── config/           # 配置管理
│   └── logger/           # 日志系统
└── tests/                # 测试文件
```

## 2. 快速开始

### 环境要求
- Go 1.23+
- Git

### 安装步骤
```bash
# 克隆项目
git clone <repository-url>
cd stocksub

# 下载依赖
go mod download

# 构建主程序
go build -o stocksub ./cmd/stocksub
```

### 基本使用
```go
// 初始化配置和日志
cfg := config.Default()
logger.Init(cfg.Logger)

// 创建组件
provider := tencent.NewProvider()
sub := subscriber.NewSubscriber(provider)
manager := subscriber.NewManager(sub)

// 启动服务
ctx := context.Background()
manager.Start(ctx)

// 订阅股票
manager.Subscribe("600000", 5*time.Second, callbackFunc)
```

## 3. 项目结构详解

### 核心组件

#### pkg/subscriber/
- `types.go`: 定义核心数据结构和接口
- `subscriber.go`: 订阅器核心实现
- `manager.go`: 高级管理功能（统计、健康检查、自动重启）

#### pkg/provider/tencent/
- `client.go`: 腾讯数据提供商实现
- `parser.go`: 腾讯数据解析器（GBK转UTF-8）

#### pkg/config/
- `config.go`: 全局配置管理

#### pkg/logger/
- 日志系统封装，基于logrus

### 主要入口点
- `cmd/stocksub/main.go`: 主程序入口，演示完整使用方式

## 4. 开发工作流

### 编码规范
- 遵循Go官方编码规范
- 使用结构化日志记录
- 错误处理采用"错误即值"原则
- 使用context进行生命周期管理

### 测试策略
```bash
# 运行所有测试
go test ./...

# 运行特定测试
go test -v ./tests/csv_storage_test.go
```

### 构建和部署
```bash
# 本地构建
go build -o stocksub ./cmd/stocksub

# 交叉编译
GOOS=linux GOARCH=amd64 go build -o stocksub-linux ./cmd/stocksub
```

## 5. 核心概念

### 订阅器模式
- **Provider**: 数据提供商接口，负责从外部API获取数据
- **Subscriber**: 订阅器接口，管理股票订阅和数据分发
- **Manager**: 管理器，提供统计、健康检查、自动重启等高级功能

### 事件驱动架构
- 通过channel实现事件发布/订阅
- 支持数据更新、错误、订阅状态变更等事件

### 并发安全
- 使用sync.RWMutex保护共享资源
- 通过goroutine实现并发数据处理

## 6. 常见开发任务

### 添加新的数据提供商
1. 实现`subscriber.Provider`接口
2. 在`cmd/stocksub/main.go`中替换提供商
3. 更新配置和文档

### 添加新的订阅股票
```go
// 使用管理器订阅
manager.Subscribe("AAPL", 5*time.Second, func(data subscriber.StockData) error {
    // 处理数据
    return nil
})
```

### 批量操作
```go
// 批量订阅
requests := []subscriber.SubscribeRequest{
    {Symbol: "600000", Interval: 5*time.Second, Callback: callback},
    {Symbol: "000001", Interval: 5*time.Second, Callback: callback},
}
manager.SubscribeBatch(requests)
```

### 获取统计信息
```go
stats := manager.GetStatistics()
fmt.Printf("活跃订阅数: %d\n", stats.ActiveSubscriptions)
```

## 7. 故障排除

### 常见问题

1. **订阅失败: symbol XXX is not supported**
   - 检查股票代码格式是否正确
   - 确认提供商是否支持该市场

2. **HTTP request failed**
   - 检查网络连接
   - 增加重试次数或延长超时时间

3. **rate limit exceeded**
   - 增加订阅间隔
   - 减少并发订阅数

### 调试技巧
```bash
# 启用调试模式
export DEBUG=1
./stocksub

# 查看详细日志
go run ./cmd/stocksub --log-level=debug
```

## 8. 参考资源

- [Go官方文档](https://golang.org/doc/)
- [Logrus文档](https://github.com/sirupsen/logrus)
- 腾讯股票API文档（内部）
- 项目README.md文件

### 性能基准
- 内存占用: < 50MB (100个订阅)
- CPU占用: < 5% (正常负载)
- 支持订阅: 最多1000个股票同时订阅