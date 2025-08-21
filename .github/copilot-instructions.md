# Copilot Instructions for StockSub

## Project Overview
StockSub is an enterprise-grade A-share (Chinese stock market) real-time data subscription service built in Go. This is a refactored version of the original qq-market-simple project, specialized for mainland Chinese A-share markets with support for Shanghai (6xxxxx), Shenzhen (0xxxxx/3xxxxx), Beijing Stock Exchange (8xxxxx), and STAR Market (688xxx) stocks.

## Architecture & Components

### Core Layered Architecture
- **`cmd/stocksub/`** - Main application entry point with graceful shutdown
- **`pkg/subscriber/`** - Core subscription engine (subscriber, manager, types)
- **`pkg/provider/tencent/`** - Tencent data source implementation (client, parser)
- **`pkg/config/`** - Configuration management with validation
- **`pkg/logger/`** - Structured logging with logrus

### Key Patterns

#### 1. Interface-Driven Provider Pattern
```go
// All data providers implement this interface in pkg/subscriber/types.go
type Provider interface {
    Name() string
    FetchData(ctx context.Context, symbols []string) ([]StockData, error)
    IsSymbolSupported(symbol string) bool
}
```

#### 2. Event-Driven Subscription Model
- Single 1-second ticker for all subscriptions in `runSubscriptions()`
- Individual subscription intervals checked on each tick
- Async `fetchAndNotify()` goroutines prevent blocking
- Event channel (`UpdateEvent`) for monitoring subscription lifecycle

#### 3. Two-Tier Management
- **Subscriber**: Core subscription logic with interval control
- **Manager**: Enterprise features (statistics, health checks, auto-restart)

## Critical Development Workflows

### Build & Run Commands
```bash
# Main application
go run ./cmd/stocksub

# Examples (better for testing)
go run ./examples/simple      # Basic subscription demo
go run ./examples/advanced    # Manager with statistics

# Production build with cross-compilation
GOOS=linux GOARCH=amd64 go build -o stocksub-linux ./cmd/stocksub
```

### Stock Symbol Format (A-Share Specific)
- Shanghai: `600000`, `601398` (6-digit numbers)
- Shenzhen: `000001`, `300750` (6-digit numbers) 
- Beijing Stock Exchange: `835174`, `832000`
- STAR Market: `688036`, `688599`

**Critical**: No prefixes (sh/sz) - use raw 6-digit codes only.

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

### 4. Statistics & Monitoring Integration
```go
// Manager always maintains subscription statistics
stats := manager.GetStatistics()
// Includes: data points, error rates, health status per symbol
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

### 3. Configuration Chain Pattern
```go
cfg := config.Default().
    SetDefaultInterval(6*time.Second).
    SetMaxSubscriptions(50)
```

## Performance Characteristics
- **Memory usage**: <50MB for 100 subscriptions
- **Concurrent limit**: Default 100 subscriptions (configurable to 1000)
- **Data delay**: 1-3 seconds (Tencent API latency)
- **Precision**: Optimized for A-share units (手 to 股 conversion)

## Common Debugging Patterns
1. **Enable debug logging**: Set `STOCKSUB_LOG_LEVEL=debug` environment variable
2. **Use simple example**: `go run ./examples/simple` for quick testing
3. **Check statistics**: Manager provides real-time health metrics
4. **Monitor events**: Subscribe to event channel for subscription lifecycle

## File Naming Conventions
- **Main business logic**: `subscriber.go`, `manager.go` 
- **Data structures**: `types.go`
- **Provider implementations**: `pkg/provider/{name}/client.go`
- **Examples**: `examples/{complexity}/main.go`


## Important Notes

⚠️ **API High Frequency Request Warning**: The Tencent API imposes frequency limitations. When designing code for API requests, please ensure a minimum interval of 200ms between each request. If you plan to test the actual API endpoints, you must notify users in advance, preferably allowing them to execute the tests themselves. Users may need to configure proxies to bypass the server's anti-crawling measures; otherwise, high-frequency API access could lead to IP bans, which would prevent this project from being used or tested properly.

