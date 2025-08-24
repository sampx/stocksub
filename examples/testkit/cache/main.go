package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"stocksub/pkg/testkit/cache"
	"stocksub/pkg/testkit/core"
)

func main() {
	fmt.Println("=== TestKit Cache 模块完整示例 ===")
	fmt.Println("请选择要运行的示例:")
	fmt.Println("1. 基础缓存操作")
	fmt.Println("2. 缓存策略演示")
	fmt.Println("3. 分层缓存演示")
	fmt.Println("4. 高级功能演示")
	fmt.Println("5. 运行所有示例")
	fmt.Print("请输入选择 (1-5): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		fmt.Println("无效输入，运行所有示例...")
		choice = 5
	}

	ctx := context.Background()

	switch choice {
	case 1:
		runBasicDemo(ctx)
	case 2:
		runPolicyDemo(ctx)
	case 3:
		runLayeredDemo(ctx)
	case 4:
		runAdvancedDemo(ctx)
	case 5:
		runAllDemos(ctx)
	default:
		fmt.Println("无效选择，运行所有示例...")
		runAllDemos(ctx)
	}

	fmt.Println("\n=== 示例运行完成 ===")
}

// runBasicDemo 运行基础缓存演示
func runBasicDemo(ctx context.Context) {
	fmt.Println("\n=== 基础缓存操作演示 ===")
	fmt.Println("\n1. 基础内存缓存演示")
	basicCacheDemo(ctx)
	fmt.Println("\n2. 带TTL的缓存演示")
	ttlCacheDemo(ctx)
	fmt.Println("\n3. 缓存统计信息演示")
	cacheStatsDemo(ctx)
	fmt.Println("\n4. 缓存清理和关闭演示")
	cacheCleanupDemo(ctx)
}

// runPolicyDemo 运行策略演示
func runPolicyDemo(ctx context.Context) {
	fmt.Println("\n=== 缓存策略演示 ===")
	fmt.Println("\n1. LRU (Least Recently Used) 策略演示")
	lruDemo(ctx)
	fmt.Println("\n2. LFU (Least Frequently Used) 策略演示")
	lfuDemo(ctx)
	fmt.Println("\n3. FIFO (First In First Out) 策略演示")
	fifoDemo(ctx)
	fmt.Println("\n4. 策略对比演示")
	policyComparisonDemo(ctx)
}

// runLayeredDemo 运行分层缓存演示
func runLayeredDemo(ctx context.Context) {
	fmt.Println("\n=== 分层缓存演示 ===")
	fmt.Println("\n1. 基础分层缓存演示")
	basicLayeredCacheDemo(ctx)
	fmt.Println("\n2. 数据提升机制演示")
	dataPromotionDemo(ctx)
	fmt.Println("\n3. 写模式演示")
	writeModeDemo(ctx)
	fmt.Println("\n4. 分层缓存统计演示")
	layeredStatsDemo(ctx)
}

// runAdvancedDemo 运行高级功能演示
func runAdvancedDemo(ctx context.Context) {
	fmt.Println("\n=== 高级功能演示 ===")
	fmt.Println("\n1. 并发安全性演示")
	concurrencyDemo(ctx)
	fmt.Println("\n2. 错误处理演示")
	errorHandlingDemo(ctx)
	fmt.Println("\n3. 性能测试演示")
	performanceDemo(ctx)
	fmt.Println("\n4. 实际应用场景演示")
	realWorldScenarioDemo(ctx)
}

// runAllDemos 运行所有演示
func runAllDemos(ctx context.Context) {
	runBasicDemo(ctx)
	runPolicyDemo(ctx)
	runLayeredDemo(ctx)
	runAdvancedDemo(ctx)
}

// ============= 基础缓存演示函数 =============

// basicCacheDemo 展示基础的缓存操作
func basicCacheDemo(ctx context.Context) {
	// 创建内存缓存配置
	config := cache.MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	// 创建内存缓存
	memCache := cache.NewMemoryCache(config)
	defer memCache.Close()

	// Set 操作：存储键值对
	err := memCache.Set(ctx, "stock:600000", "浦发银行", 0)
	if err != nil {
		log.Printf("设置缓存失败: %v", err)
		return
	}
	fmt.Printf("   ✓ 已存储: stock:600000 -> 浦发银行\n")

	err = memCache.Set(ctx, "stock:000001", "平安银行", 0)
	if err != nil {
		log.Printf("设置缓存失败: %v", err)
		return
	}
	fmt.Printf("   ✓ 已存储: stock:000001 -> 平安银行\n")

	// Get 操作：获取缓存值
	value, err := memCache.Get(ctx, "stock:600000")
	if err != nil {
		log.Printf("获取缓存失败: %v", err)
		return
	}
	fmt.Printf("   ✓ 获取成功: stock:600000 = %v\n", value)

	// 获取不存在的键
	_, err = memCache.Get(ctx, "stock:nonexistent")
	if err != nil {
		if testKitErr, ok := err.(*core.TestKitError); ok && testKitErr.Code == core.ErrCacheMiss {
			fmt.Printf("   ✓ 缓存未命中(预期行为): stock:nonexistent\n")
		}
	}

	// Delete 操作：删除缓存
	err = memCache.Delete(ctx, "stock:600000")
	if err != nil {
		log.Printf("删除缓存失败: %v", err)
		return
	}
	fmt.Printf("   ✓ 已删除: stock:600000\n")

	// 验证删除结果
	_, err = memCache.Get(ctx, "stock:600000")
	if err != nil {
		fmt.Printf("   ✓ 删除验证成功: stock:600000 已不存在\n")
	}
}

// ttlCacheDemo 展示TTL(过期时间)功能
func ttlCacheDemo(ctx context.Context) {
	config := cache.MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      500 * time.Millisecond, // 短TTL便于演示
		CleanupInterval: 100 * time.Millisecond,
	}

	memCache := cache.NewMemoryCache(config)
	defer memCache.Close()

	// 设置带TTL的缓存
	err := memCache.Set(ctx, "temp:data1", "临时数据1", 300*time.Millisecond)
	if err != nil {
		log.Printf("设置缓存失败: %v", err)
		return
	}
	fmt.Printf("   ✓ 已存储临时数据，TTL=300ms\n")

	// 立即获取，应该成功
	value, err := memCache.Get(ctx, "temp:data1")
	if err != nil {
		log.Printf("获取缓存失败: %v", err)
		return
	}
	fmt.Printf("   ✓ 立即获取成功: %v\n", value)

	// 等待超过TTL时间
	fmt.Printf("   → 等待400ms让缓存过期...\n")
	time.Sleep(400 * time.Millisecond)

	// 再次获取，应该失败
	_, err = memCache.Get(ctx, "temp:data1")
	if err != nil {
		fmt.Printf("   ✓ 过期后获取失败(预期行为): %v\n", err)
	}

	// 使用默认TTL
	err = memCache.Set(ctx, "default:ttl", "使用默认TTL", 0)
	if err != nil {
		log.Printf("设置缓存失败: %v", err)
		return
	}
	fmt.Printf("   ✓ 已存储数据，使用默认TTL(500ms)\n")
}

// cacheStatsDemo 展示缓存统计功能
func cacheStatsDemo(ctx context.Context) {
	config := cache.MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	memCache := cache.NewMemoryCache(config)
	defer memCache.Close()

	// 初始统计
	stats := memCache.Stats()
	fmt.Printf("   初始统计 - 大小:%d, 命中:%d, 未命中:%d, 命中率:%.2f%%\n",
		stats.Size, stats.HitCount, stats.MissCount, stats.HitRate*100)

	// 添加一些数据
	symbols := []string{"600000", "000001", "000002"}
	for _, symbol := range symbols {
		err := memCache.Set(ctx, "stock:"+symbol, "股票"+symbol, 0)
		if err != nil {
			log.Printf("设置缓存失败: %v", err)
			continue
		}
	}

	stats = memCache.Stats()
	fmt.Printf("   添加数据后 - 大小:%d, 最大容量:%d\n", stats.Size, stats.MaxSize)

	// 执行一些Get操作来产生命中和未命中
	for i := 0; i < 3; i++ {
		memCache.Get(ctx, "stock:600000") // 命中
	}
	for i := 0; i < 2; i++ {
		memCache.Get(ctx, "stock:999999") // 未命中
	}

	// 查看最终统计
	stats = memCache.Stats()
	fmt.Printf("   最终统计 - 大小:%d, 命中:%d, 未命中:%d, 命中率:%.2f%%\n",
		stats.Size, stats.HitCount, stats.MissCount, stats.HitRate*100)
	fmt.Printf("   TTL:%v, 最后清理:%v\n", stats.TTL, stats.LastCleanup.Format("15:04:05"))
}

// cacheCleanupDemo 展示缓存清理和资源管理
func cacheCleanupDemo(ctx context.Context) {
	config := cache.MemoryCacheConfig{
		MaxSize:         5, // 小容量便于演示淘汰机制
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	memCache := cache.NewMemoryCache(config)
	defer memCache.Close()

	// 填充缓存到最大容量
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("stock:%d", i)
		value := fmt.Sprintf("股票%d", i)
		err := memCache.Set(ctx, key, value, 0)
		if err != nil {
			log.Printf("设置缓存失败: %v", err)
		}
	}

	stats := memCache.Stats()
	fmt.Printf("   填充后容量: %d/%d\n", stats.Size, stats.MaxSize)

	// 继续添加数据，触发淘汰机制
	err := memCache.Set(ctx, "stock:new", "新股票", 0)
	if err != nil {
		log.Printf("设置缓存失败: %v", err)
	} else {
		fmt.Printf("   ✓ 添加新数据，触发淘汰机制\n")
	}

	stats = memCache.Stats()
	fmt.Printf("   淘汰后容量: %d/%d\n", stats.Size, stats.MaxSize)

	// 清空缓存
	err = memCache.Clear(ctx)
	if err != nil {
		log.Printf("清空缓存失败: %v", err)
	} else {
		fmt.Printf("   ✓ 已清空缓存\n")
	}

	stats = memCache.Stats()
	fmt.Printf("   清空后统计 - 大小:%d, 命中:%d, 未命中:%d\n",
		stats.Size, stats.HitCount, stats.MissCount)
}
