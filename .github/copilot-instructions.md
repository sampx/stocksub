# Project Context for AI Agents

## Project Overview
StockSub is an enterprise-grade A-share real-time data subscription service built in Go, specialized for mainland Chinese markets with support for Shanghai (6xxxxx), Shenzhen (0xxxxx/3xxxxx), Beijing Stock Exchange (8xxxxx), and STAR Market (688xxx) stocks.

## Critical Warnings & Constraints

⚠️ **API Frequency Warning [EXTREMELY IMPORTANT]**: Tencent API imposes strict frequency limits. Never perform API access tests without proper authorization. Notify users before testing actual endpoints. High-frequency access may cause IP bans.

⚠️ **Testing Restrictions**: Due to external API call limitations, you cannot run integration tests and benchmark tests yourself. If absolutely necessary, ask users to run them.

⚠️ **Language Policy**: Always communicate in Chinese, including code comments, explanations, and markdown documentation.

## Core Architecture

### Service-Oriented Architecture with Redis Streams
The system is built around a producer/consumer pattern using Redis Streams as the message bus:

**Key Services:**
- **`cmd/fetcher/`** - Primary data producer: loads jobs from `config/jobs.yaml`, applies decorator chains, publishes standardized messages to Redis Streams
- **`cmd/api_server/`** - REST API consumer that serves Redis Stream data to clients
- **`cmd/influxdb_collector/`** - Time-series database collector for historical analytics
- **`cmd/redis_collector/`** - Additional Redis-based data persistence layer
- **`cmd/logging_collector/`** - Centralized logging data collection service
- **`cmd/api_monitor/`** - Long-term monitoring with intelligent rate limiting and CSV export

### Core Package Architecture
- **`pkg/provider/`** - Data source implementations (Tencent, Sina) with decorator patterns
- **`pkg/message/`** - Standardized message formats for Redis Streams communication
- **`pkg/scheduler/`** - Cron-based job scheduling with graceful shutdown support
- **`pkg/timing/`** - Market trading hours detection and time service abstraction
- **`pkg/limiter/`** - Intelligent rate limiting with error classification and circuit breaking
- **`pkg/storage/`** - Multi-format data persistence (CSV, memory, structured data)
- **`pkg/cache/`** - Layered caching system with eviction policies
- **`pkg/testkit/`** - Comprehensive testing utilities with mock providers, unified data manager, and configurable cache/storage layers
- **`pkg/core/`** - Core interfaces and data structures
- **`pkg/error/`** - Error handling interfaces and utilities

### Provider Pattern with Decorators
All data providers implement the `Provider` interface from `pkg/provider/interfaces.go`:
```go
type Provider interface {
    Name() string
    IsHealthy() bool
    GetRateLimit() time.Duration
}
```

**Decorator Chain Pattern**: Providers are enhanced through composable decorators in `pkg/provider/decorators/`:
- `simplified_frequency_control.go` - Rate limiting with exponential backoff
- `simplified_circuit_breaker.go` - Circuit breaker pattern for fault tolerance
- `factory.go` - Centralized decorator chain creation and management

**Example**: Tencent provider → Frequency Control → Circuit Breaker → Redis Publisher

### Standardized Message Format
All data flows through `pkg/message/types.go` with structured formats:
```go
type MessageFormat struct {
    Header   MessageHeader   // Producer info, timestamps, versioning
    Metadata MessageMetadata // Provider, data type, market info
    Payload  interface{}     // Actual stock/index data
    Checksum string          // Data integrity verification
}
```

## Development Workflows

### Configuration-Driven Job Scheduling
The `config/jobs.yaml` uses cron expressions for flexible scheduling:
```yaml
jobs:
  - name: "fetch-realtime-stock-ashare-main"
    schedule: "*/5 * 9-11,13-14 * * 1-5"  # Every 5s during trading hours, weekdays
    provider: { name: "tencent", type: "RealtimeStock" }
    params: { symbols: ["600000", "000001"] }
    output: { type: "redis_stream", stream: "stream:stock:realtime" }
```

### Build & Run Commands
```bash
# Main data producer (primary service)
go run ./cmd/fetcher --config config/jobs.yaml --redis localhost:6379

# API monitoring with CSV export
go run ./cmd/api_monitor -symbols=600000,000001 -duration=5m -interval=3s
go run ./cmd/api_monitor -symbols=600000 -duration=24h -data-dir=./collected_data

# InfluxDB time-series collector
go run ./cmd/influxdb_collector --config config/influxdb_collector.yaml

# Production build
GOOS=linux GOARCH=amd64 go build -o stocksub-linux ./cmd/stocksub
```

### Testing Commands
```bash
# Run All Unit Tests:
go test -v ./pkg/...
# Run All Integration Tests:
go test -v -tags=integration ./pkg/...
# Run Integration Tests for a Specific Package:
go test -v -tags=integration ./pkg/provider/tencent/
# Run System-Level Tests:
go test -v -tags=integration ./tests/
# Run Performance Benchmarks:
go test -v -bench=. -benchmem ./pkg/testkit/
go test -v -coverprofile=coverage.out ./pkg/...
go tool cover -func=coverage.out
```

## Testing Architecture

### Test Organization
- **`tests/`** - System-level and broad integration tests.
- **`pkg/testkit/`** - Comprehensive test utilities:
    - **`config/`** - Test configuration management with storage, cache, and provider settings.
    - **`manager/`** - Unified test data manager integrating cache, storage, and provider layers.
    - **`providers/`** - Mock and cached provider implementations with Tencent data generators.
    - Core files: `interfaces.go` (TestDataManager, MockProvider), `benchmark_test.go`, `integration_test.go`

### Testing Standards

* Organize tests by functional modules, one test file per source file.
* Focus each test file on specific functionality.
* Cover core features, edge cases, error handling, and concurrency safety.
* Maintain high test coverage (target >80%).
* Keep test code quality matching implementation code.
* Test cases should clearly state the testing purpose in Chinese.

### Test Types and Relevant regulations

* Unit Tests
    - **File Pattern:** `*_test.go`
    - **Purpose:** Test individual functions/methods within a package.
    - **Location:** Same directory as the source file (e.g., `config.go` -> `config_test.go`).
    - **Package:** `package xxx` (internal) or `package xxx_test` (external API).

* Integration Tests
    - **File Pattern:** `integration_test.go`, `*_integration_test.go`
    - **Purpose:** Test component interactions or external system integration (e.g., Tencent API).
    - **Location:** Same directory as the primary source.
    - **Package:** `package xxx_test` (e.g., `package tencent_test`).
    - **Build Tag:** Requires `//go:build integration` for selective execution.

* Benchmark Tests
    - **File Pattern:** `benchmark_test.go`, `*_benchmark_test.go`
    - **Purpose:** Measure performance (time, memory).
    - **Location:** Same directory as the source file.
    - **Package:** `package xxx` or `package xxx_test`.
    - **Example:** `pkg/testkit/benchmark_test.go` for `testkit` utilities.

* System/E2E Tests
    - **File Pattern:** `tests/*_test.go`
    - **Purpose:** System-level and End-to-End tests located in this directory.
    - **Location:** Centralized in the `tests/` directory.
    - **Package:** `package tests`.
    - **Example:** `tests/system_test.go`.

### Test Case Naming Conventions

* Unit/Integration Tests: `Test[Type][Method][Scenario]`
    - **Examples:**
    - `TestProvider_FetchData_WithValidSymbols_ReturnsData`
    - `TestConfig_Validate_WithEmptyProvider_ReturnsError`

* Benchmark Tests: `Benchmark[Type][Operation][Scenario]`
    - **Examples:**
    - `BenchmarkCSVStorage_Save_SingleItem`
    - `BenchmarkMemoryCache_Get_ConcurrentAccess`

## Configuration & Conventions

### Stock Symbol Format
- **Shanghai**: `600000`, `601398` (6-digit numbers)
- **Shenzhen**: `000001`, `300750` (6-digit numbers)
- **Beijing Stock Exchange**: `835174`, `832000`
- **STAR Market**: `688036`, `688599`
- **Critical**: No prefixes (sh/sz) - use raw 6-digit codes only

### Rate Limiting Constraints
- **Minimum interval**: 200ms between requests (Tencent API limit)
- **Recommended production**: ≥5 seconds
- **Request batching**: Multiple symbols per API call
- **Retry pattern**: 3 retries with exponential backoff
- Centralized API call management with circuit breaking
- Error classification and retry strategy management

### Key Patterns and Conventions

#### Trading Hours Management
Use `pkg/timing/MarketTime` for market trading session detection:
```go
marketTime := timing.NewMarketTime(&timing.SystemTimeService{})
if marketTime.IsTradingHours() {
    // Execute trading-related operations
}
```

#### Intelligent Rate Limiting
The `pkg/limiter/IntelligentLimiter` provides batch-aware circuit breaking:
```go
limiter := limiter.NewIntelligentLimiter(classifier, marketTime)
if limiter.ShouldSkip(symbols) {
    return errors.New("rate limit exceeded")
}
```

#### Structured Data Storage
Use `pkg/storage` for flexible data persistence patterns:
```go
schema := storage.DefineSchema([]storage.FieldDefinition{...})
csvStorage := storage.NewCSVStorage(directory)
csvStorage.SaveStructuredData(data, schema)
```

#### Job Scheduling Pattern
Jobs are managed through `pkg/scheduler` with cron expressions:
```go
scheduler := scheduler.NewJobScheduler()
job := &scheduler.Job{
    ID: uuid.New().String(),
    Name: "fetch-realtime-data",
    Schedule: "*/5 * 9-11,13-14 * * 1-5",
}
scheduler.AddJob(job)
```

#### Mock Testing Pattern
For external API dependencies, use testkit's unified manager system:
```go
cfg := &config.Config{
    Cache:   config.CacheConfig{Type: "memory"},
    Storage: config.StorageConfig{Type: "csv", Directory: "_data"},
    Provider: config.ProviderConfig{Type: "mock"},
}
manager := manager.NewTestDataManager(cfg)
manager.EnableMock(true)
manager.SetMockData(symbols, mockData)  // Avoid real API calls in tests
```

#### Decorator Chain Creation
Use factory pattern for consistent provider enhancement:
```go
decoratedProvider, err := decorators.CreateDecoratedProvider(
    baseProvider, 
    decorators.DefaultDecoratorConfig()
)
```

#### Message Publishing to Redis Streams
All services follow standardized message publishing:
```go
msgFormat := message.NewMessageFormat(producer, provider, dataType, payload)
// Publish to stream with checksum validation
```

## Key Implementation Notes

### Provider Registration Pattern
Providers must be registered with type assertions:
```go
if realtimeProvider, ok := decoratedProvider.(provider.RealtimeStockProvider); ok {
    providerManager.RegisterRealtimeStockProvider("tencent", realtimeProvider)
}
```

### Graceful Shutdown Handling
All main services implement signal handling with context cancellation:
```go
ctx, cancel := context.WithCancel(context.Background())
// Listen for SIGINT, SIGTERM signals for graceful shutdown
```

### Cron-Based Job Scheduling
Jobs are defined in YAML with cron expressions for trading hour precision:
- `*/5 * 9-11,13-14 * * 1-5` - Every 5s during trading hours, weekdays only
- Markets: Shanghai (600xxx), Shenzhen (000xxx/300xxx), Beijing (8xxxxx), STAR (688xxx)

[byterover-mcp]

# important 
always use byterover-retrieve-knowledge tool to get the related context before any tasks 
always use byterover-store-knowledge to store all the critical informations after sucessful tasks