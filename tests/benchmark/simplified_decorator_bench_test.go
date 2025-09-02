package benchmark_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"stocksub/pkg/core"
	"stocksub/pkg/provider/decorators"
)

// MockStockProvider 模拟股票数据提供商
type MockStockProvider struct {
	callCount    int
	failInterval int
}

func NewMockStockProvider(failInterval int) *MockStockProvider {
	return &MockStockProvider{
		callCount:    0,
		failInterval: failInterval,
	}
}

func (p *MockStockProvider) Name() string {
	return "MockStockProvider"
}

func (p *MockStockProvider) IsHealthy() bool {
	return true
}

func (p *MockStockProvider) GetRateLimit() time.Duration {
	return 100 * time.Millisecond
}

func (p *MockStockProvider) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	p.callCount++

	// 模拟故障
	if p.failInterval > 0 && p.callCount%p.failInterval == 0 {
		return nil, fmt.Errorf("simulated network error")
	}

	// 模拟IP封禁
	if p.callCount%8 == 0 {
		return nil, fmt.Errorf("IP banned - too many requests")
	}

	mockData := []core.StockData{
		{Symbol: "000001.SZ", Name: "平安银行", Price: 12.34, Change: 0.56, ChangePercent: 4.75, Volume: 1000000, Timestamp: time.Now()},
		{Symbol: "000002.SZ", Name: "万科A", Price: 23.45, Change: -0.12, ChangePercent: -0.51, Volume: 800000, Timestamp: time.Now()},
	}

	result := make([]core.StockData, 0, len(symbols))
	for _, symbol := range symbols {
		for _, data := range mockData {
			if data.Symbol == symbol {
				newData := data
				newData.Timestamp = time.Now()
				result = append(result, newData)
				break
			}
		}
	}

	return result, nil
}

func (p *MockStockProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	data, err := p.FetchStockData(ctx, symbols)
	if err != nil {
		return nil, "", err
	}
	raw := fmt.Sprintf("Raw response for symbols: %v (call #%d)", symbols, p.callCount)
	return data, raw, nil
}

func (p *MockStockProvider) IsSymbolSupported(symbol string) bool {
	return true
}

func (p *MockStockProvider) GetCallCount() int {
	return p.callCount
}

// BenchmarkSimplifiedFrequencyControlProvider 基准测试简化频率控制装饰器
func BenchmarkSimplifiedFrequencyControlProvider(b *testing.B) {
	baseProvider := NewMockStockProvider(0) // 不模拟故障
	config := decorators.DefaultSimplifiedFrequencyControlConfig()
	config.MinInterval = 10 * time.Millisecond // 使用较短的间隔以提高测试速度
	config.MarketTimeAware = true

	decoratedProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, config)
	ctx := context.Background()
	symbols := []string{"000001.SZ"}

	// 预热
	_, _ = decoratedProvider.FetchStockData(ctx, symbols)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := decoratedProvider.FetchStockData(ctx, symbols)
		if err != nil {
			b.Fatalf("基准测试失败: %v", err)
		}
	}
}

// BenchmarkSimplifiedCircuitBreakerProvider 基准测试简化熔断器装饰器
func BenchmarkSimplifiedCircuitBreakerProvider(b *testing.B) {
	baseProvider := NewMockStockProvider(2) // 每2次调用失败一次
	config := decorators.DefaultSimplifiedCircuitBreakerConfig()
	config.ReadyToTrip = 10 // 提高阈值以减少熔断影响
	config.Interval = 5 * time.Second
	config.Timeout = 3 * time.Second

	decoratedProvider := decorators.NewSimplifiedCircuitBreakerProvider(baseProvider, config)
	ctx := context.Background()
	symbols := []string{"000001.SZ"}

	// 预热
	_, _ = decoratedProvider.FetchStockData(ctx, symbols)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := decoratedProvider.FetchStockData(ctx, symbols)
		if err != nil {
			// 在基准测试中，我们接受错误作为正常情况的一部分
			// 因为我们正在测试性能，而不是正确性
		}
	}
}

// BenchmarkSimplifiedDecoratorsCombined 基准测试组合使用简化装饰器
func BenchmarkSimplifiedDecoratorsCombined(b *testing.B) {
	baseProvider := NewMockStockProvider(3)

	// 创建频率控制装饰器
	freqConfig := decorators.DefaultSimplifiedFrequencyControlConfig()
	freqConfig.MinInterval = 10 * time.Millisecond // 使用较短的间隔以提高测试速度
	freqProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, freqConfig)

	// 创建熔断器装饰器
	cbConfig := decorators.DefaultSimplifiedCircuitBreakerConfig()
	cbConfig.ReadyToTrip = 10 // 提高阈值以减少熔断影响
	cbProvider := decorators.NewSimplifiedCircuitBreakerProvider(freqProvider, cbConfig)

	ctx := context.Background()
	symbols := []string{"000001.SZ", "000002.SZ"}

	// 预热
	_, _ = cbProvider.FetchStockData(ctx, symbols)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := cbProvider.FetchStockData(ctx, symbols)
		if err != nil {
			// 在基准测试中，我们接受错误作为正常情况的一部分
		}
	}
}

// BenchmarkSimplifiedDecoratorsMemoryUsage 内存使用基准测试
func BenchmarkSimplifiedDecoratorsMemoryUsage(b *testing.B) {
	b.Run("SimplifiedFrequencyControlProvider_Memory", func(b *testing.B) {
		baseProvider := NewMockStockProvider(0)
		config := decorators.DefaultSimplifiedFrequencyControlConfig()
		config.MinInterval = 10 * time.Millisecond
		config.MarketTimeAware = true

		decoratedProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, config)
		ctx := context.Background()
		symbols := []string{"000001.SZ"}

		// 预热
		_, _ = decoratedProvider.FetchStockData(ctx, symbols)

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = decoratedProvider.FetchStockData(ctx, symbols)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
		b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
	})

	b.Run("SimplifiedCircuitBreakerProvider_Memory", func(b *testing.B) {
		baseProvider := NewMockStockProvider(2)
		config := decorators.DefaultSimplifiedCircuitBreakerConfig()
		config.ReadyToTrip = 10
		config.Interval = 5 * time.Second
		config.Timeout = 3 * time.Second

		decoratedProvider := decorators.NewSimplifiedCircuitBreakerProvider(baseProvider, config)
		ctx := context.Background()
		symbols := []string{"000001.SZ"}

		// 预热
		_, _ = decoratedProvider.FetchStockData(ctx, symbols)

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = decoratedProvider.FetchStockData(ctx, symbols)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
		b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
	})
}

// BenchmarkSimplifiedDecoratorsConcurrency 并发性能基准测试
func BenchmarkSimplifiedDecoratorsConcurrency(b *testing.B) {
	b.Run("Concurrent_SimplifiedFrequencyControlProvider", func(b *testing.B) {
		baseProvider := NewMockStockProvider(0)
		config := decorators.DefaultSimplifiedFrequencyControlConfig()
		config.MinInterval = 10 * time.Millisecond
		config.MarketTimeAware = true

		decoratedProvider := decorators.NewSimplifiedFrequencyControlProvider(baseProvider, config)
		ctx := context.Background()
		symbols := []string{"000001.SZ"}

		// 预热
		_, _ = decoratedProvider.FetchStockData(ctx, symbols)

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := decoratedProvider.FetchStockData(ctx, symbols)
				if err != nil {
					b.Fatalf("并发测试失败: %v", err)
				}
			}
		})
	})

	b.Run("Concurrent_SimplifiedCircuitBreakerProvider", func(b *testing.B) {
		baseProvider := NewMockStockProvider(2)
		config := decorators.DefaultSimplifiedCircuitBreakerConfig()
		config.ReadyToTrip = 10
		config.Interval = 5 * time.Second
		config.Timeout = 3 * time.Second

		decoratedProvider := decorators.NewSimplifiedCircuitBreakerProvider(baseProvider, config)
		ctx := context.Background()
		symbols := []string{"000001.SZ"}

		// 预热
		_, _ = decoratedProvider.FetchStockData(ctx, symbols)

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := decoratedProvider.FetchStockData(ctx, symbols)
				if err != nil {
					// 在并发基准测试中，我们接受错误作为正常情况的一部分
				}
			}
		})
	})
}
