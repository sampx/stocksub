package cache

import (
	"container/list"
	"context"
	"sync"
	"time"

	"stocksub/pkg/testkit/core"
)

// PolicyType 淘汰策略类型
type PolicyType string

const (
	PolicyLRU  PolicyType = "lru"  // Least Recently Used
	PolicyLFU  PolicyType = "lfu"  // Least Frequently Used
	PolicyFIFO PolicyType = "fifo" // First In First Out
)

// EvictionPolicy 缓存淘汰策略
type EvictionPolicy interface {
	ShouldEvict(entries map[string]*core.CacheEntry) []string
	OnAccess(key string, entry *core.CacheEntry)
	OnAdd(key string, entry *core.CacheEntry)
	OnRemove(key string, entry *core.CacheEntry)
}

// NewEvictionPolicy 创建淘汰策略
func NewEvictionPolicy(policyType PolicyType) EvictionPolicy {
	switch policyType {
	case PolicyLRU:
		return newLRUPolicy()
	case PolicyLFU:
		return newLFUPolicy()
	case PolicyFIFO:
		return newFIFOPolicy()
	default:
		return newLRUPolicy() // 默认使用LRU
	}
}

// lruPolicy LRU淘汰策略
type lruPolicy struct {
	mu       sync.Mutex
	lruList  *list.List
	lruIndex map[string]*list.Element
}

// lruEntry LRU条目
type lruEntry struct {
	Key        string
	AccessTime time.Time
}

// newLRUPolicy 创建LRU策略
func newLRUPolicy() *lruPolicy {
	return &lruPolicy{
		lruList:  list.New(),
		lruIndex: make(map[string]*list.Element),
	}
}

// ShouldEvict 确定应该淘汰的键
func (lru *lruPolicy) ShouldEvict(entries map[string]*core.CacheEntry) []string {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if len(entries) == 0 {
		return nil
	}

	var oldestKey string
	var oldestTime time.Time

	for key, entry := range entries {
		if oldestKey == "" || entry.AccessTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessTime
		}
	}

	if oldestKey != "" {
		return []string{oldestKey}
	}

	return nil
}

// OnAccess 访问时的回调
func (lru *lruPolicy) OnAccess(key string, entry *core.CacheEntry) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if elem, exists := lru.lruIndex[key]; exists {
		lru.lruList.MoveToFront(elem)
		elem.Value.(*lruEntry).AccessTime = time.Now()
	}
}

// OnAdd 添加时的回调
func (lru *lruPolicy) OnAdd(key string, entry *core.CacheEntry) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	lruEntry := &lruEntry{
		Key:        key,
		AccessTime: time.Now(),
	}

	elem := lru.lruList.PushFront(lruEntry)
	lru.lruIndex[key] = elem
}

// OnRemove 移除时的回调
func (lru *lruPolicy) OnRemove(key string, entry *core.CacheEntry) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if elem, exists := lru.lruIndex[key]; exists {
		lru.lruList.Remove(elem)
		delete(lru.lruIndex, key)
	}
}

// lfuPolicy LFU淘汰策略
type lfuPolicy struct {
	mu          sync.Mutex
	frequencies map[string]int64
}

// newLFUPolicy 创建LFU策略
func newLFUPolicy() *lfuPolicy {
	return &lfuPolicy{
		frequencies: make(map[string]int64),
	}
}

// ShouldEvict 确定应该淘汰的键
func (lfu *lfuPolicy) ShouldEvict(entries map[string]*core.CacheEntry) []string {
	lfu.mu.Lock()
	defer lfu.mu.Unlock()

	if len(entries) == 0 {
		return nil
	}

	var minFreq int64 = -1
	var evictKey string

	for key, entry := range entries {
		freq := entry.HitCount
		if minFreq == -1 || freq < minFreq {
			minFreq = freq
			evictKey = key
		}
	}

	if evictKey != "" {
		return []string{evictKey}
	}

	return nil
}

// OnAccess 访问时的回调
func (lfu *lfuPolicy) OnAccess(key string, entry *core.CacheEntry) {
	lfu.mu.Lock()
	defer lfu.mu.Unlock()

	lfu.frequencies[key]++
}

// OnAdd 添加时的回调
func (lfu *lfuPolicy) OnAdd(key string, entry *core.CacheEntry) {
	lfu.mu.Lock()
	defer lfu.mu.Unlock()

	lfu.frequencies[key] = 1
}

// OnRemove 移除时的回调
func (lfu *lfuPolicy) OnRemove(key string, entry *core.CacheEntry) {
	lfu.mu.Lock()
	defer lfu.mu.Unlock()

	delete(lfu.frequencies, key)
}

// fifoPolicy FIFO淘汰策略
type fifoPolicy struct {
	mu    sync.Mutex
	queue *list.List
	index map[string]*list.Element
}

// fifoEntry FIFO条目
type fifoEntry struct {
	Key        string
	CreateTime time.Time
}

// newFIFOPolicy 创建FIFO策略
func newFIFOPolicy() *fifoPolicy {
	return &fifoPolicy{
		queue: list.New(),
		index: make(map[string]*list.Element),
	}
}

// ShouldEvict 确定应该淘汰的键
func (fifo *fifoPolicy) ShouldEvict(entries map[string]*core.CacheEntry) []string {
	fifo.mu.Lock()
	defer fifo.mu.Unlock()

	if len(entries) == 0 {
		return nil
	}

	var oldestKey string
	var oldestTime time.Time

	for key, entry := range entries {
		if oldestKey == "" || entry.CreateTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreateTime
		}
	}

	if oldestKey != "" {
		return []string{oldestKey}
	}

	return nil
}

// OnAccess 访问时的回调（FIFO不需要处理访问）
func (fifo *fifoPolicy) OnAccess(key string, entry *core.CacheEntry) {
	// FIFO策略不需要处理访问事件
}

// OnAdd 添加时的回调
func (fifo *fifoPolicy) OnAdd(key string, entry *core.CacheEntry) {
	fifo.mu.Lock()
	defer fifo.mu.Unlock()

	fifoEntry := &fifoEntry{
		Key:        key,
		CreateTime: time.Now(),
	}

	elem := fifo.queue.PushBack(fifoEntry)
	fifo.index[key] = elem
}

// OnRemove 移除时的回调
func (fifo *fifoPolicy) OnRemove(key string, entry *core.CacheEntry) {
	fifo.mu.Lock()
	defer fifo.mu.Unlock()

	if elem, exists := fifo.index[key]; exists {
		fifo.queue.Remove(elem)
		delete(fifo.index, key)
	}
}

// PolicyConfig 策略配置
type PolicyConfig struct {
	Type    PolicyType    `yaml:"type"`
	MaxSize int64         `yaml:"max_size"`
	TTL     time.Duration `yaml:"ttl"`
}

// SmartCache 智能缓存，支持策略配置
type SmartCache struct {
	*MemoryCache
	policy  EvictionPolicy
	maxSize int64
}

// NewSmartCache 创建智能缓存
func NewSmartCache(config MemoryCacheConfig, policyConfig PolicyConfig) *SmartCache {
	baseCache := NewMemoryCache(config)
	policy := NewEvictionPolicy(policyConfig.Type)

	return &SmartCache{
		MemoryCache: baseCache,
		policy:      policy,
		maxSize:     policyConfig.MaxSize,
	}
}

// Set 重写Set方法，集成淘汰策略
func (sc *SmartCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = sc.defaultTTL
	}

	now := time.Now()
	entry := &core.CacheEntry{
		Value:      value,
		ExpireTime: now.Add(ttl),
		AccessTime: now,
		CreateTime: now,
		HitCount:   0,
		Size:       estimateSize(value),
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	// 如果达到最大容量，执行淘汰策略
	if int64(len(sc.entries)) >= sc.maxSize {
		toEvict := sc.policy.ShouldEvict(sc.entries)
		for _, evictKey := range toEvict {
			if existingEntry, exists := sc.entries[evictKey]; exists {
				sc.policy.OnRemove(evictKey, existingEntry)
				delete(sc.entries, evictKey)
			}
		}
	}

	// 直接设置条目，避免调用基类方法造成双重加锁
	sc.entries[key] = entry

	// 通知策略新增了条目
	sc.policy.OnAdd(key, entry)

	return nil
}

// Get 重写Get方法，集成访问通知
func (sc *SmartCache) Get(ctx context.Context, key string) (interface{}, error) {
	value, err := sc.MemoryCache.Get(ctx, key)

	if err == nil {
		sc.mu.RLock()
		if entry, exists := sc.entries[key]; exists {
			sc.policy.OnAccess(key, entry)
		}
		sc.mu.RUnlock()
	}

	return value, err
}
