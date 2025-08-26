package provider

import (
	"context"
	"stocksub/pkg/provider/core"
	"stocksub/pkg/subscriber"
	testkitCore "stocksub/pkg/testkit/core"
	"testing"
	"time"
)

// mockRealtimeStockProvider 模拟实时股票数据提供商
type mockRealtimeStockProvider struct {
	name string
}

func (m *mockRealtimeStockProvider) Name() string {
	return m.name
}

func (m *mockRealtimeStockProvider) GetRateLimit() time.Duration {
	return 200 * time.Millisecond
}

func (m *mockRealtimeStockProvider) IsHealthy() bool {
	return true
}

func (m *mockRealtimeStockProvider) FetchStockData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	return []subscriber.StockData{{Symbol: symbols[0], Price: 10.0}}, nil
}

func (m *mockRealtimeStockProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]subscriber.StockData, string, error) {
	data, err := m.FetchStockData(ctx, symbols)
	return data, "raw_data", err
}

func (m *mockRealtimeStockProvider) IsSymbolSupported(symbol string) bool {
	return true
}

// mockRealtimeIndexProvider 模拟实时指数数据提供商
type mockRealtimeIndexProvider struct {
	name string
}

func (m *mockRealtimeIndexProvider) Name() string {
	return m.name
}

func (m *mockRealtimeIndexProvider) GetRateLimit() time.Duration {
	return 300 * time.Millisecond
}

func (m *mockRealtimeIndexProvider) IsHealthy() bool {
	return true
}

func (m *mockRealtimeIndexProvider) FetchIndexData(ctx context.Context, indexSymbols []string) ([]core.IndexData, error) {
	return []core.IndexData{{Symbol: indexSymbols[0], Value: 3000.0}}, nil
}

func (m *mockRealtimeIndexProvider) IsIndexSupported(indexSymbol string) bool {
	return true
}

// mockHistoricalProvider 模拟历史数据提供商
type mockHistoricalProvider struct {
	name string
}

func (m *mockHistoricalProvider) Name() string {
	return m.name
}

func (m *mockHistoricalProvider) GetRateLimit() time.Duration {
	return 500 * time.Millisecond
}

func (m *mockHistoricalProvider) IsHealthy() bool {
	return true
}

func (m *mockHistoricalProvider) FetchHistoricalData(ctx context.Context, symbol string, start, end time.Time, period string) ([]core.HistoricalDataPoint, error) {
	return []core.HistoricalDataPoint{{Symbol: symbol, Close: 15.0}}, nil
}

func (m *mockHistoricalProvider) GetSupportedPeriods() []string {
	return []string{"1d", "1h"}
}

func (m *mockHistoricalProvider) IsSymbolSupported(symbol string) bool {
	return true
}

// mockLegacyProvider 模拟旧版提供商
type mockLegacyProvider struct {
	name string
}

func (m *mockLegacyProvider) Name() string {
	return m.name
}

func (m *mockLegacyProvider) GetRateLimit() time.Duration {
	return 400 * time.Millisecond
}

func (m *mockLegacyProvider) IsSymbolSupported(symbol string) bool {
	return true
}

func (m *mockLegacyProvider) FetchData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	return []subscriber.StockData{{Symbol: symbols[0], Price: 12.0}}, nil
}

// mockTestKitProvider 模拟 testkit 提供商
type mockTestKitProvider struct {
	name string
}

func (m *mockTestKitProvider) FetchData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	return []subscriber.StockData{{Symbol: symbols[0], Price: 8.0}}, nil
}

func (m *mockTestKitProvider) SetMockMode(enabled bool) {}

func (m *mockTestKitProvider) SetMockData(symbols []string, data []subscriber.StockData) {}

func (m *mockTestKitProvider) Close() error {
	return nil
}

func TestNewProviderManager(t *testing.T) {
	manager := NewProviderManager()
	if manager == nil {
		t.Error("Expected non-nil ProviderManager")
	}

	// 测试初始状态
	providers := manager.ListProviders()
	if len(providers) != 0 {
		t.Errorf("Expected empty provider list, got %v", providers)
	}
}

func TestProviderManager_RegisterRealtimeStockProvider(t *testing.T) {
	manager := NewProviderManager()
	provider := &mockRealtimeStockProvider{name: "test_stock"}

	// 正常注册
	err := manager.RegisterRealtimeStockProvider("test_stock", provider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证注册成功
	retrievedProvider, err := manager.GetRealtimeStockProvider("test_stock")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if retrievedProvider == nil {
		t.Error("Expected non-nil provider")
	}
	if retrievedProvider.Name() != "test_stock" {
		t.Errorf("Expected provider name 'test_stock', got '%s'", retrievedProvider.Name())
	}

	// 测试错误情况
	err = manager.RegisterRealtimeStockProvider("", provider)
	if err == nil {
		t.Error("Expected error for empty name")
	}

	err = manager.RegisterRealtimeStockProvider("test", nil)
	if err == nil {
		t.Error("Expected error for nil provider")
	}
}

func TestProviderManager_RegisterRealtimeIndexProvider(t *testing.T) {
	manager := NewProviderManager()
	provider := &mockRealtimeIndexProvider{name: "test_index"}

	err := manager.RegisterRealtimeIndexProvider("test_index", provider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	retrievedProvider, err := manager.GetRealtimeIndexProvider("test_index")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if retrievedProvider.Name() != "test_index" {
		t.Errorf("Expected provider name 'test_index', got '%s'", retrievedProvider.Name())
	}
}

func TestProviderManager_RegisterHistoricalProvider(t *testing.T) {
	manager := NewProviderManager()
	provider := &mockHistoricalProvider{name: "test_historical"}

	err := manager.RegisterHistoricalProvider("test_historical", provider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	retrievedProvider, err := manager.GetHistoricalProvider("test_historical")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if retrievedProvider.Name() != "test_historical" {
		t.Errorf("Expected provider name 'test_historical', got '%s'", retrievedProvider.Name())
	}
}

func TestProviderManager_RegisterLegacyProvider(t *testing.T) {
	manager := NewProviderManager()
	provider := &mockLegacyProvider{name: "test_legacy"}

	err := manager.RegisterLegacyProvider("test_legacy", provider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	retrievedProvider, err := manager.GetLegacyProvider("test_legacy")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if retrievedProvider.Name() != "test_legacy" {
		t.Errorf("Expected provider name 'test_legacy', got '%s'", retrievedProvider.Name())
	}
}

func TestProviderManager_RegisterProvider(t *testing.T) {
	manager := NewProviderManager()

	// 测试注册实时股票提供商
	stockProvider := &mockRealtimeStockProvider{name: "auto_stock"}
	err := manager.RegisterProvider("auto_stock", stockProvider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 测试注册指数提供商
	indexProvider := &mockRealtimeIndexProvider{name: "auto_index"}
	err = manager.RegisterProvider("auto_index", indexProvider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 测试注册历史数据提供商
	historicalProvider := &mockHistoricalProvider{name: "auto_historical"}
	err = manager.RegisterProvider("auto_historical", historicalProvider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 测试注册旧版提供商
	legacyProvider := &mockLegacyProvider{name: "auto_legacy"}
	err = manager.RegisterProvider("auto_legacy", legacyProvider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 测试注册 testkit 提供商（通过智能适配器）
	testkitProvider := &mockTestKitProvider{name: "auto_testkit"}
	err = manager.RegisterProvider("auto_testkit", testkitProvider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证所有提供商都已注册
	providers := manager.ListProviders()
	expectedTypes := []ProviderType{TypeRealtimeStock, TypeRealtimeIndex, TypeHistorical, TypeLegacy}
	for _, providerType := range expectedTypes {
		if _, exists := providers[providerType]; !exists {
			t.Errorf("Expected provider type %s to be registered", providerType)
		}
	}

	// 测试不支持的类型
	err = manager.RegisterProvider("unsupported", "not_a_provider")
	if err == nil {
		t.Error("Expected error for unsupported provider type")
	}

	// 测试错误情况
	err = manager.RegisterProvider("", stockProvider)
	if err == nil {
		t.Error("Expected error for empty name")
	}

	err = manager.RegisterProvider("test", nil)
	if err == nil {
		t.Error("Expected error for nil provider")
	}
}

func TestProviderManager_GetRealtimeStockProvider_WithLegacyFallback(t *testing.T) {
	manager := NewProviderManager()

	// 注册旧版提供商
	legacyProvider := &mockLegacyProvider{name: "legacy_fallback"}
	err := manager.RegisterLegacyProvider("legacy_fallback", legacyProvider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 通过 GetRealtimeStockProvider 获取（应该自动适配）
	retrievedProvider, err := manager.GetRealtimeStockProvider("legacy_fallback")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if retrievedProvider == nil {
		t.Error("Expected non-nil provider")
	}

	// 验证适配器工作正常
	ctx := context.Background()
	data, err := retrievedProvider.FetchStockData(ctx, []string{"600000"})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(data) != 1 || data[0].Symbol != "600000" {
		t.Errorf("Expected data for '600000', got %+v", data)
	}
}

func TestProviderManager_GetProvider_NotFound(t *testing.T) {
	manager := NewProviderManager()

	// 测试获取不存在的提供商
	_, err := manager.GetRealtimeStockProvider("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent provider")
	}

	_, err = manager.GetRealtimeIndexProvider("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent provider")
	}

	_, err = manager.GetHistoricalProvider("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent provider")
	}

	_, err = manager.GetLegacyProvider("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent provider")
	}
}

func TestProviderManager_ListProviders(t *testing.T) {
	manager := NewProviderManager()

	// 注册不同类型的提供商
	stockProvider := &mockRealtimeStockProvider{name: "stock1"}
	indexProvider := &mockRealtimeIndexProvider{name: "index1"}
	historicalProvider := &mockHistoricalProvider{name: "historical1"}
	legacyProvider := &mockLegacyProvider{name: "legacy1"}

	manager.RegisterRealtimeStockProvider("stock1", stockProvider)
	manager.RegisterRealtimeIndexProvider("index1", indexProvider)
	manager.RegisterHistoricalProvider("historical1", historicalProvider)
	manager.RegisterLegacyProvider("legacy1", legacyProvider)

	providers := manager.ListProviders()

	// 验证所有类型都存在
	if len(providers[TypeRealtimeStock]) != 1 || providers[TypeRealtimeStock][0] != "stock1" {
		t.Errorf("Expected stock provider 'stock1', got %v", providers[TypeRealtimeStock])
	}

	if len(providers[TypeRealtimeIndex]) != 1 || providers[TypeRealtimeIndex][0] != "index1" {
		t.Errorf("Expected index provider 'index1', got %v", providers[TypeRealtimeIndex])
	}

	if len(providers[TypeHistorical]) != 1 || providers[TypeHistorical][0] != "historical1" {
		t.Errorf("Expected historical provider 'historical1', got %v", providers[TypeHistorical])
	}

	if len(providers[TypeLegacy]) != 1 || providers[TypeLegacy][0] != "legacy1" {
		t.Errorf("Expected legacy provider 'legacy1', got %v", providers[TypeLegacy])
	}
}

func TestProviderManager_UnregisterProvider(t *testing.T) {
	manager := NewProviderManager()

	// 注册提供商
	stockProvider := &mockRealtimeStockProvider{name: "to_remove"}
	err := manager.RegisterRealtimeStockProvider("to_remove", stockProvider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证提供商存在
	_, err = manager.GetRealtimeStockProvider("to_remove")
	if err != nil {
		t.Errorf("Expected provider to exist, got %v", err)
	}

	// 注销提供商
	err = manager.UnregisterProvider("to_remove")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证提供商已被移除
	_, err = manager.GetRealtimeStockProvider("to_remove")
	if err == nil {
		t.Error("Expected error for removed provider")
	}

	// 测试注销不存在的提供商
	err = manager.UnregisterProvider("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent provider")
	}

	// 测试空名称
	err = manager.UnregisterProvider("")
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

func TestProviderManager_Close(t *testing.T) {
	manager := NewProviderManager()

	// 注册一些提供商
	stockProvider := &mockRealtimeStockProvider{name: "stock_close"}
	indexProvider := &mockRealtimeIndexProvider{name: "index_close"}
	legacyProvider := &mockLegacyProvider{name: "legacy_close"}

	manager.RegisterRealtimeStockProvider("stock_close", stockProvider)
	manager.RegisterRealtimeIndexProvider("index_close", indexProvider)
	manager.RegisterLegacyProvider("legacy_close", legacyProvider)

	// 关闭管理器
	err := manager.Close()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证所有提供商都已清空
	providers := manager.ListProviders()
	if len(providers) != 0 {
		t.Errorf("Expected empty provider list after close, got %v", providers)
	}
}

// 确保所有接口都正确实现
func TestProviderManagerInterfaceCompliance(t *testing.T) {
	var _ core.RealtimeStockProvider = (*mockRealtimeStockProvider)(nil)
	var _ core.RealtimeIndexProvider = (*mockRealtimeIndexProvider)(nil)
	var _ core.HistoricalProvider = (*mockHistoricalProvider)(nil)
	var _ subscriber.Provider = (*mockLegacyProvider)(nil)
	var _ testkitCore.Provider = (*mockTestKitProvider)(nil)
}
