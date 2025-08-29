package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"stocksub/pkg/core"
	"stocksub/pkg/provider"
)

// MockIndexProvider 模拟的指数数据提供商，用于演示
type MockIndexProvider struct{}

func (m *MockIndexProvider) Name() string {
	return "mock-index"
}

func (m *MockIndexProvider) IsHealthy() bool {
	return true
}

func (m *MockIndexProvider) GetRateLimit() time.Duration {
	return 300 * time.Millisecond // 指数数据请求频率中等
}

func (m *MockIndexProvider) FetchIndexData(ctx context.Context, indexSymbols []string) ([]core.IndexData, error) {
	// 模拟生成一些指数数据
	var data []core.IndexData

	// 预定义的指数基础数据
	indexBaseData := map[string]struct {
		name      string
		baseValue float64
	}{
		"sh000001": {"上证指数", 3100.0},
		"sz399001": {"深证成指", 12000.0},
		"sz399006": {"创业板指", 2400.0},
		"sh000300": {"沪深300", 4200.0},
		"sz399905": {"中证500", 6800.0},
	}

	for i, symbol := range indexSymbols {
		baseInfo, exists := indexBaseData[symbol]
		if !exists {
			// 对于未知指数，使用默认值
			baseInfo = struct {
				name      string
				baseValue float64
			}{"未知指数", 1000.0}
		}

		// 模拟价格波动
		change := float64((i%20 - 10)) * 0.5 // -5 到 +5 点的变化
		currentValue := baseInfo.baseValue + change
		changePercent := (change / baseInfo.baseValue) * 100

		indexData := core.IndexData{
			Symbol:        symbol,
			Name:          baseInfo.name,
			Value:         currentValue,
			Change:        change,
			ChangePercent: changePercent,
			Volume:        int64(100000000 + i*10000000),   // 模拟成交量
			Turnover:      float64(500000000 + i*50000000), // 模拟成交额
		}

		data = append(data, indexData)
	}

	return data, nil
}

func (m *MockIndexProvider) IsIndexSupported(indexSymbol string) bool {
	// 支持常见的中国指数代码
	supportedIndexes := map[string]bool{
		"sh000001": true, // 上证指数
		"sz399001": true, // 深证成指
		"sz399006": true, // 创业板指
		"sh000300": true, // 沪深300
		"sz399905": true, // 中证500
	}

	return supportedIndexes[indexSymbol]
}

// 演示如何使用指数数据提供商
func main() {
	fmt.Println("=== 指数数据提供商示例 ===")

	// 1. 创建模拟的指数数据提供商
	indexProvider := &MockIndexProvider{}

	fmt.Printf("提供商名称: %s\n", indexProvider.Name())
	fmt.Printf("健康状态: %t\n", indexProvider.IsHealthy())
	fmt.Printf("频率限制: %v\n", indexProvider.GetRateLimit())

	// 2. 检查指数代码支持
	fmt.Println("\n--- 指数代码支持检查 ---")
	testIndexes := []string{
		"sh000001",   // 上证指数
		"sz399001",   // 深证成指
		"sz399006",   // 创业板指
		"sh000300",   // 沪深300
		"sz399905",   // 中证500
		"unknown001", // 不支持的指数
	}

	fmt.Println("检查指数代码支持:")
	var supportedIndexes []string
	for _, index := range testIndexes {
		supported := indexProvider.IsIndexSupported(index)
		fmt.Printf("  %s: %t\n", index, supported)
		if supported {
			supportedIndexes = append(supportedIndexes, index)
		}
	}

	// 3. 获取指数数据
	fmt.Println("\n--- 获取指数数据示例 ---")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("获取 %d 个指数的实时数据\n", len(supportedIndexes))
	indexData, err := indexProvider.FetchIndexData(ctx, supportedIndexes)
	if err != nil {
		log.Fatalf("获取指数数据失败: %v", err)
	}

	fmt.Printf("成功获取 %d 个指数的数据\n", len(indexData))
	for _, index := range indexData {
		fmt.Printf("  %s (%s): %.2f点, 涨跌: %+.2f (%.2f%%), 成交量: %d, 成交额: %.0f万元\n",
			index.Symbol, index.Name, index.Value,
			index.Change, index.ChangePercent,
			index.Volume, index.Turnover/10000)
	}

	// 4. 演示使用装饰器
	fmt.Println("\n--- 使用装饰器的指数提供商示例 ---")

	// 注意：由于当前装饰器主要为 RealtimeStockProvider 和 HistoricalProvider 设计，
	// 对于 IndexProvider 我们可以创建一个适配器或使用通用装饰器
	// 这里演示如何在未来扩展装饰器支持时使用

	// 创建自定义装饰器配置，专门用于指数数据
	customConfig := provider.ProviderDecoratorConfig{
		Index: []provider.DecoratorConfig{
			{
				Type:         provider.FrequencyControlType,
				Enabled:      true,
				Priority:     1,
				ProviderType: "index",
				Config: map[string]interface{}{
					"min_interval_ms": 500, // 指数数据更新频率相对较慢
					"max_retries":     3,
					"enabled":         true,
				},
			},
			{
				Type:         provider.CircuitBreakerType,
				Enabled:      true,
				Priority:     2,
				ProviderType: "index",
				Config: map[string]interface{}{
					"name":          "IndexProvider",
					"max_requests":  5,
					"interval":      "60s",
					"timeout":       "30s",
					"ready_to_trip": 5,
					"enabled":       true,
				},
			},
		},
	}

	// 注意：当前的装饰器实现主要支持 RealtimeStockProvider 和 HistoricalProvider
	// 为了完整的指数提供商支持，需要扩展装饰器框架
	fmt.Println("指数提供商装饰器配置已准备:")
	fmt.Printf("  装饰器配置数量: %d\n", len(customConfig.Index))
	fmt.Printf("  频率控制: %dms 间隔\n", 500)
	fmt.Printf("  熔断器: 最大5个请求，60s 间隔\n")
	fmt.Println("  (注: 完整的装饰器支持需要进一步的框架扩展)")

	// 5. 演示指数数据分析
	fmt.Println("\n--- 指数数据分析示例 ---")
	if len(indexData) > 0 {
		// 计算市场整体表现
		var totalChange float64
		var positiveCount, negativeCount int
		var maxGain, maxLoss float64
		var maxGainIndex, maxLossIndex string

		for _, index := range indexData {
			totalChange += index.ChangePercent

			if index.ChangePercent > 0 {
				positiveCount++
				if index.ChangePercent > maxGain {
					maxGain = index.ChangePercent
					maxGainIndex = index.Name
				}
			} else if index.ChangePercent < 0 {
				negativeCount++
				if index.ChangePercent < maxLoss {
					maxLoss = index.ChangePercent
					maxLossIndex = index.Name
				}
			}
		}

		avgChange := totalChange / float64(len(indexData))

		fmt.Printf("市场整体分析:\n")
		fmt.Printf("  平均涨跌幅: %.2f%%\n", avgChange)
		fmt.Printf("  上涨指数: %d个\n", positiveCount)
		fmt.Printf("  下跌指数: %d个\n", negativeCount)
		fmt.Printf("  平盘指数: %d个\n", len(indexData)-positiveCount-negativeCount)

		if maxGainIndex != "" {
			fmt.Printf("  最大涨幅: %s (%.2f%%)\n", maxGainIndex, maxGain)
		}
		if maxLossIndex != "" {
			fmt.Printf("  最大跌幅: %s (%.2f%%)\n", maxLossIndex, maxLoss)
		}

		// 成交量分析
		var totalVolume int64
		var totalTurnover float64
		for _, index := range indexData {
			totalVolume += index.Volume
			totalTurnover += index.Turnover
		}

		fmt.Printf("交易分析:\n")
		fmt.Printf("  总成交量: %d\n", totalVolume)
		fmt.Printf("  总成交额: %.0f万元\n", totalTurnover/10000)
		fmt.Printf("  平均成交量: %d\n", totalVolume/int64(len(indexData)))
		fmt.Printf("  平均成交额: %.0f万元\n", totalTurnover/10000/float64(len(indexData)))
	}

	// 6. 演示定时获取指数数据
	fmt.Println("\n--- 定时获取指数数据示例 ---")
	fmt.Println("演示每3秒获取一次指数数据 (共获取3次):")

	for i := 0; i < 3; i++ {
		fmt.Printf("\n第 %d 次获取 (%s):\n", i+1, time.Now().Format("15:04:05"))

		// 只获取主要指数
		mainIndexes := []string{"sh000001", "sz399001", "sz399006"}
		currentData, err := indexProvider.FetchIndexData(ctx, mainIndexes)
		if err != nil {
			log.Printf("获取指数数据失败: %v", err)
			continue
		}

		for _, index := range currentData {
			fmt.Printf("  %s: %.2f点 (%+.2f%%)\n",
				index.Name, index.Value, index.ChangePercent)
		}

		if i < 2 { // 最后一次不需要等待
			time.Sleep(3 * time.Second)
		} 
	}

	// 7. 演示错误处理
	fmt.Println("\n--- 错误处理示例 ---")

	// 使用超时的上下文
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer timeoutCancel()

	_, err = indexProvider.FetchIndexData(timeoutCtx, []string{"sh000001"})
	if err != nil {
		fmt.Printf("预期的超时错误: %v\n", err)
	}

	// 测试不支持的指数
	fmt.Println("测试不支持的指数代码:")
	unsupportedIndexes := []string{"invalid001", "test999"}
	unsupportedData, err := indexProvider.FetchIndexData(ctx, unsupportedIndexes)
	if err != nil {
		fmt.Printf("获取不支持指数时的错误: %v\n", err)
	} else {
		fmt.Printf("获取到 %d 条不支持指数的数据 (使用默认值)\n", len(unsupportedData))
		for _, index := range unsupportedData {
			fmt.Printf("  %s: %s %.2f点\n", index.Symbol, index.Name, index.Value)
		}
	}

	fmt.Println("\n=== 示例完成 ===")
}
