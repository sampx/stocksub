package core

import (
	"context"
	"errors"
	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/core"
	"testing"
	"time"
)

// mockLegacyProvider 模拟旧版 Provider
type mockLegacyProvider struct {
	name          string
	rateLimit     time.Duration
	supportedSyms map[string]bool
	mockData      []subscriber.StockData
	mockError     error
}

func (m *mockLegacyProvider) Name() string {
	return m.name
}

func (m *mockLegacyProvider) GetRateLimit() time.Duration {
	return m.rateLimit
}

func (m *mockLegacyProvider) IsSymbolSupported(symbol string) bool {
	if m.supportedSyms == nil {
		return true
	}
	return m.supportedSyms[symbol]
}

func (m *mockLegacyProvider) FetchData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	if m.mockError != nil {
		return nil, m.mockError
	}
	return m.mockData, nil
}

// mockTestKitProvider 模拟 testkit Provider
type mockTestKitProvider struct {
	mockData  []subscriber.StockData
	mockError error
	mockMode  bool
}

func (m *mockTestKitProvider) FetchData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	if m.mockError != nil {
		return nil, m.mockError
	}
	return m.mockData, nil
}

func (m *mockTestKitProvider) SetMockMode(enabled bool) {
	m.mockMode = enabled
}

func (m *mockTestKitProvider) SetMockData(symbols []string, data []subscriber.StockData) {
	m.mockData = data
}

func (m *mockTestKitProvider) Close() error {
	return nil
}

func TestLegacyProviderAdapter(t *testing.T) {
	// 准备测试数据
	testData := []subscriber.StockData{
		{
			Symbol: "600000",
			Name:   "浦发银行",
			Price:  10.50,
		},
	}

	mockProvider := &mockLegacyProvider{
		name:      "test_provider",
		rateLimit: 500 * time.Millisecond,
		mockData:  testData,
	}

	// 创建适配器
	adapter := NewLegacyProviderAdapter(mockProvider)

	// 测试 Name 方法
	if adapter.Name() != "test_provider" {
		t.Errorf("Expected name 'test_provider', got '%s'", adapter.Name())
	}

	// 测试 GetRateLimit 方法
	if adapter.GetRateLimit() != 500*time.Millisecond {
		t.Errorf("Expected rate limit 500ms, got %v", adapter.GetRateLimit())
	}

	// 测试 IsHealthy 方法
	if !adapter.IsHealthy() {
		t.Error("Expected adapter to be healthy")
	}

	// 测试 FetchStockData 方法
	ctx := context.Background()
	symbols := []string{"600000"}
	data, err := adapter.FetchStockData(ctx, symbols)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(data) != 1 || data[0].Symbol != "600000" {
		t.Errorf("Expected data for '600000', got %+v", data)
	}

	// 测试 FetchStockDataWithRaw 方法
	dataWithRaw, raw, err := adapter.FetchStockDataWithRaw(ctx, symbols)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(dataWithRaw) != 1 || dataWithRaw[0].Symbol != "600000" {
		t.Errorf("Expected data for '600000', got %+v", dataWithRaw)
	}
	if raw != "" {
		t.Errorf("Expected empty raw data, got '%s'", raw)
	}

	// 测试 IsSymbolSupported 方法
	if !adapter.IsSymbolSupported("600000") {
		t.Error("Expected symbol '600000' to be supported")
	}

	// 测试错误情况
	mockProvider.mockError = errors.New("test error")
	_, err = adapter.FetchStockData(ctx, symbols)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestTestKitProviderAdapter(t *testing.T) {
	// 准备测试数据
	testData := []subscriber.StockData{
		{
			Symbol: "000001",
			Name:   "平安银行",
			Price:  15.20,
		},
	}

	mockProvider := &mockTestKitProvider{
		mockData: testData,
	}

	// 创建适配器
	adapter := NewTestKitProviderAdapter(mockProvider)

	// 测试基本方法
	if adapter.Name() != "testkit_provider" {
		t.Errorf("Expected name 'testkit_provider', got '%s'", adapter.Name())
	}

	if adapter.GetRateLimit() != 200*time.Millisecond {
		t.Errorf("Expected default rate limit 200ms, got %v", adapter.GetRateLimit())
	}

	if !adapter.IsHealthy() {
		t.Error("Expected adapter to be healthy")
	}

	// 测试数据获取
	ctx := context.Background()
	symbols := []string{"000001"}
	data, err := adapter.FetchStockData(ctx, symbols)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(data) != 1 || data[0].Symbol != "000001" {
		t.Errorf("Expected data for '000001', got %+v", data)
	}

	// 测试 Mock 功能
	adapter.SetMockMode(true)
	if !mockProvider.mockMode {
		t.Error("Expected mock mode to be set")
	}

	newData := []subscriber.StockData{{Symbol: "300750", Price: 25.0}}
	adapter.SetMockData([]string{"300750"}, newData)
	if len(mockProvider.mockData) != 1 || mockProvider.mockData[0].Symbol != "300750" {
		t.Errorf("Expected mock data to be set, got %+v", mockProvider.mockData)
	}

	// 测试符号支持（testkit 适配器总是返回 true）
	if !adapter.IsSymbolSupported("any_symbol") {
		t.Error("Expected all symbols to be supported in testkit adapter")
	}
}

func TestSmartProviderAdapter(t *testing.T) {
	// 测试新接口提供商
	newProvider := &mockLegacyProvider{name: "new_provider"}
	smartAdapter := NewSmartProviderAdapter(newProvider)
	if !smartAdapter.IsSupported() {
		t.Error("Expected smart adapter to support legacy provider")
	}
	if smartAdapter.Name() != "new_provider" {
		t.Errorf("Expected name 'new_provider', got '%s'", smartAdapter.Name())
	}

	// 测试 testkit 提供商
	testkitProvider := &mockTestKitProvider{}
	smartAdapter = NewSmartProviderAdapter(testkitProvider)
	if !smartAdapter.IsSupported() {
		t.Error("Expected smart adapter to support testkit provider")
	}
	if smartAdapter.Name() != "testkit_provider" {
		t.Errorf("Expected name 'testkit_provider', got '%s'", smartAdapter.Name())
	}

	// 测试不支持的类型
	unsupportedProvider := "not_a_provider"
	smartAdapter = NewSmartProviderAdapter(unsupportedProvider)
	if smartAdapter.IsSupported() {
		t.Error("Expected smart adapter to not support string type")
	}

	// 测试获取原始提供商
	if smartAdapter.GetOriginalProvider() != unsupportedProvider {
		t.Error("Expected to get original provider")
	}

	// 测试不支持类型的方法调用
	if smartAdapter.Name() != "unknown_provider" {
		t.Error("Expected unknown_provider for unsupported type")
	}

	ctx := context.Background()
	_, err := smartAdapter.FetchStockData(ctx, []string{"test"})
	if err != ErrProviderNotSupported {
		t.Errorf("Expected ErrProviderNotSupported, got %v", err)
	}
}

func TestNewProviderAdapter(t *testing.T) {
	// 测试 nil 输入
	result := NewProviderAdapter(nil)
	if result != nil {
		t.Error("Expected nil result for nil input")
	}

	// 测试旧版提供商
	legacyProvider := &mockLegacyProvider{name: "legacy"}
	result = NewProviderAdapter(legacyProvider)
	if result == nil {
		t.Error("Expected non-nil result for legacy provider")
	}
	if result.Name() != "legacy" {
		t.Errorf("Expected name 'legacy', got '%s'", result.Name())
	}

	// 测试 testkit 提供商
	testkitProvider := &mockTestKitProvider{}
	// 注意：NewProviderAdapter 只处理 subscriber.Provider，不处理 testkit.Provider
	// 所以这里应该返回 nil
	result = NewProviderAdapter(testkitProvider)
	if result != nil {
		t.Error("Expected nil result for testkit provider in NewProviderAdapter")
	}

	// 测试不支持的类型
	result = NewProviderAdapter("unsupported")
	if result != nil {
		t.Error("Expected nil result for unsupported type")
	}
}

// 确保所有接口都正确实现
func TestInterfaceCompliance(t *testing.T) {
	var _ RealtimeStockProvider = (*LegacyProviderAdapter)(nil)
	var _ RealtimeStockProvider = (*TestKitProviderAdapter)(nil)
	var _ RealtimeStockProvider = (*SmartProviderAdapter)(nil)
	var _ subscriber.Provider = (*mockLegacyProvider)(nil)
	var _ core.Provider = (*mockTestKitProvider)(nil)
}
