package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"stocksub/pkg/testkit/cache"
)

// basicLayeredCacheDemo 基础分层缓存演示
func basicLayeredCacheDemo(ctx context.Context) {
	// 使用默认分层配置
	config := cache.DefaultLayeredCacheConfig()
	fmt.Printf("   创建默认分层缓存配置:\n")
	fmt.Printf("   - L1: %s, 容量=%d, TTL=%v, 策略=%s\n",
		config.Layers[0].Type, config.Layers[0].MaxSize, config.Layers[0].TTL, config.Layers[0].Policy)
	fmt.Printf("   - L2: %s, 容量=%d, TTL=%v, 策略=%s\n",
		config.Layers[1].Type, config.Layers[1].MaxSize, config.Layers[1].TTL, config.Layers[1].Policy)

	layeredCache, err := cache.NewLayeredCache(config)
	if err != nil {
		log.Printf("创建分层缓存失败: %v", err)
		return
	}
	defer layeredCache.Close()

	// 存储一些数据
	stocks := map[string]string{
		"600000": "浦发银行",
		"000001": "平安银行",
		"600036": "招商银行",
		"601318": "中国平安",
	}

	for symbol, name := range stocks {
		err := layeredCache.Set(ctx, "stock:"+symbol, name, 0)
		if err != nil {
			log.Printf("设置缓存失败: %v", err)
			continue
		}
		fmt.Printf("   ✓ 存储: %s -> %s\n", symbol, name)
	}

	// 获取数据
	value, err := layeredCache.Get(ctx, "stock:600000")
	if err != nil {
		log.Printf("获取缓存失败: %v", err)
	} else {
		fmt.Printf("   ✓ 获取: stock:600000 = %v\n", value)
	}

	// 显示基础统计
	stats := layeredCache.Stats()
	fmt.Printf("   统计: 大小=%d, 命中=%d, 未命中=%d, 命中率=%.2f%%\n",
		stats.Size, stats.HitCount, stats.MissCount, stats.HitRate*100)
}

// dataPromotionDemo 数据提升机制演示
func dataPromotionDemo(ctx context.Context) {
	// 创建启用数据提升的分层缓存
	config := cache.LayeredCacheConfig{
		Layers: []cache.LayerConfig{
			{
				Type:            cache.LayerMemory,
				MaxSize:         50, // 小的L1缓存
				TTL:             1 * time.Minute,
				Enabled:         true,
				Policy:          cache.PolicyLRU,
				CleanupInterval: 30 * time.Second,
			},
			{
				Type:            cache.LayerMemory, // 模拟L2缓存
				MaxSize:         200,               // 较大的L2缓存
				TTL:             10 * time.Minute,
				Enabled:         true,
				Policy:          cache.PolicyLFU,
				CleanupInterval: 1 * time.Minute,
			},
		},
		PromoteEnabled: true, // 启用数据提升
		WriteThrough:   false,
		WriteBack:      false,
	}

	layeredCache, err := cache.NewLayeredCache(config)
	if err != nil {
		log.Printf("创建分层缓存失败: %v", err)
		return
	}
	defer layeredCache.Close()

	fmt.Printf("   数据提升已启用，L1=%d容量, L2=%d容量\n",
		config.Layers[0].MaxSize, config.Layers[1].MaxSize)

	// 直接向L2添加数据（模拟L1已满的情况）
	testData := map[string]string{
		"promote:001": "提升测试数据1",
		"promote:002": "提升测试数据2",
		"promote:003": "提升测试数据3",
	}

	for key, value := range testData {
		err := layeredCache.Set(ctx, key, value, 0)
		if err != nil {
			log.Printf("设置缓存失败: %v", err)
			continue
		}
	}
	fmt.Printf("   ✓ 已存储%d条测试数据\n", len(testData))

	// 模拟数据提升：反复访问某个键
	fmt.Printf("   → 反复访问 promote:001 触发提升机制\n")
	for i := 0; i < 3; i++ {
		value, err := layeredCache.Get(ctx, "promote:001")
		if err != nil {
			log.Printf("获取缓存失败: %v", err)
		} else {
			fmt.Printf("   ✓ 第%d次访问成功: %v\n", i+1, value)
		}
		time.Sleep(10 * time.Millisecond) // 给提升协程时间执行
	}

	// 获取分层统计
	layerStats := layeredCache.GetLayerStats()
	fmt.Printf("   提升统计: 总提升次数=%d\n", layerStats.PromoteCount)
}

// writeModeDemo 写模式演示
func writeModeDemo(ctx context.Context) {
	fmt.Printf("   演示不同的写模式:\n")

	// 1. 默认模式（只写L1）
	fmt.Printf("\n   --- 默认写模式（只写L1） ---\n")
	config1 := cache.DefaultLayeredCacheConfig()
	config1.WriteThrough = false
	config1.WriteBack = false

	cache1, _ := cache.NewLayeredCache(config1)
	defer cache1.Close()

	cache1.Set(ctx, "write:default", "默认写模式", 0)
	fmt.Printf("   ✓ 数据写入L1层\n")

	// 2. 写穿透模式（写所有层）
	fmt.Printf("\n   --- 写穿透模式（写所有层） ---\n")
	config2 := cache.DefaultLayeredCacheConfig()
	config2.WriteThrough = true
	config2.WriteBack = false

	cache2, _ := cache.NewLayeredCache(config2)
	defer cache2.Close()

	cache2.Set(ctx, "write:through", "写穿透模式", 0)
	fmt.Printf("   ✓ 数据写入所有层\n")

	layerStats := cache2.GetLayerStats()
	fmt.Printf("   写穿透次数: %d\n", layerStats.WriteThrough)
}

// layeredStatsDemo 分层缓存统计演示
func layeredStatsDemo(ctx context.Context) {
	config := cache.DefaultLayeredCacheConfig()
	layeredCache, err := cache.NewLayeredCache(config)
	if err != nil {
		log.Printf("创建分层缓存失败: %v", err)
		return
	}
	defer layeredCache.Close()

	fmt.Printf("   详细统计信息演示:\n")

	// 添加测试数据
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("stats:%d", i)
		value := fmt.Sprintf("统计数据%d", i)
		layeredCache.Set(ctx, key, value, 0)
	}

	// 执行一些操作产生统计数据
	layeredCache.Get(ctx, "stats:1")    // 命中
	layeredCache.Get(ctx, "stats:2")    // 命中
	layeredCache.Get(ctx, "stats:999")  // 未命中
	layeredCache.Delete(ctx, "stats:3") // 删除

	// 获取整体统计
	fmt.Printf("\n   === 整体统计 ===\n")
	overallStats := layeredCache.Stats()
	fmt.Printf("   总大小: %d/%d\n", overallStats.Size, overallStats.MaxSize)
	fmt.Printf("   命中: %d, 未命中: %d, 命中率: %.2f%%\n",
		overallStats.HitCount, overallStats.MissCount, overallStats.HitRate*100)

	// 获取分层统计
	fmt.Printf("\n   === 分层统计 ===\n")
	layerStats := layeredCache.GetLayerStats()

	fmt.Printf("   各层统计:\n")
	for i, stats := range layerStats.LayerStats {
		fmt.Printf("   - L%d: 大小=%d/%d, 命中=%d, 未命中=%d, 命中率=%.2f%%\n",
			i+1, stats.Size, stats.MaxSize, stats.HitCount, stats.MissCount, stats.HitRate*100)
	}

	fmt.Printf("\n   运行时统计:\n")
	fmt.Printf("   - 总命中: %d\n", layerStats.TotalHits)
	fmt.Printf("   - 总未命中: %d\n", layerStats.TotalMisses)
	fmt.Printf("   - 提升次数: %d\n", layerStats.PromoteCount)
	fmt.Printf("   - 写穿透次数: %d\n", layerStats.WriteThrough)
	fmt.Printf("   - 写回次数: %d\n", layerStats.WriteBack)

	// 演示缓存预热
	fmt.Printf("\n   === 缓存预热演示 ===\n")
	warmData := map[string]interface{}{
		"warm:stock1": "预热股票1",
		"warm:stock2": "预热股票2",
		"warm:stock3": "预热股票3",
	}

	err = layeredCache.Warm(ctx, warmData)
	if err != nil {
		log.Printf("缓存预热失败: %v", err)
	} else {
		fmt.Printf("   ✓ 预热了%d条数据\n", len(warmData))
	}

	finalStats := layeredCache.Stats()
	fmt.Printf("   预热后总大小: %d\n", finalStats.Size)
}

// batchOperationsDemo 批量操作演示
func batchOperationsDemo(ctx context.Context) {
	fmt.Printf("\n   === 批量操作演示 ===\n")
	config := cache.DefaultLayeredCacheConfig()
	layeredCache, err := cache.NewLayeredCache(config)
	if err != nil {
		log.Printf("创建分层缓存失败: %v", err)
		return
	}
	defer layeredCache.Close()

	// 准备批量数据
	batchItems := map[string]interface{}{
		"batch:001": "批量数据1",
		"batch:002": "批量数据2",
		"batch:003": "批量数据3",
	}

	// 批量设置
	err = layeredCache.BatchSet(ctx, batchItems, 5*time.Minute)
	if err != nil {
		log.Printf("批量设置失败: %v", err)
	} else {
		fmt.Printf("   ✓ 成功批量设置 %d 条数据\n", len(batchItems))
	}

	// 批量获取
	keys := []string{"batch:001", "batch:003", "batch:nonexistent"}
	results, err := layeredCache.BatchGet(ctx, keys)
	if err != nil {
		log.Printf("批量获取失败: %v", err)
	} else {
		fmt.Printf("   ✓ 成功批量获取 %d 条数据:\n", len(results))
		for key, value := range results {
			fmt.Printf("     - %s -> %v\n", key, value)
		}
	}

	// 验证未命中的键
	if _, exists := results["batch:nonexistent"]; !exists {
		fmt.Printf("   ✓ 验证批量获取未命中(预期行为): batch:nonexistent\n")
	}
}
