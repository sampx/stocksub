# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview
StockSub is an enterprise-grade A-share real-time data subscription service built in Go, specialized for mainland Chinese markets with support for Shanghai (6xxxxx), Shenzhen (0xxxxx/3xxxxx), Beijing Stock Exchange (8xxxxx), and STAR Market (688xxx) stocks.

## Core Architecture

### Directory Structure
- **`cmd/stocksub/`** - Main application with graceful shutdown
- **`cmd/api_monitor/`** - Long-term API monitoring with intelligent rate limiting
- **`cmd/api_analyzer/`** - Data analysis tools for CSV data
- **`cmd/data_exporter/`** - Data export utilities
- **`pkg/subscriber/`** - Core subscription engine (subscriber, manager, types)
- **`pkg/provider/tencent/`** - Tencent data source implementation
- **`pkg/config/`** - Configuration management with validation
- **`pkg/logger/`** - Structured logging with logrus
- **`pkg/limiter/`** - Intelligent rate limiting and error classification
- **`pkg/timing/`** - Market time detection and trading hours
- **`pkg/testkit/`** - Comprehensive test utilities
- **`tests/`** - Integration tests

### Key Design Patterns

#### Interface-Driven Provider
```go
type Provider interface {
    Name() string
    FetchData(ctx context.Context, symbols []string) ([]StockData, error)
    IsSymbolSupported(symbol string) bool
    GetRateLimit() time.Duration
}
```

#### Event-Driven Subscription Model
- Single 1-second ticker for all subscriptions
- Async `fetchAndNotify()` goroutines
- Four event types: Data, Error, Subscribed, Unsubscribed
- Event channel for monitoring lifecycle

#### Two-Tier Management
- **Subscriber**: Core subscription logic with interval control
- **Manager**: Enterprise features (statistics, health checks, auto-restart)

#### Intelligent Rate Limiting
- Centralized API call management with circuit breaking
- Error classification and retry strategy management
- Trading hours detection (09:13:30-11:30:10, 12:57:30-15:00:10 weekdays)

## Development Workflows

### Build & Run Commands
```bash
# Main application
go run ./cmd/stocksub

# Examples for testing
go run ./examples/simple      # Basic subscription demo
go run ./examples/advanced    # Manager with statistics

# API Monitoring (long-term data collection)
go run ./cmd/api_monitor -symbols=600000,000001 -duration=5m -interval=3s
go run ./cmd/api_monitor -symbols=600000 -duration=24h -data-dir=./collected_data

# Production build
GOOS=linux GOARCH=amd64 go build -o stocksub-linux ./cmd/stocksub
```

### Testing Commands
```bash
# Run all tests
go test -v ./tests/...

# Specific test categories
go test -v ./tests/api_format_test.go          # API format validation
go test -v ./tests/csv_integration_test.go     # 30-second integration test
go test -v ./tests/time_field_long_run_test.go # Long-running data collection
go test -v ./tests/performance_benchmark_test.go # Performance benchmarking
go test -v ./pkg/testkit/...                   # Testkit unit tests

# Integration tests with build tag
go test -v -tags=integration ./tests/...
```

## Configuration & Conventions

### Stock Symbol Format
- **Shanghai**: `600000`, `601398` (6-digit numbers)
- **Shenzhen**: `000001`, `300750` (6-digit numbers)
- **Beijing Stock Exchange**: `835174`, `832000`
- **STAR Market**: `688036`, `688599`
- **Critical**: No prefixes (sh/sz) - use raw 6-digit codes only

### Concurrency Safety
```go
// Use read locks for iteration, write locks for modification
s.subsMu.RLock()
for symbol, sub := range s.subscriptions {
    // read operations
}
s.subsMu.RUnlock()
```

### Context-Based Lifecycle
- All long-running operations use `context.WithCancel`
- Graceful shutdown via signal handling
- 30-second timeout for data fetching

### Rate Limiting Constraints
- **Minimum interval**: 200ms between requests (Tencent API limit)
- **Recommended production**: ≥5 seconds
- **Request batching**: Multiple symbols per API call
- **Retry pattern**: 3 retries with exponential backoff

### Configuration Pattern
```go
cfg := config.Default().
    SetDefaultInterval(6*time.Second).
    SetMaxSubscriptions(50)
```

## Integration Points

### Tencent API
- **Base URL**: `http://qt.gtimg.cn/q=` + symbol list
- **Response format**: CSV-like pipe-separated values
- **Parser**: `pkg/provider/tencent/parser.go`
- **Rate limiting**: Built-in with mutex protection

### Event System
- `EventTypeSubscribed`/`EventTypeUnsubscribed`: Subscription lifecycle
- `EventTypeData`: Successful data updates
- `EventTypeError`: Error notifications

### Statistics & Monitoring
```go
// Subscription statistics
stats := manager.GetStatistics()  // data points, error rates, health status

// Limiter status  
limiterStatus := intelligentLimiter.GetStatus() // retry counts, trading time, error patterns
```

## Performance & Characteristics
- **Memory**: <50MB for 100 subscriptions
- **Concurrency**: Default 100 subscriptions (configurable to 1000)
- **Data delay**: 1-3 seconds (Tencent API latency)
- **Precision**: Optimized for A-share units (手 to 股 conversion)
- **Storage**: CSV compression with automatic file rotation

## Testing Architecture

### Test Organization
- **`tests/`** - Integration tests for API functionality and validation
- **`pkg/testkit/`** - Comprehensive test utilities:
  - **`cache/`** - Layered caching with eviction policies
  - **`storage/`** - CSV and memory storage implementations
  - **`providers/`** - Mock and cached provider implementations
  - **`helpers/`** - Resource management utilities

### Test Data Management
- Data stored in `tests/data/` with timestamp organization
- CSV files for persistent data collection
- Automatic file rotation based on size and time
- Data organized by type and date: `stocksub_stock_data_2025-08-22.csv`
- Performance metrics: `stocksub_performance_2025-08-22.csv`

### Intelligent Testing Features
- Market time awareness (respects trading hours)
- Error classification and automatic retry strategies
- Data consistency detection (post-trading hours)
- Resource pooling for efficient file handling

## Debugging & Troubleshooting
1. **Debug logging**: Set `STOCKSUB_LOG_LEVEL=debug`
2. **Quick testing**: Use `go run ./examples/simple`
3. **Statistics**: Check manager's real-time health metrics
4. **Event monitoring**: Subscribe to event channel
5. **API analysis**: Use `go run ./cmd/api_monitor`
6. **Trading hours**: Check `timing.DefaultMarketTime().IsTradingTime()`

## File Naming Conventions
- **Business logic**: `subscriber.go`, `manager.go`
- **Data structures**: `types.go`
- **Providers**: `pkg/provider/{name}/client.go`
- **Examples**: `examples/{complexity}/main.go`
- **Test utilities**: `pkg/testkit/{component}/` structure

## Important Warnings

⚠️ **API Frequency Warning**: Tencent API imposes strict frequency limits. Maintain minimum 200ms interval between requests. Notify users before testing actual endpoints. High-frequency access may cause IP bans.

⚠️ **Trading Hours**: Intelligent limiter automatically stops API calls outside trading hours (09:13:30-11:30:10 and 12:57:30-15:00:10 on weekdays).

## Go Testing Standards
1. Organize tests by functional modules, one test file per source file
2. Follow Go naming conventions: `source.go` → `source_test.go`
3. Focus each test file on specific functionality
4. Cover core features, edge cases, error handling, and concurrency safety
5. Include performance benchmarks (Benchmark)
6. Use `testify/assert` for readable assertions
7. Maintain high test coverage (target >90%)
8. Keep test code quality matching implementation code
9. Unit test cases should clearly state the testing purpose in Chinese