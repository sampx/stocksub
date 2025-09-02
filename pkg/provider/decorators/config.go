package decorators

import (
	"fmt"
	"stocksub/pkg/provider"
	"time"

	"github.com/spf13/viper"
)

// 简化装饰器类型常量
const (
	SimplifiedFrequencyControlType provider.DecoratorType = "simplified_frequency_control"
	SimplifiedCircuitBreakerType   provider.DecoratorType = "simplified_circuit_breaker"
)

// SimplifiedDecoratorConfig 简化装饰器配置管理器
type SimplifiedDecoratorConfig struct {
	// 频率控制配置
	FrequencyControl *SimplifiedFrequencyControlConfig `yaml:"frequency_control" mapstructure:"frequency_control"`

	// 熔断器配置
	CircuitBreaker *SimplifiedCircuitBreakerConfig `yaml:"circuit_breaker" mapstructure:"circuit_breaker"`

	// 通用配置
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
}

// NewSimplifiedDecoratorConfig 创建默认的简化装饰器配置
func NewSimplifiedDecoratorConfig() *SimplifiedDecoratorConfig {
	return &SimplifiedDecoratorConfig{
		FrequencyControl: DefaultSimplifiedFrequencyControlConfig(),
		CircuitBreaker:   DefaultSimplifiedCircuitBreakerConfig(),
		Enabled:          true,
	}
}

// LoadFromViper 从 Viper 配置加载简化装饰器配置
func (sdc *SimplifiedDecoratorConfig) LoadFromViper(v *viper.Viper, configKey string) error {
	if err := v.UnmarshalKey(configKey, sdc); err != nil {
		return fmt.Errorf("无法解析简化装饰器配置: %w", err)
	}

	// 验证配置
	if err := sdc.Validate(); err != nil {
		return fmt.Errorf("简化装饰器配置验证失败: %w", err)
	}

	return nil
}

// Validate 验证配置的有效性
func (sdc *SimplifiedDecoratorConfig) Validate() error {
	if sdc.FrequencyControl != nil {
		if err := sdc.FrequencyControl.Validate(); err != nil {
			return fmt.Errorf("频率控制配置无效: %w", err)
		}
	}

	if sdc.CircuitBreaker != nil {
		if err := sdc.CircuitBreaker.Validate(); err != nil {
			return fmt.Errorf("熔断器配置无效: %w", err)
		}
	}

	return nil
}

// Validate 验证简化频率控制配置
func (sfc *SimplifiedFrequencyControlConfig) Validate() error {
	if sfc.MinInterval <= 0 {
		return fmt.Errorf("最小请求间隔必须大于0")
	}

	if sfc.MaxRetries < 0 {
		return fmt.Errorf("最大重试次数不能为负数")
	}

	if sfc.IPBanRetryMax < 0 {
		return fmt.Errorf("IP封禁最大重试次数不能为负数")
	}

	if sfc.IPBanRetryInterval <= 0 {
		return fmt.Errorf("IP封禁重试间隔必须大于0")
	}

	if sfc.PreMarketBuffer < 0 {
		return fmt.Errorf("交易前缓冲时间不能为负数")
	}

	if sfc.PostMarketBuffer < 0 {
		return fmt.Errorf("交易后缓冲时间不能为负数")
	}

	return nil
}

// Validate 验证简化熔断器配置
func (scb *SimplifiedCircuitBreakerConfig) Validate() error {
	if scb.MaxRequests == 0 {
		return fmt.Errorf("半开状态最大请求数不能为0")
	}

	if scb.Interval <= 0 {
		return fmt.Errorf("统计窗口时间必须大于0")
	}

	if scb.Timeout <= 0 {
		return fmt.Errorf("熔断器超时时间必须大于0")
	}

	if scb.ReadyToTrip == 0 {
		return fmt.Errorf("触发熔断的失败次数不能为0")
	}

	return nil
}

// ToDecoratorConfigs 将简化装饰器配置转换为标准装饰器配置
func (sdc *SimplifiedDecoratorConfig) ToDecoratorConfigs() []provider.DecoratorConfig {
	var configs []provider.DecoratorConfig

	if sdc.Enabled && sdc.FrequencyControl != nil && sdc.FrequencyControl.Enabled {
		configs = append(configs, provider.DecoratorConfig{
			Type:         SimplifiedFrequencyControlType,
			Enabled:      true,
			Priority:     1, // 频率控制优先级较高
			ProviderType: "realtime",
			Config: map[string]interface{}{
				"min_interval":          sdc.FrequencyControl.MinInterval.String(),
				"max_retries":           sdc.FrequencyControl.MaxRetries,
				"enabled":               sdc.FrequencyControl.Enabled,
				"market_time_aware":     sdc.FrequencyControl.MarketTimeAware,
				"pre_market_buffer":     sdc.FrequencyControl.PreMarketBuffer.String(),
				"post_market_buffer":    sdc.FrequencyControl.PostMarketBuffer.String(),
				"ip_ban_retry_interval": sdc.FrequencyControl.IPBanRetryInterval.String(),
				"ip_ban_retry_max":      sdc.FrequencyControl.IPBanRetryMax,
			},
		})
	}

	if sdc.Enabled && sdc.CircuitBreaker != nil && sdc.CircuitBreaker.Enabled {
		configs = append(configs, provider.DecoratorConfig{
			Type:         SimplifiedCircuitBreakerType,
			Enabled:      true,
			Priority:     2, // 熔断器优先级较低
			ProviderType: "realtime",
			Config: map[string]interface{}{
				"name":              sdc.CircuitBreaker.Name,
				"max_requests":      sdc.CircuitBreaker.MaxRequests,
				"interval":          sdc.CircuitBreaker.Interval.String(),
				"timeout":           sdc.CircuitBreaker.Timeout.String(),
				"ready_to_trip":     sdc.CircuitBreaker.ReadyToTrip,
				"enabled":           sdc.CircuitBreaker.Enabled,
				"market_time_aware": sdc.CircuitBreaker.MarketTimeAware,
			},
		})
	}

	return configs
}

// CreateSimplifiedDecoratedProvider 使用简化配置创建装饰的提供商
func CreateSimplifiedDecoratedProvider(stockProvider provider.RealtimeStockProvider, config *SimplifiedDecoratorConfig) (provider.Provider, error) {
	if config == nil {
		config = NewSimplifiedDecoratorConfig()
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	current := stockProvider

	// 应用频率控制装饰器
	if config.Enabled && config.FrequencyControl != nil && config.FrequencyControl.Enabled {
		current = NewSimplifiedFrequencyControlProvider(current, config.FrequencyControl)
	}

	// 应用熔断器装饰器
	if config.Enabled && config.CircuitBreaker != nil && config.CircuitBreaker.Enabled {
		current = NewSimplifiedCircuitBreakerProvider(current, config.CircuitBreaker)
	}

	return current, nil
}

// CreateSimplifiedDecoratedProviderFromViper 从 Viper 配置创建简化装饰的提供商
func CreateSimplifiedDecoratedProviderFromViper(stockProvider provider.RealtimeStockProvider, v *viper.Viper, configKey string) (provider.Provider, error) {
	config := NewSimplifiedDecoratorConfig()

	if err := config.LoadFromViper(v, configKey); err != nil {
		return nil, err
	}

	return CreateSimplifiedDecoratedProvider(stockProvider, config)
}

// 预定义配置模板

// DefaultSimplifiedConfig 默认简化装饰器配置模板
func DefaultSimplifiedConfig() *SimplifiedDecoratorConfig {
	return &SimplifiedDecoratorConfig{
		FrequencyControl: &SimplifiedFrequencyControlConfig{
			MinInterval:        200 * time.Millisecond,
			MaxRetries:         3,
			Enabled:            true,
			MarketTimeAware:    true,
			PreMarketBuffer:    5 * time.Minute,
			PostMarketBuffer:   10 * time.Minute,
			IPBanRetryInterval: 5 * time.Minute,
			IPBanRetryMax:      3,
		},
		CircuitBreaker: &SimplifiedCircuitBreakerConfig{
			Name:            "SimplifiedStockProvider",
			MaxRequests:     5,
			Interval:        60 * time.Second,
			Timeout:         30 * time.Second,
			ReadyToTrip:     5,
			Enabled:         true,
			MarketTimeAware: true,
		},
		Enabled: true,
	}
}

// ProductionSimplifiedConfig 生产环境简化装饰器配置模板
func ProductionSimplifiedConfig() *SimplifiedDecoratorConfig {
	return &SimplifiedDecoratorConfig{
		FrequencyControl: &SimplifiedFrequencyControlConfig{
			MinInterval:        1 * time.Second, // 生产环境使用更长的间隔
			MaxRetries:         5,
			Enabled:            true,
			MarketTimeAware:    true,
			PreMarketBuffer:    10 * time.Minute,
			PostMarketBuffer:   15 * time.Minute,
			IPBanRetryInterval: 10 * time.Minute,
			IPBanRetryMax:      5,
		},
		CircuitBreaker: &SimplifiedCircuitBreakerConfig{
			Name:            "ProductionSimplifiedStockProvider",
			MaxRequests:     3,
			Interval:        120 * time.Second,
			Timeout:         60 * time.Second,
			ReadyToTrip:     3,
			Enabled:         true,
			MarketTimeAware: true,
		},
		Enabled: true,
	}
}

// TestSimplifiedConfig 测试环境简化装饰器配置模板
func TestSimplifiedConfig() *SimplifiedDecoratorConfig {
	return &SimplifiedDecoratorConfig{
		FrequencyControl: &SimplifiedFrequencyControlConfig{
			MinInterval:        10 * time.Millisecond, // 测试环境使用很短的间隔
			MaxRetries:         1,
			Enabled:            false, // 测试环境默认禁用
			MarketTimeAware:    false,
			PreMarketBuffer:    1 * time.Minute,
			PostMarketBuffer:   1 * time.Minute,
			IPBanRetryInterval: 1 * time.Minute,
			IPBanRetryMax:      1,
		},
		CircuitBreaker: &SimplifiedCircuitBreakerConfig{
			Name:            "TestSimplifiedStockProvider",
			MaxRequests:     10,
			Interval:        10 * time.Second,
			Timeout:         5 * time.Second,
			ReadyToTrip:     10,
			Enabled:         false, // 测试环境默认禁用
			MarketTimeAware: false,
		},
		Enabled: false,
	}
}

// MonitoringSimplifiedConfig 监控环境简化装饰器配置模板
func MonitoringSimplifiedConfig() *SimplifiedDecoratorConfig {
	return &SimplifiedDecoratorConfig{
		FrequencyControl: &SimplifiedFrequencyControlConfig{
			MinInterval:        3 * time.Second, // 监控环境使用中等间隔
			MaxRetries:         5,
			Enabled:            true,
			MarketTimeAware:    true,
			PreMarketBuffer:    5 * time.Minute,
			PostMarketBuffer:   10 * time.Minute,
			IPBanRetryInterval: 15 * time.Minute,
			IPBanRetryMax:      10,
		},
		CircuitBreaker: &SimplifiedCircuitBreakerConfig{
			Name:            "MonitoringSimplifiedStockProvider",
			MaxRequests:     10,
			Interval:        300 * time.Second,
			Timeout:         120 * time.Second,
			ReadyToTrip:     10,
			Enabled:         true,
			MarketTimeAware: true,
		},
		Enabled: true,
	}
}
