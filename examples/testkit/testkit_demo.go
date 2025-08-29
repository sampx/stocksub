package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"stocksub/pkg/core"
	"stocksub/pkg/testkit/config"
	"stocksub/pkg/testkit/manager"
)

func main() {
	fmt.Println("=== pkg/testkit 框架使用示例 ===")

	// 为测试数据创建一个临时目录
	testDir, err := os.MkdirTemp("", "testkit_demo_*")
	if err != nil {
		log.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(testDir)
	fmt.Printf("\n1. 初始化 TestDataManager...\n")
	fmt.Printf("   - 临时数据目录: %s\n", testDir)

	// 示例1: 初始化 TestDataManager
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "memory",
			MaxSize: 100,
			TTL:     5 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "csv",
			Directory: testDir,
		},
	}
	manager := manager.NewTestDataManager(cfg)
	defer manager.Close()

	// 示例2: 使用 Mock 功能
	fmt.Println("\n2. 使用 Mock 功能获取数据...")
	manager.EnableMock(true)

	symbols := []string{"MOCK001"}
	mockData := []core.StockData{
		{
			Symbol: "MOCK001",
			Name:   "模拟股票",
			Price:  123.45,
		},
	}
	manager.SetMockData(symbols, mockData)

	data, err := manager.GetStockData(context.Background(), symbols)
	if err != nil {
		log.Printf("获取Mock数据失败: %v", err)
	} else {
		fmt.Printf("   - 成功获取到 %d 条Mock数据\n", len(data))
		fmt.Printf("   - 股票名称: %s, 价格: %.2f\n", data[0].Name, data[0].Price)
	}

	// 示例3: 获取统计信息
	fmt.Println("\n3. 获取统计信息...")
	stats := manager.GetStats()
	fmt.Printf("   - 缓存命中: %d\n", stats.CacheHits)
	fmt.Printf("   - 缓存未命中: %d\n", stats.CacheMisses)
	fmt.Printf("   - Mock模式: %v\n", stats.MockMode)

	fmt.Println("\n=== 示例结束 ===")
}
