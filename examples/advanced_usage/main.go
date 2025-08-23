package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit"
	"stocksub/pkg/testkit/config"
)

func main() {
	fmt.Println("=== Testkit 高级用法示例 ===")

	// --- 1. 配置一个更复杂的 TestDataManager ---
	// 使用分层缓存(LayeredCache)和CSV持久化存储
	tempDir, err := os.MkdirTemp("", "testkit_advanced_*")
	if err != nil {
		log.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("\n1. 初始化 TestDataManager...\n")
	fmt.Printf("   - 缓存类型: layered (分层缓存)\n")
	fmt.Printf("   - 存储类型: csv (数据将持久化到 %s)\n", tempDir)

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type: "layered", // 使用分层缓存
		},
		Storage: config.StorageConfig{
			Type:      "csv",
			Directory: tempDir,
		},
	}
	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	// --- 2. 演示 MockProvider 的自动数据生成 ---
	fmt.Println("\n2. 测试 MockProvider 的自动数据生成...")
	manager.EnableMock(true)
	ctx := context.Background()

	// 在不设置任何Mock数据的情况下，MockProvider会回退到自动生成随机数据
	generatedData, err := manager.GetStockData(ctx, []string{"AUTO001", "AUTO002"})
	if err != nil {
		log.Printf("   - 错误: %v", err)
	} else {
		fmt.Printf("   - 成功: 自动生成了 %d 条数据\n", len(generatedData))
		for _, d := range generatedData {
			fmt.Printf("     - %s: %s, 价格: %.2f\n", d.Symbol, d.Name, d.Price)
		}
	}

	// --- 3. 演示设置特定Mock数据以覆盖自动生成 ---
	fmt.Println("\n3. 设置特定Mock数据...")
	symbols := []string{"SPECIFIC001", "AUTO003"} // 一个特定的，一个继续自动生成
	specificData := []subscriber.StockData{
		{
			Symbol: "SPECIFIC001",
			Name:   "指定的模拟股票",
			Price:  999.99,
		},
	}
	// 只为 SPECIFIC001 设置数据
	manager.SetMockData([]string{"SPECIFIC001"}, specificData)

	mixedData, err := manager.GetStockData(ctx, symbols)
	if err != nil {
		log.Printf("   - 错误: %v", err)
	} else {
		fmt.Printf("   - 成功: 获取到 %d 条混合数据\n", len(mixedData))
		for _, d := range mixedData {
			if d.Symbol == "SPECIFIC001" {
				fmt.Printf("     - %s: %s, 价格: %.2f (来自SetMockData)\n", d.Symbol, d.Name, d.Price)
			} else {
				fmt.Printf("     - %s: %s, 价格: %.2f (自动生成)\n", d.Symbol, d.Name, d.Price)
			}
		}
	}

	// --- 4. 在测试中的用法提示 ---
	fmt.Println("\n4. 在Go测试中的典型用法 (伪代码)")
	fmt.Println(`
func TestMyFeature(t *testing.T) {
    // 1. 使用 t.TempDir() 为每个测试创建隔离的目录
    tmpDir := t.TempDir()

    // 2. 配置并创建 manager
    cfg := &config.Config{
        Storage: config.StorageConfig{ Directory: tmpDir },
        // ... 其他配置
    }
    manager := testkit.NewTestDataManager(cfg)
    defer manager.Close()

    // 3. 设置Mock数据并执行测试
    manager.EnableMock(true)
    manager.SetMockData(...)

    // 4. 运行你的业务逻辑并断言
    // myBusinessLogic(manager)
    // assert.Equal(t, ...)
}`)

	fmt.Println("\n=== 示例结束 ===")
}
