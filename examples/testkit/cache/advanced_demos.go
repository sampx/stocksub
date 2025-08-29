package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"stocksub/pkg/cache"
	"stocksub/pkg/testkit"
)

// concurrencyDemo 并发安全性演示
func concurrencyDemo(ctx context.Context) {
	config := cache.MemoryCacheConfig{
		MaxSize:         1000,
		DefaultTTL:      1 * time.Minute,
		CleanupInterval: 10 * time.Second,
	}

	memCache := cache.NewMemoryCache(config)
	defer memCache.Close()

	fmt.Printf("   启动10个并发goroutine进行读写操作...\n")

	var wg sync.WaitGroup
	numGoroutines := 10
	operationsPerGoroutine := 100

	// 并发写入
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				key := fmt.Sprintf("concurrent:%d:%d", goroutineID, j)
				value := fmt.Sprintf("Value-%d-%d", goroutineID, j)

				err := memCache.Set(ctx, key, value, 0)
				if err != nil {
					log.Printf("Goroutine %d 设置失败: %v", goroutineID, err)
				}
			}
		}(i)
	}

	wg.Wait()
	fmt.Printf("   ✓ 并发写入完成\n")

	// 并发读取
	wg.Add(numGoroutines)
	var readSuccessCount int64
	var readMutex sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				key := fmt.Sprintf("concurrent:%d:%d", goroutineID, j)

				_, err := memCache.Get(ctx, key)
				if err == nil {
					readMutex.Lock()
					readSuccessCount++
					readMutex.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	stats := memCache.Stats()
	fmt.Printf("   ✓ 并发测试结果:\n")
	fmt.Printf("     - 缓存大小: %d\n", stats.Size)
	fmt.Printf("     - 成功读取: %d\n", readSuccessCount)
	fmt.Printf("     - 总命中: %d, 总未命中: %d\n", stats.HitCount, stats.MissCount)
}

// errorHandlingDemo 错误处理演示
func errorHandlingDemo(ctx context.Context) {
	config := cache.MemoryCacheConfig{
		MaxSize:         5,                      // 小容量触发错误情况
		DefaultTTL:      100 * time.Millisecond, // 短TTL
		CleanupInterval: 50 * time.Millisecond,
	}

	memCache := cache.NewMemoryCache(config)
	defer memCache.Close()

	fmt.Printf("   演示各种错误情况的处理:\n")

	// 1. 缓存未命中错误
	fmt.Printf("\n   1. 缓存未命中错误:\n")
	_, err := memCache.Get(ctx, "nonexistent-key")
	if err != nil {
		if testKitErr, ok := err.(*testkit.TestKitError); ok {
			fmt.Printf("     ✓ 捕获到TestKit错误: %s - %s\n", testKitErr.Code, testKitErr.Message)
		}
	}

	// 2. TTL过期错误
	fmt.Printf("\n   2. TTL过期演示:\n")
	err = memCache.Set(ctx, "ttl-test", "过期测试", 80*time.Millisecond)
	if err != nil {
		log.Printf("设置失败: %v", err)
	} else {
		fmt.Printf("     ✓ 设置带TTL的数据\n")
	}

	// 立即获取成功
	value, err := memCache.Get(ctx, "ttl-test")
	if err == nil {
		fmt.Printf("     ✓ 立即获取成功: %v\n", value)
	}

	// 等待过期
	time.Sleep(100 * time.Millisecond)
	_, err = memCache.Get(ctx, "ttl-test")
	if err != nil {
		fmt.Printf("     ✓ TTL过期后获取失败(预期)\n")
	}

	// 3. 容量限制演示
	fmt.Printf("\n   3. 容量限制演示:\n")
	for i := 0; i < 10; i++ { // 超过最大容量5
		key := fmt.Sprintf("capacity-test:%d", i)
		value := fmt.Sprintf("数据%d", i)

		err := memCache.Set(ctx, key, value, 0)
		if err != nil {
			fmt.Printf("     ✗ 设置%s失败: %v\n", key, err)
		} else if i < 5 {
			fmt.Printf("     ✓ 设置%s成功\n", key)
		} else {
			fmt.Printf("     ✓ 设置%s成功(淘汰旧数据)\n", key)
		}
	}

	stats := memCache.Stats()
	fmt.Printf("     最终缓存大小: %d/%d\n", stats.Size, stats.MaxSize)
}

// performanceDemo 性能测试演示
func performanceDemo(ctx context.Context) {
	fmt.Printf("   性能基准测试:\n")

	// 测试不同缓存大小的性能
	sizes := []int64{100, 1000, 10000}

	for _, size := range sizes {
		fmt.Printf("\n   --- 缓存容量: %d ---\n", size)

		config := cache.MemoryCacheConfig{
			MaxSize:         size,
			DefaultTTL:      5 * time.Minute,
			CleanupInterval: 1 * time.Minute,
		}

		memCache := cache.NewMemoryCache(config)

		// 写入性能测试
		writeStart := time.Now()
		for i := int64(0); i < size/2; i++ {
			key := fmt.Sprintf("perf:write:%d", i)
			value := fmt.Sprintf("性能测试数据%d", i)
			memCache.Set(ctx, key, value, 0)
		}
		writeDuration := time.Since(writeStart)
		writeOpsPerSec := float64(size/2) / writeDuration.Seconds()

		fmt.Printf("     写入性能: %d ops, 耗时=%v, 速度=%.0f ops/sec\n",
			size/2, writeDuration, writeOpsPerSec)

		// 读取性能测试
		readStart := time.Now()
		for i := int64(0); i < size/2; i++ {
			key := fmt.Sprintf("perf:write:%d", i)
			memCache.Get(ctx, key)
		}
		readDuration := time.Since(readStart)
		readOpsPerSec := float64(size/2) / readDuration.Seconds()

		fmt.Printf("     读取性能: %d ops, 耗时=%v, 速度=%.0f ops/sec\n",
			size/2, readDuration, readOpsPerSec)

		stats := memCache.Stats()
		fmt.Printf("     命中率: %.2f%%\n", stats.HitRate*100)

		memCache.Close()
	}
}

// realWorldScenarioDemo 实际应用场景演示
func realWorldScenarioDemo(ctx context.Context) {
	fmt.Printf("   模拟股票数据缓存应用场景:\n")

	// 创建股票数据缓存
	stockCacheConfig := cache.MemoryCacheConfig{
		MaxSize:         1000,
		DefaultTTL:      30 * time.Second, // 股票数据30秒过期
		CleanupInterval: 10 * time.Second,
	}

	stockCache := cache.NewMemoryCache(stockCacheConfig)
	defer stockCache.Close()

	// 模拟股票数据结构
	type StockInfo struct {
		Symbol string  `json:"symbol"`
		Name   string  `json:"name"`
		Price  float64 `json:"price"`
		Volume int64   `json:"volume"`
		Time   string  `json:"time"`
	}

	// 1. 批量加载股票数据
	fmt.Printf("\n   1. 模拟批量加载股票数据:\n")
	stocks := []StockInfo{
		{"600000", "浦发银行", 12.34, 1000000, time.Now().Format("15:04:05")},
		{"000001", "平安银行", 15.67, 800000, time.Now().Format("15:04:05")},
		{"600036", "招商银行", 45.23, 1200000, time.Now().Format("15:04:05")},
		{"601318", "中国平安", 67.89, 900000, time.Now().Format("15:04:05")},
		{"000002", "万科A", 23.45, 1500000, time.Now().Format("15:04:05")},
	}

	for _, stock := range stocks {
		key := fmt.Sprintf("stock_info:%s", stock.Symbol)
		err := stockCache.Set(ctx, key, stock, 0)
		if err != nil {
			log.Printf("缓存股票数据失败: %v", err)
		} else {
			fmt.Printf("     ✓ 缓存: %s - %s (¥%.2f)\n",
				stock.Symbol, stock.Name, stock.Price)
		}
	}

	// 2. 模拟查询操作
	fmt.Printf("\n   2. 模拟股票查询操作:\n")
	querySymbols := []string{"600000", "000001", "999999"} // 包含不存在的

	for _, symbol := range querySymbols {
		key := fmt.Sprintf("stock_info:%s", symbol)
		value, err := stockCache.Get(ctx, key)

		if err != nil {
			fmt.Printf("     ✗ %s: 数据不存在或已过期\n", symbol)
		} else {
			stock := value.(StockInfo)
			fmt.Printf("     ✓ %s: %s - ¥%.2f (成交量:%d)\n",
				stock.Symbol, stock.Name, stock.Price, stock.Volume)
		}
	}

	// 3. 模拟热点数据访问
	fmt.Printf("\n   3. 模拟热点股票频繁查询:\n")
	hotStock := "600000" // 热点股票
	hotKey := fmt.Sprintf("stock_info:%s", hotStock)

	for i := 0; i < 5; i++ {
		value, err := stockCache.Get(ctx, hotKey)
		if err == nil {
			stock := value.(StockInfo)
			fmt.Printf("     ✓ 热点查询%d: %s - ¥%.2f\n", i+1, stock.Name, stock.Price)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 4. 显示最终统计
	fmt.Printf("\n   4. 股票缓存统计报告:\n")
	stats := stockCache.Stats()
	fmt.Printf("     - 缓存的股票数量: %d\n", stats.Size)
	fmt.Printf("     - 查询命中次数: %d\n", stats.HitCount)
	fmt.Printf("     - 查询未命中次数: %d\n", stats.MissCount)
	fmt.Printf("     - 缓存命中率: %.2f%%\n", stats.HitRate*100)
	fmt.Printf("     - 数据TTL: %v\n", stats.TTL)

	// 5. 模拟数据更新
	fmt.Printf("\n   5. 模拟股票价格更新:\n")
	updateStock := StockInfo{
		Symbol: "600000",
		Name:   "浦发银行",
		Price:  12.56, // 价格上涨
		Volume: 1100000,
		Time:   time.Now().Format("15:04:05"),
	}

	err := stockCache.Set(ctx, "stock_info:600000", updateStock, 0)
	if err != nil {
		log.Printf("更新股票数据失败: %v", err)
	} else {
		fmt.Printf("     ✓ 更新股票价格: %s ¥%.2f -> ¥%.2f\n",
			updateStock.Name, 12.34, updateStock.Price)
	}

	// 验证更新
	value, _ := stockCache.Get(ctx, "stock_info:600000")
	updatedStock := value.(StockInfo)
	fmt.Printf("     ✓ 验证更新成功: %s 当前价格 ¥%.2f\n",
		updatedStock.Name, updatedStock.Price)
}
