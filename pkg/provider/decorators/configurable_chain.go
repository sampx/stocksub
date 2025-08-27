package decorators

import (
	"fmt"
	"stocksub/pkg/provider/core"
	"time"

	"github.com/spf13/viper"
)

// DecoratorType 装饰器类型枚举
type DecoratorType string

const (
	FrequencyControlType DecoratorType = "frequency_control"
	CircuitBreakerType   DecoratorType = "circuit_breaker"
)

// ConfigurableDecoratorChain 可配置的装饰器链
type ConfigurableDecoratorChain struct {
	decorators []DecoratorConfig
	factory    *DecoratorFactory
}

// DecoratorConfig 装饰器配置
type DecoratorConfig struct {
	Type     DecoratorType          `yaml:"type" mapstructure:"type"`
	Enabled  bool                   `yaml:"enabled" mapstructure:"enabled"`
	Priority int                    `yaml:"priority" mapstructure:"priority"` // 优先级，数值越小越先应用
	Config   map[string]interface{} `yaml:"config" mapstructure:"config"`
}

// ProviderDecoratorConfig 提供商装饰器完整配置
type ProviderDecoratorConfig struct {
	Decorators []DecoratorConfig `yaml:"decorators" mapstructure:"decorators"`
}

// NewConfigurableDecoratorChain 创建可配置装饰器链
func NewConfigurableDecoratorChain(factory *DecoratorFactory) *ConfigurableDecoratorChain {
	return &ConfigurableDecoratorChain{
		decorators: make([]DecoratorConfig, 0),
		factory:    factory,
	}
}

// LoadFromViper 从 Viper 配置加载装饰器链配置
func (cdc *ConfigurableDecoratorChain) LoadFromViper(v *viper.Viper, configKey string) error {
	var config ProviderDecoratorConfig
	if err := v.UnmarshalKey(configKey, &config); err != nil {
		return fmt.Errorf("无法解析装饰器配置: %w", err)
	}

	cdc.decorators = config.Decorators
	return nil
}

// LoadFromConfig 从配置结构体加载装饰器链配置
func (cdc *ConfigurableDecoratorChain) LoadFromConfig(config ProviderDecoratorConfig) {
	cdc.decorators = config.Decorators
}

// AddDecorator 添加装饰器配置
func (cdc *ConfigurableDecoratorChain) AddDecorator(decoratorConfig DecoratorConfig) {
	cdc.decorators = append(cdc.decorators, decoratorConfig)
}

// Apply 将装饰器链应用到指定的 RealtimeStockProvider
func (cdc *ConfigurableDecoratorChain) Apply(stockProvider core.RealtimeStockProvider) (core.RealtimeStockProvider, error) {
	// 按优先级排序装饰器
	sortedDecorators := cdc.getSortedEnabledDecorators()

	// 逐个应用装饰器
	current := stockProvider
	for _, decoratorConfig := range sortedDecorators {
		decorated, err := cdc.factory.CreateRealtimeStockDecorator(decoratorConfig.Type, current, decoratorConfig.Config)
		if err != nil {
			return nil, fmt.Errorf("无法创建装饰器 %s: %w", decoratorConfig.Type, err)
		}
		current = decorated
	}

	return current, nil
}

// getSortedEnabledDecorators 获取按优先级排序的已启用装饰器
func (cdc *ConfigurableDecoratorChain) getSortedEnabledDecorators() []DecoratorConfig {
	enabled := make([]DecoratorConfig, 0)

	// 过滤出启用的装饰器
	for _, decorator := range cdc.decorators {
		if decorator.Enabled {
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
func (cdc *ConfigurableDecoratorChain) GetAppliedDecorators() []DecoratorType {
	sorted := cdc.getSortedEnabledDecorators()
	types := make([]DecoratorType, len(sorted))
	for i, decorator := range sorted {
		types[i] = decorator.Type
	}
	return types
}

// 增强的装饰器工厂，支持配置驱动创建
func (df *DecoratorFactory) CreateRealtimeStockDecorator(decoratorType DecoratorType, stockProvider core.RealtimeStockProvider, config map[string]interface{}) (core.RealtimeStockProvider, error) {
	switch decoratorType {
	case FrequencyControlType:
		return df.createFrequencyControlProvider(stockProvider, config)
	case CircuitBreakerType:
		return df.createCircuitBreakerProvider(stockProvider, config)
	default:
		return nil, fmt.Errorf("不支持的装饰器类型: %s", decoratorType)
	}
}

// createFrequencyControlProvider 创建频率控制装饰器
func (df *DecoratorFactory) createFrequencyControlProvider(stockProvider core.RealtimeStockProvider, configMap map[string]interface{}) (core.RealtimeStockProvider, error) {
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

	return NewFrequencyControlProvider(stockProvider, config), nil
}

// createCircuitBreakerProvider 创建熔断器装饰器
func (df *DecoratorFactory) createCircuitBreakerProvider(stockProvider core.RealtimeStockProvider, configMap map[string]interface{}) (core.RealtimeStockProvider, error) {
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

	return NewCircuitBreakerProvider(stockProvider, config), nil
}

// CreateDecoratedProvider 便捷方法：使用配置创建完全装饰的提供商
func CreateDecoratedProvider(stockProvider core.RealtimeStockProvider, config ProviderDecoratorConfig) (core.RealtimeStockProvider, error) {
	factory := NewDecoratorFactory()
	chain := NewConfigurableDecoratorChain(factory)
	chain.LoadFromConfig(config)
	return chain.Apply(stockProvider)
}

// CreateDecoratedProviderFromViper 便捷方法：从 Viper 配置创建完全装饰的提供商
func CreateDecoratedProviderFromViper(stockProvider core.RealtimeStockProvider, v *viper.Viper, configKey string) (core.RealtimeStockProvider, error) {
	factory := NewDecoratorFactory()
	chain := NewConfigurableDecoratorChain(factory)

	if err := chain.LoadFromViper(v, configKey); err != nil {
		return nil, err
	}

	return chain.Apply(stockProvider)
}

// DefaultDecoratorConfig 创建默认的装饰器配置
func DefaultDecoratorConfig() ProviderDecoratorConfig {
	return ProviderDecoratorConfig{
		Decorators: []DecoratorConfig{
			{
				Type:     FrequencyControlType,
				Enabled:  true,
				Priority: 1, // 频率控制优先级高，先应用
				Config: map[string]interface{}{
					"min_interval_ms": 200,
					"max_retries":     3,
					"enabled":         true,
				},
			},
			{
				Type:     CircuitBreakerType,
				Enabled:  true,
				Priority: 2, // 熔断器优先级低，后应用
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
	}
}

// ProductionDecoratorConfig 生产环境装饰器配置
func ProductionDecoratorConfig() ProviderDecoratorConfig {
	return ProviderDecoratorConfig{
		Decorators: []DecoratorConfig{
			{
				Type:     FrequencyControlType,
				Enabled:  true,
				Priority: 1,
				Config: map[string]interface{}{
					"min_interval_ms": 5000, // 生产环境更保守的频率限制
					"max_retries":     3,
					"enabled":         true,
				},
			},
			{
				Type:     CircuitBreakerType,
				Enabled:  true,
				Priority: 2,
				Config: map[string]interface{}{
					"name":          "ProductionStockProvider",
					"max_requests":  3,
					"interval":      "120s",
					"timeout":       "60s",
					"ready_to_trip": 3, // 更敏感的熔断策略
					"enabled":       true,
				},
			},
		},
	}
}

// TestDecoratorConfig 测试环境装饰器配置
func TestDecoratorConfig() ProviderDecoratorConfig {
	return ProviderDecoratorConfig{
		Decorators: []DecoratorConfig{
			{
				Type:     FrequencyControlType,
				Enabled:  false, // 测试环境可以关闭频率限制
				Priority: 1,
				Config: map[string]interface{}{
					"enabled": false,
				},
			},
			{
				Type:     CircuitBreakerType,
				Enabled:  false, // 测试环境关闭熔断器
				Priority: 2,
				Config: map[string]interface{}{
					"enabled": false,
				},
			},
		},
	}
}

// MonitoringDecoratorConfig 监控环境装饰器配置
// 为长期监控 (api_monitor) 量身定制的配置
func MonitoringDecoratorConfig() ProviderDecoratorConfig {
	return ProviderDecoratorConfig{
		Decorators: []DecoratorConfig{
			{
				Type:     FrequencyControlType,
				Enabled:  true,
				Priority: 1,
				Config: map[string]interface{}{
					"min_interval_ms": 3000, // 3秒间隔，适合长期监控
					"max_retries":     5,    // 更多重试次数
					"enabled":         true,
				},
			},
			{
				Type:     CircuitBreakerType,
				Enabled:  true,
				Priority: 2,
				Config: map[string]interface{}{
					"name":          "LongTermMonitoringProvider",
					"max_requests":  10,     // 更宽松的熔断策略
					"interval":      "300s", // 5分钟统计窗口
					"timeout":       "120s", // 2分钟超时
					"ready_to_trip": 10,     // 更高的阈值
					"enabled":       true,
				},
			},
		},
	}
}
