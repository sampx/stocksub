package main

import (
	"context"
	"fmt"
	"stocksub/pkg/core"
	"stocksub/pkg/provider"
	"stocksub/pkg/provider/decorators"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// ConfigurableStockProvider 可配置的股票数据提供商
type ConfigurableStockProvider struct {
	name   string
	stocks map[string]core.StockData
}

// NewConfigurableStockProvider 创建可配置的股票数据提供商
func NewConfigurableStockProvider(name string) *ConfigurableStockProvider {
	return &ConfigurableStockProvider{
		name: name,
		stocks: map[string]core.StockData{
			"000001.SZ": {
				Symbol:        "000001.SZ",
				Name:          "平安银行",
				Price:         12.34,
				Change:        0.56,
				ChangePercent: 4.75,
				Volume:        1000000,
				Timestamp:     time.Now(),
			},
			"000002.SZ": {
				Symbol:        "000002.SZ",
				Name:          "万科A",
				Price:         23.45,
				Change:        -0.12,
				ChangePercent: -0.51,
				Volume:        800000,
				Timestamp:     time.Now(),
			},
			"600000.SH": {
				Symbol:        "600000.SH",
				Name:          "浦发银行",
				Price:         8.90,
				Change:        0.15,
				ChangePercent: 1.71,
				Volume:        2000000,
				Timestamp:     time.Now(),
			},
		},
	}
}

func (p *ConfigurableStockProvider) Name() string {
	return p.name
}

func (p *ConfigurableStockProvider) IsHealthy() bool {
	return true
}

func (p *ConfigurableStockProvider) GetRateLimit() time.Duration {
	return 100 * time.Millisecond
}

func (p *ConfigurableStockProvider) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	result := make([]core.StockData, 0, len(symbols))
	
	for _, symbol := range symbols {
		if stock, exists := p.stocks[symbol]; exists {
			// 更新时间戳
			stock.Timestamp = time.Now()
			result = append(result, stock)
		}
	}
	
	return result, nil
}

func (p *ConfigurableStockProvider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	data, err := p.FetchStockData(ctx, symbols)
	if err != nil {
		return nil, "", err
	}
	raw := fmt.Sprintf("Raw response from %s for symbols: %v", p.name, symbols)
	return data, raw, nil
}

func (p *ConfigurableStockProvider) IsSymbolSupported(symbol string) bool {
	_, exists := p.stocks[symbol]
	return exists
}

func main() {
	fmt.Println("=== 装饰器配置示例 ===")

	// 创建基础提供商
	baseProvider := NewConfigurableStockProvider("ConfigurableProvider")

	// 示例1：使用代码定义的配置
	fmt.Println("\n=== 代码定义配置 ===")
	
	// 创建默认配置
	defaultConfig := decorators.DefaultDecoratorConfig()
	fmt.Println("默认配置:")
	printDecoratorConfig(defaultConfig)

	// 应用默认配置
	defaultDecorated, err := decorators.CreateDecoratedProvider(baseProvider, defaultConfig)
	if err != nil {
		fmt.Printf("应用默认配置失败: %v\n", err)
		return
	}

	fmt.Printf("默认装饰器: %s\n", defaultDecorated.Name())
	testProvider(defaultDecorated, "默认配置")

	// 示例2：生产环境配置
	fmt.Println("\n=== 生产环境配置 ===")
	
	productionConfig := decorators.ProductionDecoratorConfig()
	fmt.Println("生产环境配置:")
	printDecoratorConfig(productionConfig)

	productionDecorated, err := decorators.CreateDecoratedProvider(baseProvider, productionConfig)
	if err != nil {
		fmt.Printf("应用生产环境配置失败: %v\n", err)
		return
	}

	fmt.Printf("生产环境装饰器: %s\n", productionDecorated.Name())
	testProvider(productionDecorated, "生产环境配置")

	// 示例3：测试环境配置（所有装饰器被禁用）
	fmt.Println("\n=== 测试环境配置 ===")
	
	testConfig := decorators.TestDecoratorConfig()
	fmt.Println("测试环境配置:")
	printDecoratorConfig(testConfig)

	testDecorated, err := decorators.CreateDecoratedProvider(baseProvider, testConfig)
	if err != nil {
		fmt.Printf("应用测试环境配置失败: %v\n", err)
		return
	}

	fmt.Printf("测试环境装饰器: %s\n", testDecorated.Name())
	testProvider(testDecorated, "测试环境配置")

	// 示例4：监控环境配置
	fmt.Println("\n=== 监控环境配置 ===")
	
	monitoringConfig := decorators.MonitoringDecoratorConfig()
	fmt.Println("监控环境配置:")
	printDecoratorConfig(monitoringConfig)

	monitoringDecorated, err := decorators.CreateDecoratedProvider(baseProvider, monitoringConfig)
	if err != nil {
		fmt.Printf("应用监控环境配置失败: %v\n", err)
		return
	}

	fmt.Printf("监控环境装饰器: %s\n", monitoringDecorated.Name())
	testProvider(monitoringDecorated, "监控环境配置")

	// 示例5：自定义配置
	fmt.Println("\n=== 自定义配置 ===")
	
	customConfig := provider.ProviderDecoratorConfig{
		All: []provider.DecoratorConfig{
			{
				Type:         provider.CircuitBreakerType,
				Enabled:      true,
				Priority:     2,
				ProviderType: "all",
				Config: map[string]interface{}{
					"name":          "CustomGlobalCB",
					"max_requests":  10,
					"interval":      "60s",
					"timeout":       "30s",
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
					"min_interval_ms": 100,
					"max_retries":     5,
					"enabled":         true,
				},
			},
		},
	}

	fmt.Println("自定义配置:")
	printDecoratorConfig(customConfig)

	customDecorated, err := decorators.CreateDecoratedProvider(baseProvider, customConfig)
	if err != nil {
		fmt.Printf("应用自定义配置失败: %v\n", err)
		return
	}

	fmt.Printf("自定义装饰器: %s\n", customDecorated.Name())
	testProvider(customDecorated, "自定义配置")

	// 示例6：使用 Viper 从文件加载配置（模拟）
	fmt.Println("\n=== 从配置文件加载 (模拟) ===")
	
	// 创建模拟的配置文件内容
	yamlConfig := `
decorator:
  all:
    - type: circuit_breaker
      enabled: true
      priority: 2
      provider_type: all
      config:
        name: "FileBasedCB"
        max_requests: 5
        interval: "30s"
        timeout: "15s"
        ready_to_trip: 5
        enabled: true
  
  realtime:
    - type: frequency_control
      enabled: true
      priority: 1
      provider_type: realtime
      config:
        min_interval_ms: 250
        max_retries: 3
        enabled: true
`

	// 使用 Viper 解析配置
	v := viper.New()
	v.SetConfigType("yaml")
	v.ReadConfig(strings.NewReader(yamlConfig))

	// 从 Viper 创建装饰器链
	fileBasedDecorated, err := decorators.CreateDecoratedProviderFromViper(baseProvider, v, "decorator")
	if err != nil {
		fmt.Printf("从 Viper 配置创建装饰器失败: %v\n", err)
		return
	}

	fmt.Printf("配置文件装饰器: %s\n", fileBasedDecorated.Name())
	testProvider(fileBasedDecorated, "配置文件")

	// 示例7：动态配置调整演示
	fmt.Println("\n=== 动态配置调整演示 ===")
	
	// 创建可配置装饰器链
	chain := decorators.NewConfigurableDecoratorChain()
	
	// 动态添加装饰器配置
	fmt.Println("动态添加频率控制装饰器...")
	chain.AddDecorator(provider.DecoratorConfig{
		Type:         provider.FrequencyControlType,
		Enabled:      true,
		Priority:     1,
		ProviderType: "realtime",
		Config: map[string]interface{}{
			"min_interval_ms": 200,
			"max_retries":     2,
			"enabled":         true,
		},
	})

	fmt.Println("动态添加熔断器装饰器...")
	chain.AddDecorator(provider.DecoratorConfig{
		Type:         provider.CircuitBreakerType,
		Enabled:      true,
		Priority:     2,
		ProviderType: "all",
		Config: map[string]interface{}{
			"name":          "DynamicCB",
			"max_requests":  3,
			"interval":      "10s",
			"timeout":       "5s",
			"ready_to_trip": 3,
			"enabled":       true,
		},
	})

	// 应用动态配置
	dynamicDecorated, err := chain.Apply(baseProvider)
	if err != nil {
		fmt.Printf("应用动态配置失败: %v\n", err)
		return
	}

	fmt.Printf("动态配置装饰器: %s\n", dynamicDecorated.Name())
	
	// 显示将要应用的装饰器类型
	decoratorTypes := chain.GetAppliedDecorators(baseProvider)
	fmt.Printf("应用的装饰器类型: %v\n", decoratorTypes)
	
	testProvider(dynamicDecorated, "动态配置")

	fmt.Println("\n装饰器配置示例完成！")
	
	fmt.Println("\n=== 总结 ===")
	fmt.Println("1. 默认配置：适合快速开发和原型")
	fmt.Println("2. 生产环境配置：更严格的限制和更长的超时")
	fmt.Println("3. 测试环境配置：禁用装饰器以便于测试")
	fmt.Println("4. 监控环境配置：为长期监控优化的配置")
	fmt.Println("5. 自定义配置：根据特定需求定制")
	fmt.Println("6. 配置文件：支持从 YAML/JSON 等格式加载")
	fmt.Println("7. 动态配置：运行时添加和调整装饰器")
}

// printDecoratorConfig 打印装饰器配置
func printDecoratorConfig(config provider.ProviderDecoratorConfig) {
	fmt.Printf("  全局装饰器: %d个\n", len(config.All))
	for i, decorator := range config.All {
		fmt.Printf("    %d. %s (启用: %v, 优先级: %d)\n", 
			i+1, decorator.Type, decorator.Enabled, decorator.Priority)
	}

	fmt.Printf("  实时装饰器: %d个\n", len(config.Realtime))
	for i, decorator := range config.Realtime {
		fmt.Printf("    %d. %s (启用: %v, 优先级: %d)\n", 
			i+1, decorator.Type, decorator.Enabled, decorator.Priority)
	}

	fmt.Printf("  历史装饰器: %d个\n", len(config.Historical))
	for i, decorator := range config.Historical {
		fmt.Printf("    %d. %s (启用: %v, 优先级: %d)\n", 
			i+1, decorator.Type, decorator.Enabled, decorator.Priority)
	}

	fmt.Printf("  指数装饰器: %d个\n", len(config.Index))
	for i, decorator := range config.Index {
		fmt.Printf("    %d. %s (启用: %v, 优先级: %d)\n", 
			i+1, decorator.Type, decorator.Enabled, decorator.Priority)
	}
}

// testProvider 测试提供商
func testProvider(decoratedProvider provider.Provider, configName string) {
	if realtimeProvider, ok := decoratedProvider.(provider.RealtimeStockProvider); ok {
		symbols := []string{"000001.SZ", "000002.SZ"}
		ctx := context.Background()

		fmt.Printf("测试 %s:\n", configName)
		
		start := time.Now()
		data, err := realtimeProvider.FetchStockData(ctx, symbols)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("  请求失败: %v, 耗时: %v\n", err, elapsed)
		} else {
			fmt.Printf("  请求成功: 获取%d条数据, 耗时: %v\n", len(data), elapsed)
			for _, stock := range data {
				fmt.Printf("    %s (%s): ¥%.2f\n", stock.Symbol, stock.Name, stock.Price)
			}
		}
		fmt.Printf("  健康状态: %v\n", decoratedProvider.IsHealthy())
		fmt.Printf("  频率限制: %v\n", decoratedProvider.GetRateLimit())
	} else {
		fmt.Printf("测试 %s: 无法转换为RealtimeStockProvider\n", configName)
	}
}