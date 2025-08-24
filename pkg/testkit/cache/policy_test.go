package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// 测试智能缓存基本操作
func TestSmartCache_BasicOperations(t *testing.T) {
	memConfig := MemoryCacheConfig{
		MaxSize:         10,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	policyConfig := PolicyConfig{
		Type:    PolicyLRU,
		MaxSize: 3,
		TTL:     5 * time.Minute,
	}

	smartCache := NewSmartCache(memConfig, policyConfig)
	defer smartCache.Close()

	ctx := context.Background()

	// 测试基本的Set和Get操作（应该不会死锁）
	err := smartCache.Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err, "Set操作不应该出现死锁")

	value, err := smartCache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)
}

// 测试智能缓存LRU淘汰策略
func TestSmartCache_LRUEviction(t *testing.T) {
	memConfig := MemoryCacheConfig{
		MaxSize:         10,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	policyConfig := PolicyConfig{
		Type:    PolicyLRU,
		MaxSize: 3,
		TTL:     5 * time.Minute,
	}

	smartCache := NewSmartCache(memConfig, policyConfig)
	defer smartCache.Close()

	ctx := context.Background()

	// 添加3个条目（达到最大容量）
	smartCache.Set(ctx, "A", "dataA", 0)
	smartCache.Set(ctx, "B", "dataB", 0)
	smartCache.Set(ctx, "C", "dataC", 0)

	// 访问A和B，使C成为最久未访问的
	smartCache.Get(ctx, "A")
	smartCache.Get(ctx, "B")

	// 添加第4个条目，应该淘汰C
	err := smartCache.Set(ctx, "D", "dataD", 0)
	assert.NoError(t, err, "添加第4个条目不应该出现死锁")

	// 验证C被淘汰
	_, err = smartCache.Get(ctx, "C")
	assert.Error(t, err, "C应该被淘汰")

	// 验证A和B仍然存在
	valueA, err := smartCache.Get(ctx, "A")
	assert.NoError(t, err)
	assert.Equal(t, "dataA", valueA)

	valueB, err := smartCache.Get(ctx, "B")
	assert.NoError(t, err)
	assert.Equal(t, "dataB", valueB)
}

// 测试智能缓存LFU淘汰策略
func TestSmartCache_LFUEviction(t *testing.T) {
	memConfig := MemoryCacheConfig{
		MaxSize:         10,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	policyConfig := PolicyConfig{
		Type:    PolicyLFU,
		MaxSize: 3,
		TTL:     5 * time.Minute,
	}

	smartCache := NewSmartCache(memConfig, policyConfig)
	defer smartCache.Close()

	ctx := context.Background()

	// 添加3个条目
	smartCache.Set(ctx, "X", "dataX", 0)
	smartCache.Set(ctx, "Y", "dataY", 0)
	smartCache.Set(ctx, "Z", "dataZ", 0)

	// 多次访问X和Y，让Z的访问频率最低
	for i := 0; i < 3; i++ {
		smartCache.Get(ctx, "X")
		smartCache.Get(ctx, "Y")
	}
	smartCache.Get(ctx, "Z") // Z只访问1次

	// 添加新数据，应该淘汰访问频率最低的Z
	err := smartCache.Set(ctx, "W", "dataW", 0)
	assert.NoError(t, err, "LFU策略不应该出现死锁")

	// 验证Z被淘汰
	_, err = smartCache.Get(ctx, "Z")
	assert.Error(t, err, "Z应该被淘汰（LFU策略）")
}

// 测试智能缓存FIFO淘汰策略
func TestSmartCache_FIFOEviction(t *testing.T) {
	memConfig := MemoryCacheConfig{
		MaxSize:         10,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	policyConfig := PolicyConfig{
		Type:    PolicyFIFO,
		MaxSize: 3,
		TTL:     5 * time.Minute,
	}

	smartCache := NewSmartCache(memConfig, policyConfig)
	defer smartCache.Close()

	ctx := context.Background()

	// 按时间顺序添加数据
	smartCache.Set(ctx, "First", "第一个", 0)
	time.Sleep(10 * time.Millisecond) // 确保时间顺序
	smartCache.Set(ctx, "Second", "第二个", 0)
	time.Sleep(10 * time.Millisecond)
	smartCache.Set(ctx, "Third", "第三个", 0)

	// 访问所有数据（FIFO不考虑访问频率）
	smartCache.Get(ctx, "First")
	smartCache.Get(ctx, "Second")
	smartCache.Get(ctx, "Third")

	// 添加新数据，应该淘汰最先进入的First
	time.Sleep(10 * time.Millisecond)
	err := smartCache.Set(ctx, "Fourth", "第四个", 0)
	assert.NoError(t, err, "FIFO策略不应该出现死锁")

	// 验证First被淘汰
	_, err = smartCache.Get(ctx, "First")
	assert.Error(t, err, "First应该被淘汰（FIFO策略）")
}

// 测试智能缓存并发操作
func TestSmartCache_ConcurrentOperations(t *testing.T) {
	memConfig := MemoryCacheConfig{
		MaxSize:         10,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	policyConfig := PolicyConfig{
		Type:    PolicyLRU,
		MaxSize: 5,
		TTL:     5 * time.Minute,
	}

	smartCache := NewSmartCache(memConfig, policyConfig)
	defer smartCache.Close()

	ctx := context.Background()

	// 并发操作测试（检测死锁）
	done := make(chan bool, 2)

	// 并发写入
	go func() {
		for i := 0; i < 10; i++ {
			smartCache.Set(ctx, fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i), 0)
		}
		done <- true
	}()

	// 并发读取
	go func() {
		for i := 0; i < 10; i++ {
			smartCache.Get(ctx, fmt.Sprintf("key%d", i%5))
		}
		done <- true
	}()

	// 等待所有操作完成（如果有死锁，这里会超时）
	timeout := time.After(5 * time.Second)
	completed := 0
	for completed < 2 {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatal("并发操作超时，可能存在死锁")
		}
	}

	// 如果能到达这里，说明没有死锁
	t.Log("并发操作成功完成，无死锁")
}

// 测试各种缓存策略
func TestSmartCache_Policies(t *testing.T) {
	memConfig := MemoryCacheConfig{
		MaxSize:         10,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}

	ctx := context.Background()

	// 测试LRU策略
	policyConfig := PolicyConfig{
		Type:    PolicyLRU,
		MaxSize: 3,
		TTL:     5 * time.Minute,
	}

	smartCache := NewSmartCache(memConfig, policyConfig)
	defer smartCache.Close()

	// 添加数据并测试LRU行为
	smartCache.Set(ctx, "A", "dataA", 0)
	smartCache.Set(ctx, "B", "dataB", 0)
	smartCache.Set(ctx, "C", "dataC", 0)

	// 访问A和B，使C成为最久未访问的
	smartCache.Get(ctx, "A")
	smartCache.Get(ctx, "B")

	// 添加第4个条目，应该淘汰C
	err := smartCache.Set(ctx, "D", "dataD", 0)
	assert.NoError(t, err)

	// 验证C被淘汰
	_, err = smartCache.Get(ctx, "C")
	assert.Error(t, err)

	// 测试LFU策略
	policyConfig.Type = PolicyLFU
	smartCacheLFU := NewSmartCache(memConfig, policyConfig)
	defer smartCacheLFU.Close()

	smartCacheLFU.Set(ctx, "X", "dataX", 0)
	smartCacheLFU.Set(ctx, "Y", "dataY", 0)
	smartCacheLFU.Set(ctx, "Z", "dataZ", 0)

	// 多次访问X和Y，让Z的访问频率最低
	for i := 0; i < 3; i++ {
		smartCacheLFU.Get(ctx, "X")
		smartCacheLFU.Get(ctx, "Y")
	}
	smartCacheLFU.Get(ctx, "Z") // Z只访问1次

	// 添加新数据，应该淘汰访问频率最低的Z
	err = smartCacheLFU.Set(ctx, "W", "dataW", 0)
	assert.NoError(t, err)

	// 验证Z被淘汰
	_, err = smartCacheLFU.Get(ctx, "Z")
	assert.Error(t, err)

	// 测试FIFO策略
	policyConfig.Type = PolicyFIFO
	smartCacheFIFO := NewSmartCache(memConfig, policyConfig)
	defer smartCacheFIFO.Close()

	smartCacheFIFO.Set(ctx, "First", "第一个", 0)
	time.Sleep(10 * time.Millisecond)
	smartCacheFIFO.Set(ctx, "Second", "第二个", 0)
	time.Sleep(10 * time.Millisecond)
	smartCacheFIFO.Set(ctx, "Third", "第三个", 0)

	// 添加新数据，应该淘汰最先进入的First
	time.Sleep(10 * time.Millisecond)
	err = smartCacheFIFO.Set(ctx, "Fourth", "第四个", 0)
	assert.NoError(t, err)

	// 验证First被淘汰
	_, err = smartCacheFIFO.Get(ctx, "First")
	assert.Error(t, err)
}

// 测试策略配置
func TestPolicyConfig(t *testing.T) {
	// 测试LRU策略
	policyConfig := PolicyConfig{
		Type:    PolicyLRU,
		MaxSize: 100,
		TTL:     5 * time.Minute,
	}

	policy := NewEvictionPolicy(policyConfig.Type)
	assert.IsType(t, &LRUPolicy{}, policy)

	// 测试LFU策略
	policyConfig.Type = PolicyLFU
	policy = NewEvictionPolicy(policyConfig.Type)
	assert.IsType(t, &LFUPolicy{}, policy)

	// 测试FIFO策略
	policyConfig.Type = PolicyFIFO
	policy = NewEvictionPolicy(policyConfig.Type)
	assert.IsType(t, &FIFOPolicy{}, policy)

	// 测试默认策略
	policyConfig.Type = "unknown"
	policy = NewEvictionPolicy(policyConfig.Type)
	assert.IsType(t, &LRUPolicy{}, policy) // 默认是LRU
}

// 测试FIFO策略的OnAccess方法
func TestFIFOPolicy_OnAccess(t *testing.T) {
	// FIFO策略的OnAccess方法应该是空实现
	// 这个测试主要是为了提高代码覆盖率
	// 在实际使用中，FIFO策略不关心访问事件
	// 我们只需要确保这个方法存在且不会panic
	fifo := NewFIFOPolicy()
	fifo.OnAccess("test", nil)
}