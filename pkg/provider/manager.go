package provider

import (
	"fmt"
	"stocksub/pkg/provider/core"
	"stocksub/pkg/subscriber"
	"sync"
)

// ProviderType 提供商类型
type ProviderType string

const (
	// TypeRealtimeStock 实时股票数据提供商
	TypeRealtimeStock ProviderType = "realtime_stock"
	// TypeRealtimeIndex 实时指数数据提供商
	TypeRealtimeIndex ProviderType = "realtime_index"
	// TypeHistorical 历史数据提供商
	TypeHistorical ProviderType = "historical"
	// TypeLegacy 旧版兼容提供商
	TypeLegacy ProviderType = "legacy"
)

// ProviderManager 提供商管理器
// 支持新旧接口的并存，提供统一的访问接口
type ProviderManager struct {
	// 新接口提供商
	realtimeStockProviders map[string]core.RealtimeStockProvider
	realtimeIndexProviders map[string]core.RealtimeIndexProvider
	historicalProviders    map[string]core.HistoricalProvider

	// 旧接口提供商（为了向后兼容）
	legacyProviders map[string]subscriber.Provider

	mu sync.RWMutex
}

// NewProviderManager 创建新的提供商管理器
func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		realtimeStockProviders: make(map[string]core.RealtimeStockProvider),
		realtimeIndexProviders: make(map[string]core.RealtimeIndexProvider),
		historicalProviders:    make(map[string]core.HistoricalProvider),
		legacyProviders:        make(map[string]subscriber.Provider),
	}
}

// RegisterRealtimeStockProvider 注册实时股票数据提供商
func (m *ProviderManager) RegisterRealtimeStockProvider(name string, provider core.RealtimeStockProvider) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.realtimeStockProviders[name] = provider
	return nil
}

// RegisterRealtimeIndexProvider 注册实时指数数据提供商
func (m *ProviderManager) RegisterRealtimeIndexProvider(name string, provider core.RealtimeIndexProvider) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.realtimeIndexProviders[name] = provider
	return nil
}

// RegisterHistoricalProvider 注册历史数据提供商
func (m *ProviderManager) RegisterHistoricalProvider(name string, provider core.HistoricalProvider) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.historicalProviders[name] = provider
	return nil
}

// RegisterLegacyProvider 注册旧版提供商（向后兼容）
func (m *ProviderManager) RegisterLegacyProvider(name string, provider subscriber.Provider) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.legacyProviders[name] = provider
	return nil
}

// RegisterProvider 智能注册提供商
// 自动检测提供商类型并注册到相应的类别
func (m *ProviderManager) RegisterProvider(name string, provider interface{}) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	// 尝试注册为新接口类型
	if stockProvider, ok := provider.(core.RealtimeStockProvider); ok {
		return m.RegisterRealtimeStockProvider(name, stockProvider)
	}

	if indexProvider, ok := provider.(core.RealtimeIndexProvider); ok {
		return m.RegisterRealtimeIndexProvider(name, indexProvider)
	}

	if historicalProvider, ok := provider.(core.HistoricalProvider); ok {
		return m.RegisterHistoricalProvider(name, historicalProvider)
	}

	// 兼容旧版接口
	if legacyProvider, ok := provider.(subscriber.Provider); ok {
		return m.RegisterLegacyProvider(name, legacyProvider)
	}

	// 使用智能适配器处理其他类型（包括 testkit Provider）
	smartAdapter := core.NewSmartProviderAdapter(provider)
	if smartAdapter.IsSupported() {
		return m.RegisterRealtimeStockProvider(name, smartAdapter)
	}

	return fmt.Errorf("unsupported provider type: %T", provider)
}

// GetRealtimeStockProvider 获取实时股票数据提供商
func (m *ProviderManager) GetRealtimeStockProvider(name string) (core.RealtimeStockProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 先检查新接口提供商
	if provider, exists := m.realtimeStockProviders[name]; exists {
		return provider, nil
	}

	// 检查旧接口提供商，如果存在则使用适配器
	if legacyProvider, exists := m.legacyProviders[name]; exists {
		return core.NewLegacyProviderAdapter(legacyProvider), nil
	}

	return nil, fmt.Errorf("realtime stock provider '%s' not found", name)
}

// GetRealtimeIndexProvider 获取实时指数数据提供商
func (m *ProviderManager) GetRealtimeIndexProvider(name string) (core.RealtimeIndexProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if provider, exists := m.realtimeIndexProviders[name]; exists {
		return provider, nil
	}

	return nil, fmt.Errorf("realtime index provider '%s' not found", name)
}

// GetHistoricalProvider 获取历史数据提供商
func (m *ProviderManager) GetHistoricalProvider(name string) (core.HistoricalProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if provider, exists := m.historicalProviders[name]; exists {
		return provider, nil
	}

	return nil, fmt.Errorf("historical provider '%s' not found", name)
}

// GetLegacyProvider 获取旧版提供商（向后兼容）
func (m *ProviderManager) GetLegacyProvider(name string) (subscriber.Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if provider, exists := m.legacyProviders[name]; exists {
		return provider, nil
	}

	return nil, fmt.Errorf("legacy provider '%s' not found", name)
}

// ListProviders 列出所有已注册的提供商
func (m *ProviderManager) ListProviders() map[ProviderType][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[ProviderType][]string)

	// 实时股票提供商
	var realtimeStockNames []string
	for name := range m.realtimeStockProviders {
		realtimeStockNames = append(realtimeStockNames, name)
	}
	if len(realtimeStockNames) > 0 {
		result[TypeRealtimeStock] = realtimeStockNames
	}

	// 实时指数提供商
	var realtimeIndexNames []string
	for name := range m.realtimeIndexProviders {
		realtimeIndexNames = append(realtimeIndexNames, name)
	}
	if len(realtimeIndexNames) > 0 {
		result[TypeRealtimeIndex] = realtimeIndexNames
	}

	// 历史数据提供商
	var historicalNames []string
	for name := range m.historicalProviders {
		historicalNames = append(historicalNames, name)
	}
	if len(historicalNames) > 0 {
		result[TypeHistorical] = historicalNames
	}

	// 旧版提供商
	var legacyNames []string
	for name := range m.legacyProviders {
		legacyNames = append(legacyNames, name)
	}
	if len(legacyNames) > 0 {
		result[TypeLegacy] = legacyNames
	}

	return result
}

// UnregisterProvider 注销提供商
func (m *ProviderManager) UnregisterProvider(name string) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	found := false

	// 从各个类别中移除
	if _, exists := m.realtimeStockProviders[name]; exists {
		delete(m.realtimeStockProviders, name)
		found = true
	}

	if _, exists := m.realtimeIndexProviders[name]; exists {
		delete(m.realtimeIndexProviders, name)
		found = true
	}

	if _, exists := m.historicalProviders[name]; exists {
		delete(m.historicalProviders, name)
		found = true
	}

	if _, exists := m.legacyProviders[name]; exists {
		delete(m.legacyProviders, name)
		found = true
	}

	if !found {
		return fmt.Errorf("provider '%s' not found", name)
	}

	return nil
}

// Close 关闭管理器，清理所有提供商资源
func (m *ProviderManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	// 关闭所有支持关闭的提供商
	for name, provider := range m.realtimeStockProviders {
		if closable, ok := provider.(core.Closable); ok {
			if err := closable.Close(); err != nil {
				errors = append(errors, fmt.Errorf("error closing realtime stock provider '%s': %w", name, err))
			}
		}
	}

	for name, provider := range m.realtimeIndexProviders {
		if closable, ok := provider.(core.Closable); ok {
			if err := closable.Close(); err != nil {
				errors = append(errors, fmt.Errorf("error closing realtime index provider '%s': %w", name, err))
			}
		}
	}

	for name, provider := range m.historicalProviders {
		if closable, ok := provider.(core.Closable); ok {
			if err := closable.Close(); err != nil {
				errors = append(errors, fmt.Errorf("error closing historical provider '%s': %w", name, err))
			}
		}
	}

	// 清空所有映射
	m.realtimeStockProviders = make(map[string]core.RealtimeStockProvider)
	m.realtimeIndexProviders = make(map[string]core.RealtimeIndexProvider)
	m.historicalProviders = make(map[string]core.HistoricalProvider)
	m.legacyProviders = make(map[string]subscriber.Provider)

	if len(errors) > 0 {
		return fmt.Errorf("errors occurred while closing providers: %v", errors)
	}

	return nil
}
