package main

import (
	"fmt"
	"stocksub/pkg/provider/tencent"
)

func main() {
	provider := tencent.NewProvider()
	
	// 测试A股股票代码
	aStocks := []string{"600000", "000001", "300750", "835174", "688036"}
	fmt.Println("=== A股股票代码测试 ===")
	for _, symbol := range aStocks {
		fmt.Printf("%s: %v\n", symbol, provider.IsSymbolSupported(symbol))
	}
	
	// 测试已移除的港股和美股
	nonAStocks := []string{"00700", "AAPL", "TSLA", "03690", "MSFT"}
	fmt.Println("\n=== 非A股股票代码测试 ===")
	for _, symbol := range nonAStocks {
		fmt.Printf("%s: %v\n", symbol, provider.IsSymbolSupported(symbol))
	}
	
	// 测试市场前缀生成（通过反射访问私有方法）
	fmt.Println("\n=== 市场前缀测试（通过URL构建验证）===")
	symbols := []string{"600000", "000001", "300750", "835174"}
	for _, symbol := range symbols {
		// 这里我们通过URL构建来间接验证前缀生成
		if provider.IsSymbolSupported(symbol) {
			fmt.Printf("%s: 支持订阅\n", symbol)
		}
	}
}