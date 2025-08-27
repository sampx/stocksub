package decorators

import (
	"context"
	"errors"
	"stocksub/pkg/provider/core"
	"stocksub/pkg/subscriber"
	"testing"
	"time"
)

// MockRealtimeStockProvider 用于测试的模拟 Provider
type MockRealtimeStockProvider struct {
	name        string
	rateLimit   time.Duration
	healthy     bool
	failNext    bool
	callCount   int
	lastSymbols []string
}

func NewMockRealtimeStockProvider(name string) *MockRealtimeStockProvider {
	return &MockRealtimeStockProvider{
		name:      name,
		rateLimit: 100 * time.Millisecond,
		healthy:   true,
		failNext:  false,
	}
}

func (m *MockRealtimeStockProvider) Name() string {
	return m.name
}

func (m *MockRealtimeStockProvider) GetRateLimit() time.Duration {
	return m.rateLimit
}

func (m *MockRealtimeStockProvider) IsHealthy() bool {
	return m.healthy
}

func (m *MockRealtimeStockProvider) FetchStockData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	m.callCount++
	m.lastSymbols = symbols

	if m.failNext {
		m.failNext = false
		return nil, errors.New("模拟错误")
	}

	// 模拟返回数据
	data := make([]subscriber.StockData, len(symbols))
	for i, symbol := range symbols {
		data[i] = subscriber.StockData{
			Symbol: symbol,
			Name:   "测试股票",
			Price:  10.50,
			Change: 0.15,
		}
	}
	return data, nil
}

func (m *MockRealtimeStockProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]subscriber.StockData, string, error) {
	data, err := m.FetchStockData(ctx, symbols)
	return data, "mock_raw_data", err
}

func (m *MockRealtimeStockProvider) IsSymbolSupported(symbol string) bool {
	return len(symbol) == 6
}

func (m *MockRealtimeStockProvider) SetFailNext(fail bool) {
	m.failNext = fail
}

func (m *MockRealtimeStockProvider) SetHealthy(healthy bool) {
	m.healthy = healthy
}

func (m *MockRealtimeStockProvider) GetCallCount() int {
	return m.callCount
}

func (m *MockRealtimeStockProvider) GetLastSymbols() []string {
	return m.lastSymbols
}

func TestBaseDecorator_基础功能测试(t *testing.T) {
	mockProvider := NewMockRealtimeStockProvider("TestProvider")

	decorator := NewBaseDecorator(mockProvider)

	// 测试基础接口代理
	if decorator.Name() != "TestProvider" {
		t.Errorf("期望名称 'TestProvider'，得到 '%s'", decorator.Name())
	}

	if decorator.GetRateLimit() != 100*time.Millisecond {
		t.Errorf("期望频率限制 100ms，得到 %v", decorator.GetRateLimit())
	}

	if !decorator.IsHealthy() {
		t.Error("期望健康状态为 true")
	}

	// 测试获取基础提供商
	if decorator.GetBaseProvider() != mockProvider {
		t.Error("GetBaseProvider 应该返回原始提供商")
	}
}

func TestRealtimeStockBaseDecorator_基础功能测试(t *testing.T) {
	mockProvider := NewMockRealtimeStockProvider("TestProvider")

	decorator := NewRealtimeStockBaseDecorator(mockProvider)

	ctx := context.Background()
	symbols := []string{"600000", "000001"}

	// 测试 FetchStockData
	data, err := decorator.FetchStockData(ctx, symbols)
	if err != nil {
		t.Fatalf("期望无错误，得到: %v", err)
	}

	if len(data) != 2 {
		t.Errorf("期望返回 2 条数据，得到 %d 条", len(data))
	}

	if data[0].Symbol != "600000" {
		t.Errorf("期望第一个股票代码为 '600000'，得到 '%s'", data[0].Symbol)
	}

	// 测试 FetchStockDataWithRaw
	dataWithRaw, raw, err := decorator.FetchStockDataWithRaw(ctx, symbols)
	if err != nil {
		t.Fatalf("期望无错误，得到: %v", err)
	}

	if raw != "mock_raw_data" {
		t.Errorf("期望原始数据 'mock_raw_data'，得到 '%s'", raw)
	}

	if len(dataWithRaw) != 2 {
		t.Errorf("期望返回 2 条数据，得到 %d 条", len(dataWithRaw))
	}

	// 测试 IsSymbolSupported
	if !decorator.IsSymbolSupported("600000") {
		t.Error("期望支持 6 位股票代码")
	}

	if decorator.IsSymbolSupported("12345") {
		t.Error("期望不支持 5 位代码")
	}
}

func TestDecoratorChain_链式组装测试(t *testing.T) {
	mockProvider := NewMockRealtimeStockProvider("BaseProvider")

	chain := NewDecoratorChain()

	// 添加测试装饰器
	chain.AddDecorator(func(provider core.Provider) core.Provider {
		if stockProvider, ok := provider.(core.RealtimeStockProvider); ok {
			return &TestDecorator1{
				RealtimeStockBaseDecorator: NewRealtimeStockBaseDecorator(stockProvider),
			}
		}
		return provider
	})

	chain.AddDecorator(func(provider core.Provider) core.Provider {
		if stockProvider, ok := provider.(core.RealtimeStockProvider); ok {
			return &TestDecorator2{
				RealtimeStockBaseDecorator: NewRealtimeStockBaseDecorator(stockProvider),
			}
		}
		return provider
	})

	// 应用装饰器链
	decorated := chain.Apply(mockProvider)

	// 验证装饰器链的应用顺序
	if decorator2, ok := decorated.(*TestDecorator2); ok {
		if decorator1, ok := decorator2.GetBaseProvider().(*TestDecorator1); ok {
			if baseProvider, ok := decorator1.GetBaseProvider().(*MockRealtimeStockProvider); ok {
				if baseProvider.Name() != "BaseProvider" {
					t.Errorf("期望基础提供商名称 'BaseProvider'，得到 '%s'", baseProvider.Name())
				}
			} else {
				t.Error("装饰器链底层应该是 MockRealtimeStockProvider")
			}
		} else {
			t.Error("第二层装饰器下应该是 TestDecorator1")
		}
	} else {
		t.Error("装饰器链顶层应该是 TestDecorator2")
	}
}

func TestDecoratorFactory_工厂模式测试(t *testing.T) {
	factory := NewDecoratorFactory()

	if factory == nil {
		t.Error("装饰器工厂应该成功创建")
	}

	// 基础工厂功能验证
	// 具体的创建功能将在各装饰器的专门测试中验证
}

// 测试用装饰器

type TestDecorator1 struct {
	*RealtimeStockBaseDecorator
}

func (d *TestDecorator1) Name() string {
	return "TestDecorator1(" + d.stockProvider.Name() + ")"
}

type TestDecorator2 struct {
	*RealtimeStockBaseDecorator
}

func (d *TestDecorator2) Name() string {
	return "TestDecorator2(" + d.stockProvider.Name() + ")"
}
