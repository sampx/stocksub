# TestKit Cache 模块使用示例

本目录包含了 `stocksub/pkg/testkit/cache` 模块的完整使用示例，演示了该模块的所有核心功能和高级特性。

## 📁 文件结构

```
examples/testkit/cache/
├── README.md                  # 本文档
├── main.go                   # 统一入口程序，包含基础演示函数
├── policy_demos.go           # 缓存策略演示函数 (LRU/LFU/FIFO)
├── layered_demos.go          # 分层缓存演示函数  
├── advanced_demos.go         # 高级功能演示函数
└── Makefile                  # 构建和运行工具
```

## 🚀 快速开始

## 🚀 快速开始

### 方式一：使用 Makefile（推荐）

```bash
# 进入 cache 示例目录
cd examples/testkit/cache

# 查看所有可用命令
make help

# 交互式运行（可以选择要运行的示例）
make run

# 直接运行特定示例
make basic      # 基础缓存操作
make policy     # 缓存策略演示
make layered    # 分层缓存演示
make advanced   # 高级功能演示
make all        # 运行所有示例

# 运行测试
make test       # 单元测试
make benchmark  # 性能基准测试
```

### 方式二：直接使用 go run

```bash
# 进入 cache 示例目录
cd examples/testkit/cache

# 交互式运行主程序
go run .

# 或者从项目根目录运行
cd /path/to/stocksub
go run ./examples/testkit/cache

# 非交互式运行特定示例
echo "1" | go run ./examples/testkit/cache  # 基础演示
echo "2" | go run ./examples/testkit/cache  # 策略演示
echo "3" | go run ./examples/testkit/cache  # 分层演示
echo "4" | go run ./examples/testkit/cache  # 高级演示
echo "5" | go run ./examples/testkit/cache  # 所有演示
```

## 📖 示例详解

### 1. 基础缓存操作 (`basic_cache_demo.go`)

演示 `MemoryCache` 的核心功能：

- **基础 CRUD 操作**：Set、Get、Delete、Clear
- **TTL (生存时间) 管理**：自动过期和清理机制
- **统计信息查询**：命中率、缓存大小等指标
- **资源管理**：缓存容量限制和淘汰机制

**核心特性：**
```go
// 创建内存缓存
config := cache.MemoryCacheConfig{
    MaxSize:         100,                // 最大缓存条目数
    DefaultTTL:      5 * time.Minute,    // 默认生存时间
    CleanupInterval: 1 * time.Minute,    // 清理间隔
}
memCache := cache.NewMemoryCache(config)

// 基础操作
memCache.Set(ctx, "key", "value", ttl)  // 存储
value, err := memCache.Get(ctx, "key")   // 获取
memCache.Delete(ctx, "key")              // 删除
memCache.Clear(ctx)                      // 清空
```

### 2. 缓存策略演示 (`policy_demos.go`)

展示不同的缓存淘汰策略：

- **LRU (Least Recently Used)**：淘汰最近最少使用的数据
- **LFU (Least Frequently Used)**：淘汰访问频率最低的数据
- **FIFO (First In First Out)**：淘汰最先进入的数据
- **策略对比**：同样操作下不同策略的行为差异

**智能缓存使用：**
```go
// 创建带策略的智能缓存
policyConfig := cache.PolicyConfig{
    Type:    cache.PolicyLRU,  // 选择策略
    MaxSize: 100,
    TTL:     5 * time.Minute,
}
smartCache := cache.NewSmartCache(memConfig, policyConfig)
```

### 3. 分层缓存演示 (`layered_demos.go`)

演示多层缓存架构的高级功能：

- **多层结构**：L1(快速小容量) + L2(大容量慢速) + L3(持久化)
- **数据提升**：热点数据自动提升到更快的层级
- **写模式**：写穿透、写回等不同的写入策略
- **自定义配置**：灵活的分层架构配置
- **统计监控**：详细的分层统计信息

**分层缓存配置：**
```go
config := cache.LayeredCacheConfig{
    Layers: []cache.LayerConfig{
        {
            Type:    cache.LayerMemory,
            MaxSize: 100,               // L1: 小而快
            TTL:     1 * time.Minute,
            Policy:  cache.PolicyLRU,
        },
        {
            Type:    cache.LayerMemory,
            MaxSize: 1000,              // L2: 大而慢
            TTL:     10 * time.Minute,
            Policy:  cache.PolicyLFU,
        },
    },
    PromoteEnabled: true,    // 启用数据提升
    WriteThrough:   false,   // 写模式配置
}
```

### 4. 高级功能演示 (`advanced_demos.go`)

展示企业级应用场景：

- **并发安全**：多 goroutine 并发读写测试
- **错误处理**：完整的错误类型和处理机制
- **性能基准**：不同配置下的性能测试
- **内存管理**：自动清理和手动管理
- **实际应用**：股票数据缓存的完整场景

**实际应用示例：**
```go
// 股票数据缓存场景
type StockInfo struct {
    Symbol string  `json:"symbol"`
    Name   string  `json:"name"`
    Price  float64 `json:"price"`
    Volume int64   `json:"volume"`
    Time   string  `json:"time"`
}

// 缓存股票数据
stockCache.Set(ctx, "stock_info:600000", stockInfo, 30*time.Second)

// 查询股票数据
value, err := stockCache.Get(ctx, "stock_info:600000")
if err == nil {
    stock := value.(StockInfo)
    // 使用股票数据...
}
```

## 🏗️ 架构设计

### Cache 模块架构

```
Cache Interface (pkg/testkit/core)
├── MemoryCache          # 基础内存缓存实现
├── SmartCache           # 集成策略的智能缓存
└── LayeredCache         # 多层缓存架构

Policy System
├── LRU Policy           # 最近最少使用
├── LFU Policy           # 最少频繁使用
└── FIFO Policy          # 先进先出

Error Handling (pkg/testkit/core)
├── TestKitError         # 统一错误类型
├── ErrorCode           # 错误代码常量
└── RetryConfig         # 重试配置
```

### 核心接口

```go
// 缓存接口
type Cache interface {
    Get(ctx context.Context, key string) (interface{}, error)
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
    Stats() CacheStats
}

// 淘汰策略接口
type EvictionPolicy interface {
    ShouldEvict(entries map[string]*core.CacheEntry) []string
    OnAccess(key string, entry *core.CacheEntry)
    OnAdd(key string, entry *core.CacheEntry)
    OnRemove(key string, entry *core.CacheEntry)
}
```

## 📊 性能特征

### 基准性能
- **内存缓存**：100,000+ ops/sec (读取)，50,000+ ops/sec (写入)
- **智能缓存**：性能与策略复杂度相关，LRU > LFU > FIFO
- **分层缓存**：L1 命中 < 1ms，L2 命中 < 5ms

### 内存使用
- **基础开销**：每个缓存条目约 200 bytes
- **数据估算**：自动估算值大小，支持字符串和字节数组优化
- **清理机制**：定期清理过期条目，可配置清理间隔

### 并发性能
- **读写分离锁**：使用 `sync.RWMutex` 优化并发读取
- **原子计数**：命中/未命中统计使用原子操作
- **无锁操作**：统计查询无锁设计

## 🎯 使用场景

### 适用场景

1. **API 响应缓存**：缓存外部 API 调用结果
2. **数据库查询缓存**：减少数据库访问压力
3. **计算结果缓存**：缓存复杂计算的中间结果
4. **会话数据缓存**：用户会话和状态管理
5. **配置数据缓存**：系统配置的内存缓存

### 策略选择指南

- **LRU**：适合有明显访问模式的场景，如用户数据缓存
- **LFU**：适合长期运行且访问频率差异大的场景
- **FIFO**：适合数据时效性重要的场景，如实时数据缓存

### 分层架构使用

- **单层**：简单应用，内存容量充足
- **双层**：常见架构，L1(热点) + L2(全量)
- **三层**：企业级应用，L1(内存) + L2(本地缓存) + L3(分布式缓存)

## ⚠️ 注意事项

### 内存管理
- 合理设置 `MaxSize` 避免 OOM
- 监控缓存命中率调优配置
- 定期检查内存使用情况

### 并发安全
- 所有操作都是线程安全的
- 避免在回调中执行长时间操作
- 注意 goroutine 泄漏问题
- **已修复**：SmartCache 中的死锁问题（v1.1.0+）

### 错误处理
- 始终检查返回的错误
- 区分缓存未命中和系统错误
- 合理设置重试策略

### 性能优化
- 根据访问模式选择合适的策略
- 调节清理间隔平衡性能和内存
- 使用分层缓存提升整体性能

## 🔧 Makefile 使用指南

本目录提供了一个完善的 Makefile，方便运行和测试缓存模块。

### 基本命令

```bash
# 必须在 examples/testkit/cache 目录下运行
cd examples/testkit/cache

# 查看所有可用命令
make help
```

### 运行示例

```bash
make run        # 交互式运行（推荐）
make basic      # 基础缓存操作演示
make policy     # 缓存策略演示
make layered    # 分层缓存演示
make advanced   # 高级功能演示
make all        # 运行所有示例
```

### 测试和开发

```bash
make test       # 运行单元测试
make benchmark  # 性能基准测试
make fmt        # 代码格式化
make lint       # 静态代码检查
```

### 构建和清理

```bash
make build      # 构建可执行文件到 bin/
make clean      # 清理临时文件和构建产物
```

### 跨目录运行

如果不想切换目录，可以使用 `-C` 参数：

```bash
# 从项目根目录运行
make -C examples/testkit/cache policy

# 从任意目录运行
make -C /path/to/stocksub/examples/testkit/cache test
```

## 🔧 配置参考

### 开发环境配置
```go
config := cache.MemoryCacheConfig{
    MaxSize:         100,
    DefaultTTL:      1 * time.Minute,
    CleanupInterval: 10 * time.Second,
}
```

### 生产环境配置
```go
config := cache.MemoryCacheConfig{
    MaxSize:         10000,
    DefaultTTL:      30 * time.Minute,
    CleanupInterval: 5 * time.Minute,
}
```

### 高性能配置
```go
config := cache.LayeredCacheConfig{
    Layers: []cache.LayerConfig{
        {Type: cache.LayerMemory, MaxSize: 1000, TTL: 5*time.Minute, Policy: cache.PolicyLRU},
        {Type: cache.LayerMemory, MaxSize: 10000, TTL: 30*time.Minute, Policy: cache.PolicyLFU},
    },
    PromoteEnabled: true,
    WriteThrough:   false,
}
```

## 📚 扩展阅读

- [TestKit 核心概念](../../../pkg/testkit/README.md)
- [缓存策略详解](../../../doc/cache_policies.md)  
- [性能调优指南](../../../doc/performance_tuning.md)
- [错误处理最佳实践](../../../doc/error_handling.md)

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request 来改进这些示例：

1. 添加新的使用场景示例
2. 优化现有示例的性能
3. 补充更详细的注释说明
4. 修复发现的问题

---

**注意**：运行示例前请确保已正确设置 Go 模块路径和依赖关系。