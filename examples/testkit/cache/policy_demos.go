package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"stocksub/pkg/cache"
)

// lruDemo 演示LRU缓存策略
func lruDemo(ctx context.Context) {
	// 1. 配置缓存
	memConfig := cache.MemoryCacheConfig{
		MaxSize:         3,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	policyConfig := cache.PolicyConfig{
		Type:    cache.PolicyLRU,
		MaxSize: 3, // 关键：设置容量为3
		TTL:     5 * time.Minute,
	}

	// 2. 创建使用LRU策略的智能缓存
	smartCache := cache.NewSmartCache(memConfig, policyConfig)
	defer smartCache.Close()

	fmt.Printf("   创建LRU缓存，容量=3\n")

	// 3. 依次添加 A, B, C
	items := []struct{ key, value string }{
		{"A", "数据A"},
		{"B", "数据B"},
		{"C", "数据C"},
	}
	// 在函数作用域顶部声明err变量，避免作用域问题
	var err error
	for _, item := range items {
		// 使用 = 而不是 :=
		err = smartCache.Set(ctx, item.key, item.value, 0)
		if err != nil {
			log.Printf("设置缓存失败: %v", err)
			continue
		}
		fmt.Printf("   ✓ 添加: %s -> %s\n", item.key, item.value)
		time.Sleep(10 * time.Millisecond) // 确保添加顺序
	}

	// 4. 访问 A，使其变为最近使用的项
	smartCache.Get(ctx, "A")
	fmt.Printf("   → 访问 A，使其成为最近使用的项\n")

	// 5. 添加 D，此时容量已满(3)，应自动淘汰最久未使用的 B
	err = smartCache.Set(ctx, "D", "数据D", 0)
	if err != nil {
		log.Printf("设置缓存失败: %v", err)
	} else {
		fmt.Printf("   ✓ 添加: D -> 数据D (应淘汰最久未使用的B)\n")
	}

	// 6. 验证结果
	// B 应该被淘汰
	_, err = smartCache.Get(ctx, "B")
	if err != nil {
		fmt.Printf("   ✓ B 已被淘汰 (LRU策略生效)\n")
	} else {
		fmt.Printf("   ✗ B 仍然存在 (LRU策略未生效)\n")
	}

	// A, C, D 应该存在
	for _, key := range []string{"A", "C", "D"} {
		if val, err := smartCache.Get(ctx, key); err == nil {
			fmt.Printf("   ✓ %s 仍存在: %v\n", key, val)
		} else {
			fmt.Printf("   ✗ %s 已被淘汰 (错误)\n", key)
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
		testPolicy(ctx, p.policy)
	}
}

// testPolicy 测试特定策略的行为
func testPolicy(ctx context.Context, policyType cache.PolicyType) {
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
