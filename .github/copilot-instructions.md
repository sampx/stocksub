# Project Context for AI Agents

## Project Overview
StockSub is an enterprise-grade A-share real-time data subscription service built in Go, specialized for mainland Chinese markets with support for Shanghai (6xxxxx), Shenzhen (0xxxxx/3xxxxx), Beijing Stock Exchange (8xxxxx), and STAR Market (688xxx) stocks.

## Important Warnings

⚠️ **API Frequency Warning [VERY VERY IMPORTANT]**:  Tencent API imposes strict frequency limits. Never perform API access tests without proper authorization. Notify users before testing actual endpoints. High-frequency access may cause IP bans.

## Core Architecture

### Directory Structure

- **`cmd/stocksub/`** - Main application with graceful shutdown
- **`cmd/fetcher/`** - Standalone data provider service that fetches from sources (Tencent, Sina) and publishes to Redis Streams.
- **`cmd/api_server/`** - API server that likely consumes data from Redis to serve clients.
- **`cmd/api_monitor/`** - Long-term API monitoring with intelligent rate limiting
- **`cmd/api_analyzer/`** - Data analysis tools for CSV data
- **`pkg/subscriber/`** - Core subscription engine (subscriber, manager, types)
- **`pkg/provider/`** - Data source implementations (Tencent, Sina) and decorators.
- **`pkg/message/`** - Standardized data message formats for Redis Streams.
- **`pkg/config/`** - Configuration management with validation
- **`pkg/logger/`** - Structured logging with logrus
- **`pkg/limiter/`** - Intelligent rate limiting and error classification
- **`pkg/testkit/`** - Comprehensive test utilities
- **`tests/`** - System/E2E tests

### Distributed Architecture & Data Flow

The system is evolving into a distributed architecture using Redis Streams as a message bus.

1.  **Data Producer (`cmd/fetcher`)**: This is the primary service for data acquisition.
    - It loads job configurations from `config/jobs.yaml`.
    - It uses providers from `pkg/provider/` (e.g., `tencent`, `sina`) to fetch real-time stock data.
    - Providers are enhanced with decorators (`pkg/provider/decorators`) for caching, rate limiting, etc.
    - Fetched data is standardized into a `message.MessageFormat` (`pkg/message/`).
    - The final message is published to a Redis Stream (e.g., `stock:stream:stock_realtime`).

2.  **Data Consumers (e.g., `cmd/api_server`)**: Other services connect to Redis and consume data from the streams for their specific purposes (e.g., serving API requests, storing in a database).

3.  **Decoupling**: This producer/consumer model decouples data acquisition from data delivery, improving scalability and resilience.

## Development Workflows

### Build & Run Commands
```bash
# Main application (legacy)
go run ./cmd/stocksub

# Fetcher (primary data producer)
go run ./cmd/fetcher --config config/jobs.yaml --redis localhost:6379

# API Monitoring (long-term data collection)
go run ./cmd/api_monitor -symbols=600000,000001 -duration=5m -interval=3s
go run ./cmd/api_monitor -symbols=600000 -duration=24h -data-dir=./collected_data

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
    - **`cache/`** - Layered caching with eviction policies.
    - **`storage/`** - CSV and memory storage implementations.
    - **`providers/`** - Mock and cached provider implementations.
    - **`helpers/`** - Resource management utilities.

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
    - **Location:** Package directory or `pkg/testkit/`.
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


- 由于本项目有对外 api 调用限制,你不能自己运行集成测试和基准测试,如果非常必要,你要让用户自己运行.