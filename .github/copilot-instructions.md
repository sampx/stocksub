# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
StockSub is an enterprise-grade A-share (Chinese stock market) real-time data subscription service built in Go. This is a refactored version of the original qq-market-simple project, specialized for mainland Chinese A-share markets with support for Shanghai (6xxxxx), Shenzhen (0xxxxx/3xxxxx), Beijing Stock Exchange (8xxxxx), and STAR Market (688xxx) stocks.

## Architecture & Components

### Core Layered Architecture
- **`cmd/stocksub/`** - Main application entry point with graceful shutdown
- **`cmd/api_monitor/`** - Long-term API monitoring and data collection with intelligent rate limiting
- **`cmd/api_analyzer/`** - Data analysis tools for collected CSV data
- **`cmd/data_exporter/`** - Data export utilities
- **`pkg/subscriber/`** - Core subscription engine (subscriber, manager, types)
- **`pkg/provider/tencent/`** - Tencent data source implementation (client, parser)
- **`pkg/config/`** - Configuration management with validation
- **`pkg/logger/`** - Structured logging with logrus
- **`pkg/limiter/`** - Intelligent rate limiting and error classification
- **`pkg/timing/`** - Market time detection and trading hours management
- **`pkg/testkit/`** - Comprehensive test utilities (caching, storage, mock providers)
- **`tests/`** - Integration tests for API functionality, data validation, and performance

### Key Patterns

#### 1. Interface-Driven Provider Pattern
```go
// All data providers implement this interface in pkg/subscriber/types.go
type Provider interface {
    Name() string
    FetchData(ctx context.Context, symbols []string) ([]StockData, error)
    IsSymbolSupported(symbol string) bool
    GetRateLimit() time.Duration
}
```

#### 2. Event-Driven Subscription Model
- Single 1-second ticker for all subscriptions in `runSubscriptions()`
- Individual subscription intervals checked on each tick
- Async `fetchAndNotify()` goroutines prevent blocking
- Four event types: `EventTypeData`, `EventTypeError`, `EventTypeSubscribed`, `EventTypeUnsubscribed`
- Event channel (`UpdateEvent`) for monitoring subscription lifecycle

#### 3. Two-Tier Management
- **Subscriber**: Core subscription logic with interval control
- **Manager**: Enterprise features (statistics, health checks, auto-restart)

#### 4. Intelligent Rate Limiting System
- **`pkg/limiter/intelligent_limiter.go`** - Centralized API call management with circuit breaking
- **`pkg/limiter/error_classifier.go`** - Error classification and retry strategy management
- **`pkg/timing/market_time.go`** - Trading hours detection and time-based restrictions

## Critical Development Workflows

### Build & Test Commands
```bash
# Main application
go run ./cmd/stocksub

# Examples (better for testing)
go run ./examples/simple      # Basic subscription demo
go run ./examples/advanced    # Manager with statistics

# API Monitoring (long-term data collection)
go run ./cmd/api_monitor -symbols=600000,000001 -duration=5m -interval=3s
go run ./cmd/api_monitor -symbols=600000 -duration=24h -data-dir=./collected_data

# Run all tests
go test -v ./tests/...

# Run specific test categories
go test -v ./tests/api_format_test.go          # API response format validation
go test -v ./tests/csv_integration_test.go     # 30-second API functionality test (integration)
go test -v ./tests/time_field_long_run_test.go # Long-running data collection tests
go test -v ./tests/performance_benchmark_test.go # Performance benchmarking
go test -v ./pkg/testkit/...                   # Testkit unit tests

# Production build with cross-compilation
GOOS=linux GOARCH=amd64 go build -o stocksub-linux ./cmd/stocksub
```

### Stock Symbol Format (A-Share Specific)
- Shanghai: `600000`, `601398` (6-digit numbers)
- Shenzhen: `000001`, `300750` (6-digit numbers) 
- Beijing Stock Exchange: `835174`, `832000`
- STAR Market: `688036`, `688599`
- **Critical**: No prefixes (sh/sz) - use raw 6-digit codes only

## Project-Specific Conventions

### 1. Concurrent Safety Patterns
```go
// Always use read locks for iteration, write locks for modification
s.subsMu.RLock()
for symbol, sub := range s.subscriptions {
    // read operations
}
s.subsMu.RUnlock()
```

### 2. Context-Based Lifecycle Management
- All long-running operations use `context.WithCancel`
- Graceful shutdown via signal handling in main
- 30-second timeout for data fetching operations

### 3. Rate Limiting & API Constraints
- **Minimum interval**: 200ms between requests (Tencent API limit)
- **Recommended production interval**: ≥5 seconds
- **Request batching**: Multiple symbols in single API call
- **Retry pattern**: 3 retries with exponential backoff
- **Intelligent limiting**: Automatic trading hours detection and error-based circuit breaking

### 4. Configuration Chain Pattern
```go
cfg := config.Default().
    SetDefaultInterval(6*time.Second).
    SetMaxSubscriptions(50)
```

### 5. CSV Testing & Monitoring System
```go
// API监控器 - 用于长期数据收集
monitor := NewAPIMonitor(config)
monitor.StartCollection(ctx, symbols, duration)

// CSV存储 - 使用新的testkit存储系统
storageCfg := storage.DefaultCSVStorageConfig()
storageCfg.Directory = "./collected_data"
csvStorage, err := storage.NewCSVStorage(storageCfg)

// 智能限制器集成
marketTime := timing.DefaultMarketTime()
intelligentLimiter := limiter.NewIntelligentLimiter(marketTime)
intelligentLimiter.InitializeBatch(symbols)
```

## Integration Points

### 1. Tencent API Integration
- **Base URL**: `http://qt.gtimg.cn/q=` + symbol list
- **Response format**: CSV-like pipe-separated values
- **Parser location**: `pkg/provider/tencent/parser.go`
- **Rate limiting**: Built into client with mutex-protected timing

### 2. Event System
Four event types in `UpdateEvent`:
- `EventTypeSubscribed`/`EventTypeUnsubscribed`: Subscription lifecycle
- `EventTypeData`: Successful data updates
- `EventTypeError`: Error notifications

### 3. Statistics & Monitoring Integration
```go
// Manager always maintains subscription statistics
stats := manager.GetStatistics()
// Includes: data points, error rates, health status per symbol

// Intelligent limiter status
limiterStatus := intelligentLimiter.GetStatus()
// Includes: retry counts, trading time status, error patterns
```

### 4. Testkit Storage System
- **`pkg/testkit/storage/csv.go`** - Advanced CSV storage with automatic file rotation
- **`pkg/testkit/cache/`** - Multi-layer caching system for test data
- **`pkg/testkit/providers/`** - Mock and cached provider implementations
- **`pkg/testkit/helpers/`** - Resource management and file utilities

## Performance Characteristics
- **Memory usage**: <50MB for 100 subscriptions
- **Concurrent limit**: Default 100 subscriptions (configurable to 1000)
- **Data delay**: 1-3 seconds (Tencent API latency)
- **Precision**: Optimized for A-share units (手 to 股 conversion)
- **Storage efficiency**: CSV compression and automatic file rotation

## Common Debugging Patterns
1. **Enable debug logging**: Set `STOCKSUB_LOG_LEVEL=debug` environment variable
2. **Use simple example**: `go run ./examples/simple` for quick testing
3. **Check statistics**: Manager provides real-time health metrics
4. **Monitor events**: Subscribe to event channel for subscription lifecycle
5. **Use API monitor**: `go run ./cmd/api_monitor` for detailed API analysis
6. **Check trading hours**: Use `timing.DefaultMarketTime().IsTradingTime()`

## File Naming Conventions
- **Main business logic**: `subscriber.go`, `manager.go` 
- **Data structures**: `types.go`
- **Provider implementations**: `pkg/provider/{name}/client.go`
- **Examples**: `examples/{complexity}/main.go`
- **Test utilities**: `pkg/testkit/{component}/` structure

## Important Notes

⚠️ **API High Frequency Request Warning**: The Tencent API imposes frequency limitations. When designing code for API requests, please ensure a minimum interval of 200ms between each request. If you plan to test the actual API endpoints, you must notify users in advance, preferably allowing them to execute the tests themselves. Users may need to configure proxies to bypass the server's anti-crawling measures; otherwise, high-frequency API access could lead to IP bans, which would prevent this project from being used or tested properly.

⚠️ **Trading Hours Consideration**: The intelligent limiter automatically stops API calls outside trading hours (09:13:30-11:30:10 and 12:57:30-15:00:10 on weekdays).

## Testing Architecture

### Test Organization
- **`tests/`** - Integration tests for API functionality, data validation, and performance
- **`pkg/testkit/`** - Comprehensive test utilities framework:
  - **`cache/`** - Layered caching with different eviction policies
  - **`storage/`** - CSV and memory storage implementations
  - **`providers/`** - Mock and cached provider implementations
  - **`helpers/`** - Resource management utilities
- **Build tags**: Use `//go:build integration` for integration tests

### Key Test Patterns
```go
// API format validation tests
go test -v ./tests/api_format_test.go

// Long-running data collection tests  
go test -v ./tests/time_field_long_run_test.go

// Performance benchmarking
go test -v ./tests/performance_benchmark_test.go

// CSV storage integration (30-second test)
go test -v ./tests/csv_integration_test.go

// Testkit unit tests
go test -v ./pkg/testkit/...

// Run with integration tag
go test -v -tags=integration ./tests/...
```

### Test Data Management
- Test data is stored in `tests/data/` with timestamp-based organization
- CSV files are used for persistent data collection and analysis
- Automatic file rotation based on size and time intervals
- Data organized by type and date: `stocksub_stock_data_2025-08-22.csv`
- Performance metrics stored separately: `stocksub_performance_2025-08-22.csv`

### Intelligent Testing Features
- **Market time awareness**: Tests respect trading hours
- **Error classification**: Automatic error handling and retry strategies
- **Data consistency**: Post-trading hours data stability detection
- **Resource pooling**: Efficient file handle and buffer management