package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"stocksub/pkg/core"
	"stocksub/pkg/provider/decorators"
	"stocksub/pkg/provider/tencent"
)

// 演示如何使用实时股票提供商
func main() {
	fmt.Println("=== 实时股票数据提供商示例 ===")

	// 1. 创建基础的腾讯提供商
	tencentProvider := tencent.NewClient()
	defer tencentProvider.Close()

	fmt.Printf("提供商名称: %s\n", tencentProvider.Name())
	fmt.Printf("默认频率限制: %v\n", tencentProvider.GetRateLimit())
	fmt.Printf("健康状态: %t\n", tencentProvider.IsHealthy())

	// 2. 演示基础数据获取
	fmt.Println("\n--- 基础数据获取示例 ---")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	symbols := []string{"000858", "600000", "300503"}

	// 检查符号支持
	fmt.Println("检查股票代码支持:")
	for _, symbol := range symbols {
		supported := tencentProvider.IsSymbolSupported(symbol)
		fmt.Printf("  %s: %t\n", symbol, supported)
	}

	// 获取股票数据
	fmt.Println("\n获取实时股票数据:")
	data, err := tencentProvider.FetchStockData(ctx, symbols)
	if err != nil {
		log.Printf("获取数据失败: %v", err)
	} else {
		fmt.Printf("成功获取 %d 条股票数据\n", len(data))
		for _, stock := range data {
			fmt.Printf("  %s (%s): ¥%.2f, 涨跌: %.2f (%.2f%%)\n",
				stock.Symbol, stock.Name, stock.Price,
				stock.Change, stock.ChangePercent)
		}
	}

	// 3. 演示带原始数据的获取
	fmt.Println("\n--- 带原始数据的获取示例 ---")
	dataWithRaw, rawData, err := tencentProvider.FetchStockDataWithRaw(ctx, []string{"000858"})
	if err != nil {
		log.Printf("获取带原始数据失败: %v", err)
	} else {
		fmt.Printf("获取数据条数: %d\n", len(dataWithRaw))
		fmt.Printf("原始数据长度: %d 字符\n", len(rawData))
		if len(rawData) > 100 {
			fmt.Printf("原始数据预览: %s...\n", rawData[:100])
		}
	}

	// 4. 演示使用装饰器的提供商
	fmt.Println("\n--- 使用装饰器的提供商示例 ---")

	// 使用默认装饰器配置
	decoratedProvider, err := decorators.CreateDecoratedProvider(
		tencentProvider,
		decorators.DefaultDecoratorConfig(),
	)
	if err != nil {
		log.Fatalf("创建装饰后的提供商失败: %v", err)
	}

	fmt.Printf("装饰后提供商类型: %T\n", decoratedProvider)

	// 类型断言以使用 RealtimeStockProvider 方法
	if stockProvider, ok := decoratedProvider.(interface {
		FetchStockData(context.Context, []string) ([]core.StockData, error)
		Name() string
		IsHealthy() bool
	}); ok {
		fmt.Printf("装饰后提供商名称: %s\n", stockProvider.Name())
		fmt.Printf("装饰后健康状态: %t\n", stockProvider.IsHealthy())

		// 注意：装饰器会添加频率控制和熔断功能
		fmt.Println("通过装饰器获取数据...")
		// 这里会应用装饰器的功能，如频率控制和熔断
		decoratedData, err := stockProvider.FetchStockData(ctx, []string{"000858"})
		if err != nil {
			log.Printf("通过装饰器获取数据失败: %v", err)
		} else {
			fmt.Printf("通过装饰器成功获取 %d 条数据\n", len(decoratedData))
			if len(decoratedData) > 0 {
				stock := decoratedData[0]
				fmt.Printf("  %s: ¥%.2f\n", stock.Symbol, stock.Price)
			}
		}
	}

	// 5. 演示生产环境配置
	fmt.Println("\n--- 生产环境装饰器配置示例 ---")
	prodProvider, err := decorators.CreateDecoratedProvider(
		tencent.NewClient(),
		decorators.ProductionDecoratorConfig(),
	)
	if err != nil {
		log.Fatalf("创建生产环境装饰器失败: %v", err)
	}

	fmt.Printf("生产环境提供商类型: %T\n", prodProvider)
	fmt.Println("生产环境配置特点:")
	fmt.Println("  - 更长的频率控制间隔 (5秒)")
	fmt.Println("  - 更严格的熔断器设置")
	fmt.Println("  - 适合长期稳定运行")

	fmt.Println("\n=== 示例完成 ===")
}
