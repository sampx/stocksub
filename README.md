# StockSub - 分布式股票数据中台

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Compose-blue.svg)](https://docker.com)
[![Redis](https://img.shields.io/badge/Redis-Streams-red.svg)](https://redis.io)
[![InfluxDB](https://img.shields.io/badge/InfluxDB-2.x-orange.svg)](https://influxdata.com)

StockSub 是一个现代化的分布式金融数据中台，基于微服务架构设计，支持实时股票数据采集、分发、存储和查询。通过 Redis Streams 消息队列实现高可用、可水平扩展的数据处理能力。

## ✨ 核心特性

### 🏗️ 分布式架构
- 🔧 **微服务设计**: 数据采集、存储、API 服务完全解耦
- 📡 **消息驱动**: 基于 Redis Streams 的发布-订阅模式
- 🔄 **水平扩展**: 支持多实例部署和负载均衡
- 🛡️ **故障隔离**: 单点故障不影响整体服务可用性

### 📊 数据能力
- 🌐 **多市场支持**: A股（上证、深证、北交所）
- 🚀 **多数据源**: 腾讯、新浪等多个数据提供商
- ⚡ **实时采集**: 毫秒级数据延迟，智能频率控制
- 🗄️ **多重存储**: Redis 缓存 + InfluxDB 时序数据库
- 📈 **历史数据**: 完整的时序数据存储和查询能力

### 🔧 工程化特性
- 🐳 **容器化部署**: Docker Compose 一键部署
- ⚙️ **配置驱动**: YAML 配置文件动态任务调度
- 📝 **结构化日志**: JSON 格式日志，便于分析和监控
- 🧪 **测试完备**: 单元测试、集成测试、性能基准测试
- 🔍 **可观测性**: 健康检查、指标监控、事件追踪

## 🚀 快速开始

### 环境要求

- Go 1.23.0+
- Docker & Docker Compose
- Make 工具（可选）

### 一键部署

``bash
# 克隆项目
git clone <repository>
cd stocksub

# 下载依赖
go mod download

# 启动所有服务（推荐）
mage docker:upAll

# 或者分步启动
mage docker:env          # 启动基础服务 (Redis + InfluxDB)
mage docker:provider     # 启动数据采集节点
mage docker:redisCollector   # 启动 Redis 收集器
mage docker:influxCollector  # 启动 InfluxDB 收集器
mage docker:apiServer    # 启动 API 服务器
```

### 验证部署

```bash
# 检查服务状态
docker-compose -f docker-compose.dev.yml ps

# 测试 API 服务
curl http://localhost:8080/health
curl http://localhost:8080/stocks/600000

# 查看实时数据流
docker logs -f stocksub-provider-node-dev
```

### API 客户端使用（推荐）

```bash
# 获取实时股票数据
curl "http://localhost:8080/stocks/600000"

# 获取多个股票数据
curl "http://localhost:8080/stocks/batch?symbols=600000,000001,AAPL"

# 获取历史数据
curl "http://localhost:8080/stocks/600000/history?start=2024-01-01&end=2024-01-31"

# 获取系统状态
curl "http://localhost:8080/health"
curl "http://localhost:8080/metrics"
```

### 单机开发模式

```
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

```
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

## 🏗️ 系统架构

### 分布式服务架构

```
graph TB
    subgraph "数据源"
        DS1[腾讯API]
        DS2[新浪API]
    end
    
    subgraph "数据采集层"
        PN1[Provider Node 1]
        PN2[Provider Node 2]
    end
    
    subgraph "消息队列"
        RS[Redis Streams]
    end
    
    subgraph "数据处理层"
        RC1[Redis Collector 1]
        RC2[Redis Collector 2]
        IC1[InfluxDB Collector 1]
        IC2[InfluxDB Collector 2]
    end
    
    subgraph "存储层"
        Redis[(Redis Cache)]
        InfluxDB[(InfluxDB)]
    end
    
    subgraph "API服务层"
        API1[API Server 1]
        API2[API Server 2]
    end
    
    DS1 --> PN1
    DS2 --> PN2
    PN1 --> RS
    PN2 --> RS
    RS --> RC1
    RS --> RC2
    RS --> IC1
    RS --> IC2
    RC1 --> Redis
    RC2 --> Redis
    IC1 --> InfluxDB
    IC2 --> InfluxDB
    Redis --> API1
    Redis --> API2
    InfluxDB --> API1
    InfluxDB --> API2
```

### 项目结构

```
stocksub/
├── cmd/                          # 微服务应用
│   ├── provider_node/           # 数据采集节点
│   ├── api_server/              # API 服务器
│   ├── influxdb_collector/      # InfluxDB 收集器
│   ├── redis_collector/         # Redis 收集器
│   ├── api_monitor/             # API 监控器
│   ├── logging_collector/       # 日志收集器
│   ├── config_migrator/         # 配置迁移工具
│   └── stocksub/               # 兼容性主程序
├── pkg/                         # 核心库
│   ├── provider/               # 数据提供商
│   │   ├── core/              # 核心接口
│   │   ├── tencent/           # 腾讯数据源
│   │   ├── sina/              # 新浪数据源
│   │   └── decorators/        # 装饰器（限流、熔断等）
│   ├── subscriber/            # 订阅器（兼容层）
│   ├── scheduler/             # 任务调度器
│   ├── message/               # 消息格式定义
│   ├── testkit/               # 测试工具包
│   ├── limiter/               # 智能限流器
│   ├── config/                # 配置管理
│   └── logger/                # 日志系统
├── config/                      # 配置文件
│   ├── jobs.yaml              # 任务调度配置
│   ├── api_server.yaml        # API 服务配置
│   ├── influxdb_collector.yaml # InfluxDB 收集器配置
│   └── redis_collector.yaml   # Redis 收集器配置
├── docker-compose.dev.yml       # 开发环境
├── docker-compose.prod.yml      # 生产环境
└── magefile.go                 # 构建任务
```

### 核心组件

#### 数据采集层
- **Provider Node**: 可独立部署的数据采集服务，支持多数据源
- **Job Scheduler**: 基于 Cron 表达式的任务调度器
- **Intelligent Limiter**: 智能频率控制和错误重试

#### 消息队列层
- **Redis Streams**: 高性能消息队列，支持消费者组模式
- **Message Format**: 标准化的 JSON 消息格式，包含校验和元数据

#### 数据处理层
- **Redis Collector**: 实时数据缓存，支持快速查询
- **InfluxDB Collector**: 时序数据持久化，支持历史查询
- **Consumer Groups**: 水平扩展和负载均衡

#### API 服务层
- **REST API**: 标准 HTTP API，支持实时和历史数据查询
- **Health Check**: 服务健康状态监控
- **Metrics**: 性能指标和统计信息

#### 存储层
- **Redis**: 实时数据缓存，毫秒级查询响应
- **InfluxDB**: 时序数据库，高效存储和查询历史数据

## 📈 支持的数据源与市场

### 数据提供商

| 提供商 | 类型 | 市场覆盖 | 特点 |
|--------|------|----------|------|
| **腾讯财经** | 实时行情 | A股 | 数据稳定，延迟低 |
| **新浪财经** | 实时行情 | A股 | 备用数据源 |
| **自定义** | 可扩展 | 任意市场 | 支持插件化扩展 |

### 支持的股票市场

| 市场 | 格式示例 | 说明 |
|------|----------|------|
| **A股上证** | `600000`, `601398` | 6开头的6位数字 |
| **A股深证** | `000001`, `300750` | 0/3开头的6位数字 |  
| **A股北交** | `835174`, `832000` | 4/8开头的6位数字 |

## ⚙️ 配置与管理

### 任务调度配置 (jobs.yaml)

```yaml
jobs:
  - name: "fetch-realtime-stock-ashare"
    enabled: true
    schedule: "*/3 * 9-11,13-14 * * 1-5"  # 每3秒，交易时段
    provider:
      name: "tencent"
      type: "RealtimeStock"
    params:
      symbols: ["600000", "000001"]
      market: "A-share"
    output:
      stream: "stream:stock:realtime"

  - name: "fetch-daily-history"
    enabled: true
    schedule: "0 16 * * 1-5"  # 每天16:00
    provider:
      name: "tencent"
      type: "Historical"
    params:
      symbols: ["all"]
      period: "1d"
```

### API 服务配置 (api_server.yaml)

```yaml
server:
  port: 8080
  timeout: 30s
  
redis:
  url: "redis://localhost:6379"
  password: ""
  db: 0
  
influxdb:
  url: "http://localhost:8086"
  token: "your-influxdb-token"
  org: "stocksub"
  bucket: "stockdata"
  
cache:
  ttl: 300s
  max_size: 10000
```

### 数据收集器配置

```yaml
# redis_collector.yaml
redis:
  url: "redis://localhost:6379"
  stream: "stream:stock:realtime"
  group: "redis-collectors"
  consumer: "redis-collector-1"
  
batch_size: 100
flush_interval: 5s

# influxdb_collector.yaml
influxdb:
  url: "http://localhost:8086"
  token: "your-token"
  org: "stocksub"
  bucket: "stockdata"
  
redis:
  url: "redis://localhost:6379"
  stream: "stream:stock:realtime"
  group: "influxdb-collectors"
  consumer: "influxdb-collector-1"
```

## 🔧 开发与运维

### Mage 任务管理

```bash
# 查看所有可用任务
mage

# 构建所有服务
mage build

# 运行测试
mage test                    # 所有测试
mage testUnit               # 单元测试
mage testIntegration        # 集成测试
mage benchmark              # 性能基准测试

# Docker 服务管理
mage docker:build           # 构建镜像
mage docker:env             # 启动基础环境
mage docker:upAll           # 启动所有服务
mage docker:provider        # 启动数据采集节点
mage docker:redisCollector  # 启动 Redis 收集器
mage docker:influxCollector # 启动 InfluxDB 收集器
mage docker:apiServer       # 启动 API 服务器
mage docker:down            # 停止所有服务

# 工具
mage clean                  # 清理构建产物
mage lint                   # 代码检查
mage coverage               # 测试覆盖率
mage deploy                 # 部署到生产环境
```

### 手动构建与运行

```bash
# 构建所有服务
go build -o dist/provider_node ./cmd/provider_node
go build -o dist/api_server ./cmd/api_server
go build -o dist/influxdb_collector ./cmd/influxdb_collector
go build -o dist/redis_collector ./cmd/redis_collector

# 单独运行服务
./dist/provider_node --config config/jobs.yaml
./dist/api_server --config config/api_server.yaml
./dist/influxdb_collector --config config/influxdb_collector.yaml
./dist/redis_collector --config config/redis_collector.yaml

# 兼容模式运行
go run ./cmd/stocksub
go run ./examples/subscriber/simple
```

### 生产部署

```bash
# 生产环境部署
docker-compose -f docker-compose.prod.yml up -d

# 查看服务状态
docker-compose -f docker-compose.prod.yml ps

# 查看日志
docker-compose -f docker-compose.prod.yml logs -f

# 滚动更新
docker-compose -f docker-compose.prod.yml pull
docker-compose -f docker-compose.prod.yml up -d
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

## 📊 REST API 参考

### 实时数据 API

```bash
# 单个股票实时数据
GET /stocks/{symbol}

# 批量获取实时数据
GET /stocks/batch?symbols=600000,000001

# 按市场获取数据
GET /stocks/market/{market}  # market: ashare
```

### 历史数据 API

```bash
# 获取历史K线数据
GET /stocks/{symbol}/history?start=2024-01-01&end=2024-01-31&period=1d

# 获取实时数据流
GET /stocks/{symbol}/stream
```

### 系统监控 API

```bash
# 健康检查
GET /health

# 性能指标
GET /metrics

# 系统统计
GET /stats
```

### API 响应格式

```json
{
  "header": {
    "messageId": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": 1678886400,
    "version": "1.0",
    "producer": "api-server-1"
  },
  "data": {
    "symbol": "600000",
    "name": "浦发银行",
    "price": 10.50,
    "change": 0.15,
    "changePercent": 1.45,
    "volume": 1250000,
    "turnover": 13125000.0,
    "timestamp": "2024-01-15T09:30:00Z",
    "market": "ashare"
  }
}
```

## 🛠️ 订阅器库接口（兼容模式）

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

## 🚀 性能指标与技术规格

### 系统性能
- **单节点内存占用**: < 100MB （每个微服务）
- **CPU 占用**: < 10% （正常负载）
- **数据延迟**: 1-3秒 （数据源延迟）
- **API 响应时间**: < 100ms （实时数据），< 500ms （历史数据）
- **并发处理**: 10,000+ QPS （API 服务器）

### 扩展性
- **数据采集节点**: 无上限水平扩展
- **数据收集器**: 支持多实例负载均衡
- **API 服务器**: 无状态，可水平扩展
- **支持股票数**: 无理论上限（取决于存储容量）

### 技术规格
- **Go 版本**: 1.23.0+
- **Redis**: 7.0+ （支持 Streams）
- **InfluxDB**: 2.7+
- **Docker**: 20.10+
- **Docker Compose**: 2.0+

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件