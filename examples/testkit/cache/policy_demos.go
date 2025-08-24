package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"stocksub/pkg/testkit/cache"
)

// lruDemo 演示LRU缓存策略
func lruDemo(ctx context.Context) {
	// 创建使用LRU策略的内存缓存(简化版本)
	memConfig := cache.MemoryCacheConfig{
		MaxSize:         3, // 小容量便于观察淘汰效果
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	// 暂时使用普通内存缓存来演示基础操作
	memCache := cache.NewMemoryCache(memConfig)
	defer memCache.Close()

	fmt.Printf("   创建基础缓存，容量=3 (策略演示)\n")

	// 依次添加数据
	items := []struct{ key, value string }{
		{"A", "数据A"},
		{"B", "数据B"},
		{"C", "数据C"},
	}

	for _, item := range items {
		err := memCache.Set(ctx, item.key, item.value, 0)
		if err != nil {
			log.Printf("设置缓存失败: %v", err)
			continue
		}
		fmt.Printf("   ✓ 添加: %s -> %s\n", item.key, item.value)
	}

	// 模拟LRU行为：访问A和B
	memCache.Get(ctx, "A")
	memCache.Get(ctx, "B")
	fmt.Printf("   → 访问了A和B，模拟LRU行为\n")

	// 显示统计信息
	stats := memCache.Stats()
	fmt.Printf("   统计: 大小=%d, 命中=%d, 未命中=%d\n",
		stats.Size, stats.HitCount, stats.MissCount)

	// 清理演示
	memCache.Delete(ctx, "C")
	fmt.Printf("   ✓ 模拟淘汰最久未访问的C\n")

	// 验证A、B仍存在
	for _, key := range []string{"A", "B"} {
		if value, err := memCache.Get(ctx, key); err == nil {
			fmt.Printf("   ✓ %s仍存在: %v\n", key, value)
		}
	}
}

// lfuDemo 演示LFU缓存策略
func lfuDemo(ctx context.Context) {
	memConfig := cache.MemoryCacheConfig{
		MaxSize:         3,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	policyConfig := cache.PolicyConfig{
		Type:    cache.PolicyLFU,
		MaxSize: 3,
		TTL:     5 * time.Minute,
	}

	smartCache := cache.NewSmartCache(memConfig, policyConfig)
	defer smartCache.Close()

	fmt.Printf("   创建LFU缓存，容量=3\n")

	// 添加初始数据
	items := []struct{ key, value string }{
		{"X", "数据X"},
		{"Y", "数据Y"},
		{"Z", "数据Z"},
	}

	for _, item := range items {
		err := smartCache.Set(ctx, item.key, item.value, 0)
		if err != nil {
			log.Printf("设置缓存失败: %v", err)
			continue
		}
		fmt.Printf("   ✓ 添加: %s -> %s\n", item.key, item.value)
	}

	// 多次访问X和Y，让Z的访问频率最低
	for i := 0; i < 3; i++ {
		smartCache.Get(ctx, "X")
		smartCache.Get(ctx, "Y")
	}
	smartCache.Get(ctx, "Z") // Z只访问1次
	fmt.Printf("   → X和Y各访问3次，Z访问1次\n")

	// 添加新数据，应该淘汰访问频率最低的Z
	err := smartCache.Set(ctx, "W", "数据W", 0)
	if err != nil {
		log.Printf("设置缓存失败: %v", err)
	} else {
		fmt.Printf("   ✓ 添加: W -> 数据W (应该淘汰频率最低的Z)\n")
	}

	// 验证Z被淘汰
	_, err = smartCache.Get(ctx, "Z")
	if err != nil {
		fmt.Printf("   ✓ Z已被淘汰(LFU策略生效)\n")
	}
}

// fifoDemo 演示FIFO缓存策略
func fifoDemo(ctx context.Context) {
	memConfig := cache.MemoryCacheConfig{
		MaxSize:         3,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	policyConfig := cache.PolicyConfig{
		Type:    cache.PolicyFIFO,
		MaxSize: 3,
		TTL:     5 * time.Minute,
	}

	smartCache := cache.NewSmartCache(memConfig, policyConfig)
	defer smartCache.Close()

	fmt.Printf("   创建FIFO缓存，容量=3\n")

	// 按时间顺序添加数据
	items := []struct{ key, value string }{
		{"First", "第一个"},
		{"Second", "第二个"},
		{"Third", "第三个"},
	}

	for i, item := range items {
		err := smartCache.Set(ctx, item.key, item.value, 0)
		if err != nil {
			log.Printf("设置缓存失败: %v", err)
			continue
		}
		fmt.Printf("   ✓ 添加第%d个: %s -> %s\n", i+1, item.key, item.value)

		// 添加小延迟确保时间顺序
		time.Sleep(10 * time.Millisecond)
	}

	// 访问所有数据（FIFO不考虑访问频率）
	for _, item := range items {
		smartCache.Get(ctx, item.key)
	}
	fmt.Printf("   → 访问了所有数据（FIFO不考虑访问模式）\n")

	// 添加新数据，应该淘汰最先进入的First
	err := smartCache.Set(ctx, "Fourth", "第四个", 0)
	if err != nil {
		log.Printf("设置缓存失败: %v", err)
	} else {
		fmt.Printf("   ✓ 添加: Fourth -> 第四个 (应该淘汰最先进入的First)\n")
	}

	// 验证First被淘汰
	_, err = smartCache.Get(ctx, "First")
	if err != nil {
		fmt.Printf("   ✓ First已被淘汰(FIFO策略生效)\n")
	}
}

// policyComparisonDemo 对比不同策略的行为
func policyComparisonDemo(ctx context.Context) {
	fmt.Printf("   对比同样操作序列下不同策略的淘汰行为:\n")

	policies := []struct {
		name   string
		policy cache.PolicyType
	}{
		{"LRU", cache.PolicyLRU},
		{"LFU", cache.PolicyLFU},
		{"FIFO", cache.PolicyFIFO},
	}

	for _, p := range policies {
		fmt.Printf("\n   --- %s策略 ---\n", p.name)
		testPolicy(ctx, p.policy, p.name)
	}
}

// testPolicy 测试特定策略的行为
func testPolicy(ctx context.Context, policyType cache.PolicyType, name string) {
	memConfig := cache.MemoryCacheConfig{
		MaxSize:         2, // 非常小的容量
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	policyConfig := cache.PolicyConfig{
		Type:    policyType,
		MaxSize: 2,
		TTL:     5 * time.Minute,
	}

	smartCache := cache.NewSmartCache(memConfig, policyConfig)
	defer smartCache.Close()

	// 相同的操作序列
	fmt.Printf("     1. 添加A和B\n")
	smartCache.Set(ctx, "A", "数据A", 0)
	time.Sleep(5 * time.Millisecond) // FIFO需要时间差
	smartCache.Set(ctx, "B", "数据B", 0)

	fmt.Printf("     2. 访问A两次\n")
	smartCache.Get(ctx, "A")
	smartCache.Get(ctx, "A")

	fmt.Printf("     3. 添加C，观察谁被淘汰\n")
	smartCache.Set(ctx, "C", "数据C", 0)

	// 检查哪个被淘汰了
	_, errA := smartCache.Get(ctx, "A")
	_, errB := smartCache.Get(ctx, "B")

	if errA != nil {
		fmt.Printf("     → A被淘汰\n")
	} else if errB != nil {
		fmt.Printf("     → B被淘汰\n")
	}
}
