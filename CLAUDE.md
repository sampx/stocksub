# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**StockSub** 是一个专精于A股市场的企业级股票数据订阅服务，基于Go语言实现，专注于提供高质量的大陆A股实时数据订阅能力。该项目是原有qq-market-simple项目的重构版本，提供了更完善的功能和更好的架构设计。

## Project Architecture

### Core Modules
- **cmd/stocksub/** - 主程序入口
- **pkg/subscriber/** - 订阅器核心（订阅管理、事件处理、统计监控）
- **pkg/provider/tencent/** - 腾讯数据源实现（HTTP客户端、数据解析）
- **pkg/config/** - 配置管理系统
- **pkg/logger/** - 结构化日志系统
- **examples/** - 使用示例和演示

### Architecture Features
- **接口驱动**: Provider接口支持多数据源扩展
- **并发安全**: 全面的并发控制和线程安全
- **事件驱动**: 基于事件的异步通信机制
- **统计监控**: 内置的健康检查和性能统计
- **优雅关闭**: Context-based的生命周期管理

## Development Commands

### Build & Run
```bash
# 主程序
go build -o stocksub ./cmd/stocksub
./stocksub

# 示例程序
go run ./examples/simple     # 基础示例
go run ./examples/advanced   # 高级功能演示

# 直接运行主程序
go run ./cmd/stocksub
```

### Testing & Quality
```bash
# 代码检查
go vet ./...
go fmt ./...

# 构建验证
go build ./...

# 模块清理
go mod tidy
```

### Cross-compilation
```bash
# Linux服务器部署
GOOS=linux GOARCH=amd64 go build -o stocksub-linux ./cmd/stocksub

# Windows桌面部署  
GOOS=windows GOARCH=amd64 go build -o stocksub.exe ./cmd/stocksub
```

## Usage Patterns

### 基础订阅模式
```go
// 1. 创建提供商和订阅器
provider := tencent.NewProvider()
subscriber := subscriber.NewSubscriber(provider)

// 2. 启动并订阅
ctx := context.Background()
subscriber.Start(ctx)
subscriber.Subscribe("600000", 5*time.Second, callback)
```

### 企业级管理模式（推荐）
```go
// 1. 配置初始化
cfg := config.Default().SetDefaultInterval(6*time.Second)
logger.Init(cfg.Logger)

// 2. 创建管理器
provider := tencent.NewProvider()
sub := subscriber.NewSubscriber(provider)
manager := subscriber.NewManager(sub)

// 3. 批量订阅和监控
manager.Start(ctx)
manager.SubscribeBatch(requests)
stats := manager.GetStatistics()
```

## Key Components

### 1. Subscriber（订阅器）
- **功能**: 股票订阅管理、数据分发、间隔控制
- **文件**: `pkg/subscriber/subscriber.go`
- **特点**: 支持动态订阅、智能分组、并发安全

### 2. Manager（管理器）
- **功能**: 高级管理、统计监控、自动重启、健康检查  
- **文件**: `pkg/subscriber/manager.go`
- **特点**: 企业级功能、故障恢复、性能统计

### 3. Provider（数据提供商）
- **功能**: 数据获取、请求限流、重试机制
- **文件**: `pkg/provider/tencent/client.go`
- **特点**: 接口抽象、可扩展设计

### 4. Config（配置系统）
- **功能**: 统一配置管理、参数验证
- **文件**: `pkg/config/config.go`  
- **特点**: 链式调用、默认值、验证

## Stock Code Support

| Market | Format | Examples |
|--------|--------|----------|
| **A股上证** | 6位数字 | `600000`, `601398` |
| **A股深证** | 6位数字 | `000001`, `300750` |
| **A股北交所** | 6位数字 | `835174`, `832000` |
| **A股科创板** | 6位数字 | `688036`, `688599` |

## Performance & Limits

- **并发订阅**: 默认100个，可配置最大1000个
- **请求频率**: 200ms最小间隔，防止API限制
- **内存占用**: <50MB (100订阅)
- **订阅间隔**: 1秒-1小时可配置范围
- **数据延迟**: 1-3秒（腾讯API延迟）
- **数据精度**: 专为A股优化，支持手到股的单位转换

## Error Handling & Recovery

- **智能重试**: 指数退避重试机制（1s→2s→3s）
- **自动重启**: 连续失败时自动重启订阅
- **健康检查**: 30秒间隔健康状态检查  
- **优雅降级**: 单个订阅失败不影响其他订阅

## Monitoring & Statistics

- **实时统计**: 订阅数、数据点、错误率
- **健康状态**: 每个订阅的健康状况
- **事件系统**: 完整的事件通知机制
- **日志系统**: 结构化日志，支持文件/控制台输出

## Important Notes

⚠️ **API频率限制**: 腾讯API有频率限制，生产环境建议订阅间隔≥5秒
⚠️ **网络稳定性**: 需要稳定的网络连接，建议部署在国内服务器
⚠️ **内存管理**: 大量订阅时注意内存使用，建议监控统计信息
⚠️ **优雅关闭**: 使用Context进行优雅关闭，避免数据丢失
⚠️ **A股专精**: 系统已优化为A股专用，不再支持港股和美股数据
⚠️ **交易时间**: 注意A股交易时间，非交易时间数据可能不更新