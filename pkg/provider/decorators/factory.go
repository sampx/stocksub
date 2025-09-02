package decorators

import (
	"fmt"
	"stocksub/pkg/provider"
	"time"

	"github.com/spf13/viper"
)

// DecoratorFactory 装饰器工厂
// 提供统一的装饰器创建和管理功能
type DecoratorFactory struct{}

// NewDecoratorFactory 创建装饰器工厂
func NewDecoratorFactory() *DecoratorFactory {
	return &DecoratorFactory{}
}

// CreateDecorator 根据类型和配置创建装饰器
func (f *DecoratorFactory) CreateDecorator(decoratorType provider.DecoratorType, baseProvider provider.Provider, config map[string]interface{}) (provider.Provider, error) {
	switch decoratorType {
	case provider.FrequencyControlType:
		return f.createFrequencyControlProvider(baseProvider, config)
	case provider.CircuitBreakerType:
		return f.createCircuitBreakerProvider(baseProvider, config)
	case provider.SimplifiedFrequencyControlType:
		return f.createSimplifiedFrequencyControlProvider(baseProvider, config)
	case provider.SimplifiedCircuitBreakerType:
		return f.createSimplifiedCircuitBreakerProvider(baseProvider, config)
	default:
		return nil, fmt.Errorf("不支持的装饰器类型: %s", decoratorType)
	}
}

// CreateDecoratorChain 根据配置创建装饰器链
func (f *DecoratorFactory) CreateDecoratorChain(baseProvider provider.Provider, configs []provider.DecoratorConfig) (provider.Provider, error) {
	current := baseProvider

	// 按优先级排序配置
	sortedConfigs := f.sortConfigsByPriority(configs)

	// 逐个应用装饰器
	for _, config := range sortedConfigs {
		if !config.Enabled {
			continue
		}

		decorated, err := f.CreateDecorator(config.Type, current, config.Config)
		if err != nil {
			return nil, fmt.Errorf("创建装饰器 %s 失败: %w", config.Type, err)
		}
		current = decorated
	}

	return current, nil
}

// CreateFromViper 从 Viper 配置创建装饰器链
func (f *DecoratorFactory) CreateFromViper(baseProvider provider.Provider, v *viper.Viper, configKey string) (provider.Provider, error) {
	var config provider.ProviderDecoratorConfig
	if err := v.UnmarshalKey(configKey, &config); err != nil {
		return nil, fmt.Errorf("解析装饰器配置失败: %w", err)
	}

	return f.CreateFromConfig(baseProvider, config)
}

// CreateFromConfig 从配置结构体创建装饰器链
func (f *DecoratorFactory) CreateFromConfig(baseProvider provider.Provider, config provider.ProviderDecoratorConfig) (provider.Provider, error) {
	// 合并所有配置
	var allConfigs []provider.DecoratorConfig
	allConfigs = append(allConfigs, config.All...)
	allConfigs = append(allConfigs, config.Realtime...)
	allConfigs = append(allConfigs, config.Historical...)
	allConfigs = append(allConfigs, config.Index...)

	return f.CreateDecoratorChain(baseProvider, allConfigs)
}

// CreateSimplifiedDecorators 创建简化装饰器组合
func (f *DecoratorFactory) CreateSimplifiedDecorators(baseProvider provider.RealtimeStockProvider, config *SimplifiedDecoratorConfig) (provider.Provider, error) {
	if config == nil {
		config = NewSimplifiedDecoratorConfig()
	}

	current := baseProvider

	// 应用频率控制装饰器
	if config.Enabled && config.FrequencyControl != nil && config.FrequencyControl.Enabled {
		decorated, err := f.createSimplifiedFrequencyControlProvider(current, nil)
		if err != nil {
			return nil, fmt.Errorf("创建简化频率控制装饰器失败: %w", err)
		}
		current = decorated.(provider.RealtimeStockProvider)
	}

	// 应用熔断器装饰器
	if config.Enabled && config.CircuitBreaker != nil && config.CircuitBreaker.Enabled {
		decorated, err := f.createSimplifiedCircuitBreakerProvider(current, nil)
		if err != nil {
			return nil, fmt.Errorf("创建简化熔断器装饰器失败: %w", err)
		}
		current = decorated.(provider.RealtimeStockProvider)
	}

	return current, nil
}

// 私有方法：创建各种装饰器

func (f *DecoratorFactory) createFrequencyControlProvider(prov provider.Provider, configMap map[string]interface{}) (provider.Provider, error) {
	config := &FrequencyControlConfig{
		MinInterval: 200 * time.Millisecond,
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
		return NewFrequencyControlForHistoricalProvider(p, config), nil
	default:
		return nil, fmt.Errorf("不支持为类型 %T 应用频率控制装饰器", p)
	}
}

func (f *DecoratorFactory) createCircuitBreakerProvider(prov provider.Provider, configMap map[string]interface{}) (provider.Provider, error) {
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

func (f *DecoratorFactory) createSimplifiedFrequencyControlProvider(prov provider.Provider, configMap map[string]interface{}) (provider.Provider, error) {
	config := DefaultSimplifiedFrequencyControlConfig()

	// 解析配置
	if configMap != nil {
		if minInterval, ok := configMap["min_interval"].(string); ok {
			if duration, err := time.ParseDuration(minInterval); err == nil {
				config.MinInterval = duration
			}
		}
		if maxRetries, ok := configMap["max_retries"].(int); ok {
			config.MaxRetries = maxRetries
		}
		if enabled, ok := configMap["enabled"].(bool); ok {
			config.Enabled = enabled
		}
		if marketTimeAware, ok := configMap["market_time_aware"].(bool); ok {
			config.MarketTimeAware = marketTimeAware
		}
		if preMarketBuffer, ok := configMap["pre_market_buffer"].(string); ok {
			if duration, err := time.ParseDuration(preMarketBuffer); err == nil {
				config.PreMarketBuffer = duration
			}
		}
		if postMarketBuffer, ok := configMap["post_market_buffer"].(string); ok {
			if duration, err := time.ParseDuration(postMarketBuffer); err == nil {
				config.PostMarketBuffer = duration
			}
		}
		if ipBanRetryInterval, ok := configMap["ip_ban_retry_interval"].(string); ok {
			if duration, err := time.ParseDuration(ipBanRetryInterval); err == nil {
				config.IPBanRetryInterval = duration
			}
		}
		if ipBanRetryMax, ok := configMap["ip_ban_retry_max"].(int); ok {
			config.IPBanRetryMax = ipBanRetryMax
		}
	}

	switch p := prov.(type) {
	case provider.RealtimeStockProvider:
		return NewSimplifiedFrequencyControlProvider(p, config), nil
	default:
		return nil, fmt.Errorf("不支持为类型 %T 应用简化频率控制装饰器", p)
	}
}

func (f *DecoratorFactory) createSimplifiedCircuitBreakerProvider(prov provider.Provider, configMap map[string]interface{}) (provider.Provider, error) {
	config := DefaultSimplifiedCircuitBreakerConfig()

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
		if marketTimeAware, ok := configMap["market_time_aware"].(bool); ok {
			config.MarketTimeAware = marketTimeAware
		}
	}

	switch p := prov.(type) {
	case provider.RealtimeStockProvider:
		return NewSimplifiedCircuitBreakerProvider(p, config), nil
	default:
		return nil, fmt.Errorf("不支持为类型 %T 应用简化熔断器装饰器", p)
	}
}

// 辅助方法

func (f *DecoratorFactory) sortConfigsByPriority(configs []provider.DecoratorConfig) []provider.DecoratorConfig {
	if len(configs) <= 1 {
		return configs
	}

	// 简单的冒泡排序，按优先级升序（数值越小优先级越高）
	sorted := make([]provider.DecoratorConfig, len(configs))
	copy(sorted, configs)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Priority > sorted[j].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// GetSupportedDecoratorTypes 获取支持的装饰器类型列表
func (f *DecoratorFactory) GetSupportedDecoratorTypes() []provider.DecoratorType {
	return []provider.DecoratorType{
		provider.FrequencyControlType,
		provider.CircuitBreakerType,
		provider.SimplifiedFrequencyControlType,
		provider.SimplifiedCircuitBreakerType,
	}
}

// ValidateConfig 验证装饰器配置
func (f *DecoratorFactory) ValidateConfig(config provider.DecoratorConfig) error {
	switch config.Type {
	case provider.FrequencyControlType, provider.CircuitBreakerType,
		provider.SimplifiedFrequencyControlType, provider.SimplifiedCircuitBreakerType:
		return nil
	default:
		return fmt.Errorf("不支持的装饰器类型: %s", config.Type)
	}
}

// GetDefaultConfig 获取指定类型的默认配置
func (f *DecoratorFactory) GetDefaultConfig(decoratorType provider.DecoratorType) (map[string]interface{}, error) {
	switch decoratorType {
	case provider.FrequencyControlType:
		return map[string]interface{}{
			"min_interval_ms": 200,
			"max_retries":     3,
			"enabled":         true,
		}, nil
	case provider.CircuitBreakerType:
		return map[string]interface{}{
			"name":          "StockProvider",
			"max_requests":  5,
			"interval":      "60s",
			"timeout":       "30s",
			"ready_to_trip": 5,
			"enabled":       true,
		}, nil
	case provider.SimplifiedFrequencyControlType:
		return map[string]interface{}{
			"min_interval":          "200ms",
			"max_retries":           3,
			"enabled":               true,
			"market_time_aware":     true,
			"pre_market_buffer":     "5m",
			"post_market_buffer":    "10m",
			"ip_ban_retry_interval": "5m",
			"ip_ban_retry_max":      3,
		}, nil
	case provider.SimplifiedCircuitBreakerType:
		return map[string]interface{}{
			"name":              "SimplifiedStockProvider",
			"max_requests":      5,
			"interval":          "60s",
			"timeout":           "30s",
			"ready_to_trip":     5,
			"enabled":           true,
			"market_time_aware": true,
		}, nil
	default:
		return nil, fmt.Errorf("不支持的装饰器类型: %s", decoratorType)
	}
}
