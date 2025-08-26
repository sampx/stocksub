package cache

import (
	"context"
	"stocksub/pkg/testkit/core"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLRUPolicy(t *testing.T) {
	policy := NewEvictionPolicy(PolicyLRU)
	entries := make(map[string]*core.CacheEntry)

	entry1 := &core.CacheEntry{CreateTime: time.Now(), AccessTime: time.Now()}
	time.Sleep(1 * time.Millisecond)
	entry2 := &core.CacheEntry{CreateTime: time.Now(), AccessTime: time.Now()}
	time.Sleep(1 * time.Millisecond)
	entry3 := &core.CacheEntry{CreateTime: time.Now(), AccessTime: time.Now()}

	entries["1"] = entry1
	policy.OnAdd("1", entry1)
	entries["2"] = entry2
	policy.OnAdd("2", entry2)
	entries["3"] = entry3
	policy.OnAdd("3", entry3)

	// 访问 entry1, 模拟SmartCache行为，更新AccessTime
	entry1.AccessTime = time.Now()
	policy.OnAccess("1", entry1)

	// 淘汰时，应该淘汰 entry2 (最久未访问)
	toEvict := policy.ShouldEvict(entries)
	assert.Contains(t, toEvict, "2")
}

func TestLFUPolicy(t *testing.T) {
	policy := NewEvictionPolicy(PolicyLFU)
	entries := make(map[string]*core.CacheEntry)

	entry1 := &core.CacheEntry{}
	entry2 := &core.CacheEntry{}
	entry3 := &core.CacheEntry{}

	entries["1"] = entry1
	policy.OnAdd("1", entry1)
	entries["2"] = entry2
	policy.OnAdd("2", entry2)
	entries["3"] = entry3
	policy.OnAdd("3", entry3)

	// 访问 entry1 和 entry2, 模拟SmartCache行为，更新HitCount
	entry1.HitCount += 2
	policy.OnAccess("1", entry1)
	policy.OnAccess("1", entry1)
	entry2.HitCount++
	policy.OnAccess("2", entry2)

	// 淘汰时，应该淘汰 entry3 (访问频率最低, HitCount=0)
	toEvict := policy.ShouldEvict(entries)
	assert.Contains(t, toEvict, "3")
}

func TestFIFOPolicy(t *testing.T) {
	policy := NewEvictionPolicy(PolicyFIFO)
	entries := make(map[string]*core.CacheEntry)

	entry1 := &core.CacheEntry{CreateTime: time.Now()}
	time.Sleep(1 * time.Millisecond)
	entry2 := &core.CacheEntry{CreateTime: time.Now()}
	time.Sleep(1 * time.Millisecond)
	entry3 := &core.CacheEntry{CreateTime: time.Now()}

	entries["1"] = entry1
	policy.OnAdd("1", entry1)
	entries["2"] = entry2
	policy.OnAdd("2", entry2)
	entries["3"] = entry3
	policy.OnAdd("3", entry3)

	// 访问 entry2, 不应该影响淘汰顺序
	policy.OnAccess("2", entry2)

	// 淘汰时，应该淘汰 entry1 (最早添加的)
	toEvict := policy.ShouldEvict(entries)
	assert.Contains(t, toEvict, "1")
}

func TestFIFOPolicy_OnAccess(t *testing.T) {
	policy := NewEvictionPolicy(PolicyFIFO)
	entry := &core.CacheEntry{}
	policy.OnAdd("1", entry)

	assert.NotPanics(t, func() {
		policy.OnAccess("1", entry)
	})
}

func TestSmartCache(t *testing.T) {
	ctx := context.Background()
	memConfig := MemoryCacheConfig{
		MaxSize:    3,
		DefaultTTL: time.Minute,
	}
	policyConfig := PolicyConfig{
		Type:    PolicyLRU,
		MaxSize: 3,
	}
	cache := NewSmartCache(memConfig, policyConfig)

	cache.Set(ctx, "key1", "value1", 0)
	time.Sleep(time.Millisecond) // 确保时间戳不同
	cache.Set(ctx, "key2", "value2", 0)
	time.Sleep(time.Millisecond)
	cache.Set(ctx, "key3", "value3", 0)

	// 访问 key1
	cache.Get(ctx, "key1")

	// 添加第4个条目，应该会淘汰 key2 (最久未访问)
	cache.Set(ctx, "key4", "value4", 0)

	_, err := cache.Get(ctx, "key2")
	assert.Error(t, err)
}
