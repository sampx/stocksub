package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"stocksub/pkg/core"
	"stocksub/pkg/provider"
	"stocksub/pkg/provider/decorators"
)

// MockHistoricalProvider 模拟的历史数据提供商，用于演示
type MockHistoricalProvider struct{}

func (m *MockHistoricalProvider) Name() string {
	return "mock-historical"
}

func (m *MockHistoricalProvider) IsHealthy() bool {
	return true
}

func (m *MockHistoricalProvider) GetRateLimit() time.Duration {
	return 1 * time.Second // 历史数据请求频率较慢
}

func (m *MockHistoricalProvider) FetchHistoricalData(ctx context.Context, symbol string, start, end time.Time, period string) ([]core.HistoricalData, error) {
	// 模拟生成一些历史数据
	var data []core.HistoricalData

	current := start
	price := 100.0 // 初始价格

	for current.Before(end) && len(data) < 30 { // 限制最多30条数据
		// 模拟价格波动
		change := (float64(len(data)%10) - 5) * 0.5
		price += change

		high := price + 2.0
		low := price - 2.0
		open := price - change/2

		histData := core.HistoricalData{
			Symbol:    symbol,
			Timestamp: current,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     price,
			Volume:    int64(10000 + len(data)*1000),
			Turnover:  price * float64(10000+len(data)*1000),
			Period:    period,
		}

		data = append(data, histData)
		current = current.AddDate(0, 0, 1) // 每天
	}

	return data, nil
}

func (m *MockHistoricalProvider) GetSupportedPeriods() []string {
	return []string{"1d", "1w", "1M"}
}

// 演示如何使用历史数据提供商
func main() {
	fmt.Println("=== 历史数据提供商示例 ===")

	// 1. 创建模拟的历史数据提供商
	historicalProvider := &MockHistoricalProvider{}

	fmt.Printf("提供商名称: %s\n", historicalProvider.Name())
	fmt.Printf("健康状态: %t\n", historicalProvider.IsHealthy())
	fmt.Printf("频率限制: %v\n", historicalProvider.GetRateLimit())

	// 2. 查看支持的时间周期
	fmt.Println("\n--- 支持的时间周期 ---")
	supportedPeriods := historicalProvider.GetSupportedPeriods()
	fmt.Printf("支持的周期: %v\n", supportedPeriods)

	// 3. 获取历史数据
	fmt.Println("\n--- 获取历史数据示例 ---")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	symbol := "000858"
	startTime := time.Now().AddDate(0, 0, -30) // 30天前
	endTime := time.Now()
	period := "1d"

	fmt.Printf("获取 %s 从 %s 到 %s 的 %s 历史数据\n",
		symbol,
		startTime.Format("2006-01-02"),
		endTime.Format("2006-01-02"),
		period)

	histData, err := historicalProvider.FetchHistoricalData(ctx, symbol, startTime, endTime, period)
	if err != nil {
		log.Fatalf("获取历史数据失败: %v", err)
	}

	fmt.Printf("成功获取 %d 条历史数据\n", len(histData))

	// 显示前5条和后5条数据
	showCount := 5
	if len(histData) > 0 {
		fmt.Println("\n前5条数据:")
		for i := 0; i < showCount && i < len(histData); i++ {
			data := histData[i]
			fmt.Printf("  %s: 开盘=%.2f, 最高=%.2f, 最低=%.2f, 收盘=%.2f, 成交量=%d\n",
				data.Timestamp.Format("2006-01-02"),
				data.Open, data.High, data.Low, data.Close, data.Volume)
		}

		if len(histData) > showCount*2 {
			fmt.Println("  ...")
			fmt.Println("后5条数据:")
			for i := len(histData) - showCount; i < len(histData); i++ {
				data := histData[i]
				fmt.Printf("  %s: 开盘=%.2f, 最高=%.2f, 最低=%.2f, 收盘=%.2f, 成交量=%d\n",
					data.Timestamp.Format("2006-01-02"),
					data.Open, data.High, data.Low, data.Close, data.Volume)
			}
		}
	}

	// 4. 演示使用装饰器
	fmt.Println("\n--- 使用装饰器的历史数据提供商示例 ---")

	// 使用生产环境配置，这会为历史数据提供商添加适当的装饰器
	decoratedProvider, err := decorators.CreateDecoratedProvider(
		historicalProvider,
		decorators.ProductionDecoratorConfig(),
	)
	if err != nil {
		log.Fatalf("创建装饰后的提供商失败: %v", err)
	}

	fmt.Printf("装饰后提供商类型: %T\n", decoratedProvider)

	// 类型断言以使用 HistoricalProvider 接口
	if histProvider, ok := decoratedProvider.(provider.HistoricalProvider); ok {
		fmt.Printf("装饰后提供商名称: %s\n", histProvider.Name())
		fmt.Printf("装饰后健康状态: %t\n", histProvider.IsHealthy())
		fmt.Printf("装饰后支持的周期: %v\n", histProvider.GetSupportedPeriods())

		// 通过装饰器获取数据 - 会应用频率控制和熔断功能
		fmt.Println("\n通过装饰器获取历史数据...")
		decoratedData, err := histProvider.FetchHistoricalData(ctx, symbol,
			time.Now().AddDate(0, 0, -5), time.Now(), "1d")
		if err != nil {
			log.Printf("通过装饰器获取数据失败: %v", err)
		} else {
			fmt.Printf("通过装饰器成功获取 %d 条数据\n", len(decoratedData))
		}
	}

	// 5. 演示不同周期的数据获取
	fmt.Println("\n--- 不同周期数据获取示例 ---")
	for _, period := range supportedPeriods {
		fmt.Printf("\n获取 %s 周期的数据:\n", period)

		periodData, err := historicalProvider.FetchHistoricalData(ctx, symbol,
			time.Now().AddDate(0, 0, -7), time.Now(), period)
		if err != nil {
			log.Printf("获取 %s 周期数据失败: %v", period, err)
			continue
		}

		fmt.Printf("  成功获取 %d 条 %s 周期数据\n", len(periodData), period)
		if len(periodData) > 0 {
			first := periodData[0]
			last := periodData[len(periodData)-1]
			fmt.Printf("  时间范围: %s 到 %s\n",
				first.Timestamp.Format("2006-01-02"),
				last.Timestamp.Format("2006-01-02"))
			fmt.Printf("  价格变化: %.2f → %.2f (%.2f%%)\n",
				first.Close, last.Close,
				(last.Close-first.Close)/first.Close*100)
		}
	}

	// 6. 演示数据分析功能
	fmt.Println("\n--- 历史数据分析示例 ---")
	if len(histData) > 0 {
		// 计算基本统计信息
		var sum, max, min float64
		max = histData[0].Close
		min = histData[0].Close

		for _, data := range histData {
			sum += data.Close
			if data.Close > max {
				max = data.Close
			}
			if data.Close < min {
				min = data.Close
			}
		}

		avg := sum / float64(len(histData))

		fmt.Printf("价格统计:\n")
		fmt.Printf("  平均价格: %.2f\n", avg)
		fmt.Printf("  最高价格: %.2f\n", max)
		fmt.Printf("  最低价格: %.2f\n", min)
		fmt.Printf("  价格波动: %.2f (%.2f%%)\n", max-min, (max-min)/avg*100)

		// 计算总成交量
		var totalVolume int64
		for _, data := range histData {
			totalVolume += data.Volume
		}
		fmt.Printf("  总成交量: %d\n", totalVolume)
		fmt.Printf("  平均成交量: %d\n", totalVolume/int64(len(histData)))
	}

	fmt.Println("\n=== 示例完成 ===")
}
