package decorators

import (
	"fmt"
	"stocksub/pkg/provider"
	"time"

	"github.com/spf13/viper"
)

// ConfigurableDecoratorChain 可配置的装饰器链
type ConfigurableDecoratorChain struct {
	decorators []provider.DecoratorConfig
}

// NewConfigurableDecoratorChain 创建可配置装饰器链
func NewConfigurableDecoratorChain() *ConfigurableDecoratorChain {
	return &ConfigurableDecoratorChain{}
}

// LoadFromViper 从 Viper 配置加载装饰器链配置
func (cdc *ConfigurableDecoratorChain) LoadFromViper(v *viper.Viper, configKey string) error {
	var config provider.ProviderDecoratorConfig
	if err := v.UnmarshalKey(configKey, &config); err != nil {
		return fmt.Errorf("无法解析装饰器配置: %w", err)
	}

	cdc.LoadFromConfig(config)
	return nil
}

// LoadFromConfig 从配置结构体加载装饰器链配置
func (cdc *ConfigurableDecoratorChain) LoadFromConfig(config provider.ProviderDecoratorConfig) {
	cdc.decorators = append(cdc.decorators, config.All...)
	cdc.decorators = append(cdc.decorators, config.Realtime...)
	cdc.decorators = append(cdc.decorators, config.Historical...)
	cdc.decorators = append(cdc.decorators, config.Index...)
}

// AddDecorator 添加装饰器配置
func (cdc *ConfigurableDecoratorChain) AddDecorator(decoratorConfig provider.DecoratorConfig) {
	cdc.decorators = append(cdc.decorators, decoratorConfig)
}

// Apply 将装饰器链应用到指定的 RealtimeStockProvider
func (cdc *ConfigurableDecoratorChain) Apply(p provider.Provider) (provider.Provider, error) {
	// 按优先级排序装饰器
	sortedDecorators := cdc.getSortedEnabledDecorators(p)

	// 逐个应用装饰器
	current := p
	for _, decoratorConfig := range sortedDecorators {
		decorated, err := CreateDecorator(decoratorConfig.Type, current, decoratorConfig.Config)
		if err != nil {
			return nil, fmt.Errorf("无法创建装饰器 %s: %w", decoratorConfig.Type, err)
		}
		current = decorated
	}

	return current, nil
}

// getSortedEnabledDecorators 获取按优先级排序的已启用装饰器
func (cdc *ConfigurableDecoratorChain) getSortedEnabledDecorators(p provider.Provider) []provider.DecoratorConfig {
	enabled := make([]provider.DecoratorConfig, 0)
	var providerType string

	switch p.(type) {
	case provider.RealtimeStockProvider:
		providerType = "realtime"
	case provider.HistoricalProvider:
		providerType = "historical"
	case provider.RealtimeIndexProvider:
		providerType = "index"
	}

	// 过滤出启用的装饰器
	for _, decorator := range cdc.decorators {
		if decorator.Enabled && (decorator.ProviderType == "all" || decorator.ProviderType == "" || decorator.ProviderType == providerType) {
			enabled = append(enabled, decorator)
		}
	}

	// 按优先级排序（数值越小优先级越高）
	for i := 0; i < len(enabled)-1; i++ {
		for j := i + 1; j < len(enabled); j++ {
			if enabled[i].Priority > enabled[j].Priority {
				enabled[i], enabled[j] = enabled[j], enabled[i]
			}
		}
	}

	return enabled
}

// GetAppliedDecorators 获取将要应用的装饰器列表
func (cdc *ConfigurableDecoratorChain) GetAppliedDecorators(p provider.Provider) []provider.DecoratorType {
	sorted := cdc.getSortedEnabledDecorators(p)
	types := make([]provider.DecoratorType, len(sorted))
	for i, decorator := range sorted {
		types[i] = decorator.Type
	}
	return types
}

// CreateDecorator 支持配置驱动创建
func CreateDecorator(decoratorType provider.DecoratorType, p provider.Provider, config map[string]interface{}) (provider.Provider, error) {
	switch decoratorType {
	case provider.FrequencyControlType:
		return createFrequencyControlProvider(p, config)
	case provider.CircuitBreakerType:
		return createCircuitBreakerProvider(p, config)
	default:
		return nil, fmt.Errorf("不支持的装饰器类型: %s", decoratorType)
	}
}

// createFrequencyControlProvider 创建频率控制装饰器
func createFrequencyControlProvider(prov provider.Provider, configMap map[string]interface{}) (provider.Provider, error) {
	config := &FrequencyControlConfig{
		MinInterval: 200 * time.Millisecond, // 默认值
		MaxRetries:  3,
		Enabled:     true,
	}

	// 解析配置
	if configMap != nil {
		if minInterval, ok := configMap["min_interval"].(string); ok {
			if duration, err := time.ParseDuration(minInterval); err == nil {
				config.MinInterval = duration
			}
		}
		if minIntervalMs, ok := configMap["min_interval_ms"].(int); ok {
			config.MinInterval = time.Duration(minIntervalMs) * time.Millisecond
		}
		if maxRetries, ok := configMap["max_retries"].(int); ok {
			config.MaxRetries = maxRetries
		}
		if enabled, ok := configMap["enabled"].(bool); ok {
			config.Enabled = enabled
		}
	}
	switch p := prov.(type) {
	case provider.RealtimeStockProvider:
		return NewFrequencyControlProvider(p, config), nil
	case provider.HistoricalProvider:
		// 历史数据提供商可能不需要频率控制，或者有不同的实现
		return NewFrequencyControlForHistoricalProvider(p, config), nil
	default:
		return nil, fmt.Errorf("不支持为类型 %T 应用频率控制装饰器", p)
	}
}

// createCircuitBreakerProvider 创建熔断器装饰器
func createCircuitBreakerProvider(prov provider.Provider, configMap map[string]interface{}) (provider.Provider, error) {
	config := DefaultCircuitBreakerConfig()

	// 解析配置
	if configMap != nil {
		if name, ok := configMap["name"].(string); ok {
			config.Name = name
		}
		if maxRequests, ok := configMap["max_requests"].(int); ok {
			config.MaxRequests = uint32(maxRequests)
		}
		if interval, ok := configMap["interval"].(string); ok {
			if duration, err := time.ParseDuration(interval); err == nil {
				config.Interval = duration
			}
		}
		if timeout, ok := configMap["timeout"].(string); ok {
			if duration, err := time.ParseDuration(timeout); err == nil {
				config.Timeout = duration
			}
		}
		if readyToTrip, ok := configMap["ready_to_trip"].(int); ok {
			config.ReadyToTrip = uint32(readyToTrip)
		}
		if enabled, ok := configMap["enabled"].(bool); ok {
			config.Enabled = enabled
		}
	}
	switch p := prov.(type) {
	case provider.RealtimeStockProvider:
		return NewCircuitBreakerProvider(p, config), nil
	case provider.HistoricalProvider:
		return NewCircuitBreakerForHistoricalProvider(p, config), nil
	default:
		return nil, fmt.Errorf("不支持为类型 %T 应用熔断器装饰器", p)
	}
}

// CreateDecoratedProvider 便捷方法：使用配置创建完全装饰的提供商
func CreateDecoratedProvider(stockProvider provider.Provider, config provider.ProviderDecoratorConfig) (provider.Provider, error) {
	chain := NewConfigurableDecoratorChain()
	chain.LoadFromConfig(config)
	return chain.Apply(stockProvider)
}

// CreateDecoratedProviderFromViper 便捷方法：从 Viper 配置创建完全装饰的提供商
func CreateDecoratedProviderFromViper(stockProvider provider.Provider, v *viper.Viper, configKey string) (provider.Provider, error) {
	chain := NewConfigurableDecoratorChain()

	if err := chain.LoadFromViper(v, configKey); err != nil {
		return nil, err
	}

	return chain.Apply(stockProvider)
}

// DefaultDecoratorConfig 创建默认的装饰器配置
func DefaultDecoratorConfig() provider.ProviderDecoratorConfig {
	return provider.ProviderDecoratorConfig{
		All: []provider.DecoratorConfig{
			{
				Type:         provider.CircuitBreakerType,
				Enabled:      true,
				Priority:     2,
				ProviderType: "all",
				Config: map[string]interface{}{
					"name":          "StockProvider",
					"max_requests":  5,
					"interval":      "60s",
					"timeout":       "30s",
					"ready_to_trip": 5,
					"enabled":       true,
				},
			},
		},
		Realtime: []provider.DecoratorConfig{
			{
				Type:         provider.FrequencyControlType,
				Enabled:      true,
				Priority:     1,
				ProviderType: "realtime",
				Config: map[string]interface{}{
					"min_interval_ms": 200,
					"max_retries":     3,
					"enabled":         true,
				},
			},
		},
	}
}

// ProductionDecoratorConfig 生产环境装饰器配置
func ProductionDecoratorConfig() provider.ProviderDecoratorConfig {
	return provider.ProviderDecoratorConfig{
		All: []provider.DecoratorConfig{
			{
				Type:         provider.CircuitBreakerType,
				Enabled:      true,
				Priority:     2,
				ProviderType: "all",
				Config: map[string]interface{}{
					"name":          "ProductionStockProvider",
					"max_requests":  3,
					"interval":      "120s",
					"timeout":       "60s",
					"ready_to_trip": 3,
					"enabled":       true,
				},
			},
		},
		Realtime: []provider.DecoratorConfig{
			{
				Type:         provider.FrequencyControlType,
				Enabled:      true,
				Priority:     1,
				ProviderType: "realtime",
				Config: map[string]interface{}{
					"min_interval_ms": 5000,
					"max_retries":     3,
					"enabled":         true,
				},
			},
		},
		Historical: []provider.DecoratorConfig{
			{
				Type:         provider.FrequencyControlType,
				Enabled:      true,
				Priority:     1,
				ProviderType: "historical",
				Config: map[string]interface{}{
					"min_interval_ms": 10000, // 历史数据使用更长的间隔
					"max_retries":     5,
					"enabled":         true,
				},
			},
		},
	}
}

// TestDecoratorConfig 测试环境装饰器配置
func TestDecoratorConfig() provider.ProviderDecoratorConfig {
	return provider.ProviderDecoratorConfig{
		All: []provider.DecoratorConfig{
			{
				Type:         provider.FrequencyControlType,
				Enabled:      false,
				ProviderType: "all",
			},
			{
				Type:         provider.CircuitBreakerType,
				Enabled:      false,
				ProviderType: "all",
			},
		},
	}
}

// MonitoringDecoratorConfig 监控环境装饰器配置
// 为长期监控 (api_monitor) 量身定制的配置
func MonitoringDecoratorConfig() provider.ProviderDecoratorConfig {
	return provider.ProviderDecoratorConfig{
		All: []provider.DecoratorConfig{
			{
				Type:         provider.CircuitBreakerType,
				Enabled:      true,
				Priority:     2,
				ProviderType: "all",
				Config: map[string]interface{}{
					"name":          "LongTermMonitoringProvider",
					"max_requests":  10,
					"interval":      "300s",
					"timeout":       "120s",
					"ready_to_trip": 10,
					"enabled":       true,
				},
			},
		},
		Realtime: []provider.DecoratorConfig{
			{
				Type:         provider.FrequencyControlType,
				Enabled:      true,
				Priority:     1,
				ProviderType: "realtime",
				Config: map[string]interface{}{
					"min_interval_ms": 3000,
					"max_retries":     5,
					"enabled":         true,
				},
			},
		},
	}
}
