package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"stocksub/pkg/testkit/core"
)

// DiskCacheConfig 磁盘缓存配置
type DiskCacheConfig struct {
	BaseDir         string        `yaml:"base_dir"`         // 缓存文件基础目录
	MaxSize         int64         `yaml:"max_size"`         // 最大缓存条目数
	DefaultTTL      time.Duration `yaml:"default_ttl"`      // 默认生存时间
	CleanupInterval time.Duration `yaml:"cleanup_interval"` // 清理间隔
	FilePrefix      string        `yaml:"file_prefix"`      // 缓存文件前缀
}

// DiskCache 磁盘缓存实现
type DiskCache struct {
	mu        sync.RWMutex
	config    DiskCacheConfig
	stats     core.CacheStats
	entries   map[string]diskCacheEntry
	cacheDir  string
	closeChan chan struct{}
	closed    bool // 缓存是否已关闭
}

// diskCacheEntry 磁盘缓存条目
// 包含内存中的元数据和指向磁盘文件的引用
type diskCacheEntry struct {
	Key        string    // 缓存键
	Filepath   string    // 磁盘文件路径
	ExpireTime time.Time // 过期时间
	AccessTime time.Time // 最后访问时间
	CreateTime time.Time // 创建时间
	HitCount   int64     // 命中次数
	Size       int64     // 数据大小（字节）
}

// NewDiskCache 创建磁盘缓存实例
func NewDiskCache(config DiskCacheConfig) (*DiskCache, error) {
	if config.BaseDir == "" {
		config.BaseDir = os.TempDir()
	}
	if config.FilePrefix == "" {
		config.FilePrefix = "stocksub_disk_cache"
	}

	cacheDir := filepath.Join(config.BaseDir, config.FilePrefix)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	dc := &DiskCache{
		config:    config,
		entries:   make(map[string]diskCacheEntry),
		cacheDir:  cacheDir,
		closeChan: make(chan struct{}),
		stats: core.CacheStats{
			MaxSize: config.MaxSize,
			TTL:     config.DefaultTTL,
		},
	}

	// 启动定期清理协程
	if config.CleanupInterval > 0 {
		go dc.cleanupWorker()
	}

	// 加载现有的缓存元数据
	if err := dc.loadMetadata(); err != nil {
		return nil, fmt.Errorf("加载缓存元数据失败: %w", err)
	}

	return dc, nil
}

// Get 从磁盘缓存获取数据
func (dc *DiskCache) Get(ctx context.Context, key string) (interface{}, error) {
	dc.mu.RLock()
	if dc.closed {
		dc.mu.RUnlock()
		return nil, fmt.Errorf("cache is closed")
	}
	entry, exists := dc.entries[key]
	dc.mu.RUnlock()

	if !exists {
		dc.stats.MissCount++
		return nil, core.NewTestKitError(core.ErrCacheMiss, "cache miss")
	}

	// 检查是否过期
	if time.Now().After(entry.ExpireTime) {
		dc.mu.Lock()
		delete(dc.entries, key)
		dc.stats.MissCount++
		dc.stats.Size--
		dc.mu.Unlock()
		
		// 异步删除磁盘文件
		go os.Remove(entry.Filepath)
		return nil, core.NewTestKitError(core.ErrCacheMiss, "cache expired")
	}

	// 从磁盘读取数据
	data, err := dc.readFromDisk(entry.Filepath)
	if err != nil {
		dc.stats.MissCount++
		return nil, fmt.Errorf("读取缓存数据失败: %w", err)
	}

	// 更新访问统计
	dc.mu.Lock()
	entry.AccessTime = time.Now()
	entry.HitCount++
	dc.entries[key] = entry
	dc.stats.HitCount++
	dc.mu.Unlock()

	return data, nil
}

// Set 向磁盘缓存设置数据
func (dc *DiskCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if dc.closed {
		return fmt.Errorf("cache is closed")
	}

	if ttl <= 0 {
		ttl = dc.config.DefaultTTL
	}

	expireTime := time.Now().Add(ttl)
	
	// 序列化数据
	dataBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化数据失败: %w", err)
	}

	// 生成唯一文件名
	filename := fmt.Sprintf("%s_%d.json", dc.config.FilePrefix, time.Now().UnixNano())
	filepath := filepath.Join(dc.cacheDir, filename)

	// 写入磁盘
	if err := dc.writeToDisk(filepath, dataBytes); err != nil {
		return fmt.Errorf("写入磁盘失败: %w", err)
	}

	// 检查是否超过最大大小
	if int64(len(dc.entries)) >= dc.config.MaxSize && dc.config.MaxSize > 0 {
		// 基于访问时间的淘汰策略：删除最久未访问的条目
		var oldestKey string
		var oldestTime time.Time
		for k, e := range dc.entries {
			if oldestTime.IsZero() || e.AccessTime.Before(oldestTime) {
				oldestKey = k
				oldestTime = e.AccessTime
			}
		}
		if oldestKey != "" {
			oldEntry := dc.entries[oldestKey]
			delete(dc.entries, oldestKey)
			dc.stats.Size--
			// 同步删除磁盘文件以避免死锁
			os.Remove(oldEntry.Filepath)
		}
	}

	// 添加新条目
	dc.entries[key] = diskCacheEntry{
		Key:        key,
		Filepath:   filepath,
		ExpireTime: expireTime,
		AccessTime: time.Now(),
		CreateTime: time.Now(),
		HitCount:   0,
		Size:       int64(len(dataBytes)),
	}
	dc.stats.Size++

	return nil
}

// Delete 从磁盘缓存删除数据
func (dc *DiskCache) Delete(ctx context.Context, key string) error {
	dc.mu.Lock()
	if dc.closed {
		dc.mu.Unlock()
		return fmt.Errorf("cache is closed")
	}
	entry, exists := dc.entries[key]
	if exists {
		delete(dc.entries, key)
		dc.stats.Size--
	}
	dc.mu.Unlock()

	if exists {
		// 异步删除磁盘文件
		go os.Remove(entry.Filepath)
	}

	return nil
}

// Clear 清空磁盘缓存
func (dc *DiskCache) Clear(ctx context.Context) error {
	dc.mu.Lock()
	if dc.closed {
		dc.mu.Unlock()
		return fmt.Errorf("cache is closed")
	}
	entries := make([]diskCacheEntry, 0, len(dc.entries))
	for _, entry := range dc.entries {
		entries = append(entries, entry)
	}
	dc.entries = make(map[string]diskCacheEntry)
	dc.stats.Size = 0
	dc.stats.HitCount = 0
	dc.stats.MissCount = 0
	dc.mu.Unlock()

	// 异步删除所有磁盘文件
	go func() {
		for _, entry := range entries {
			os.Remove(entry.Filepath)
		}
	}()

	return nil
}

// Stats 获取缓存统计信息
func (dc *DiskCache) Stats() core.CacheStats {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	stats := dc.stats
	stats.LastCleanup = time.Now()

	// 计算命中率
	total := stats.HitCount + stats.MissCount
	if total > 0 {
		stats.HitRate = float64(stats.HitCount) / float64(total)
	}

	return stats
}

// Close 关闭磁盘缓存
func (dc *DiskCache) Close() error {
	dc.mu.Lock()
	if dc.closed {
		dc.mu.Unlock()
		return nil // 已经关闭
	}
	dc.closed = true
	close(dc.closeChan)

	// 先解锁，再调用 saveMetadata，避免死锁
	dc.mu.Unlock()

	// 保存元数据
	if err := dc.saveMetadata(); err != nil {
		return fmt.Errorf("保存元数据失败: %w", err)
	}

	return nil
}

// readFromDisk 从磁盘读取数据
func (dc *DiskCache) readFromDisk(filepath string) (interface{}, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("反序列化数据失败: %w", err)
	}

	return value, nil
}

// writeToDisk 向磁盘写入数据
func (dc *DiskCache) writeToDisk(filepath string, data []byte) error {
	tempFile := filepath + ".tmp"
	
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	if err := os.Rename(tempFile, filepath); err != nil {
		return fmt.Errorf("重命名文件失败: %w", err)
	}

	return nil
}

// loadMetadata 加载缓存元数据
func (dc *DiskCache) loadMetadata() error {
	metadataFile := filepath.Join(dc.cacheDir, "metadata.json")
	if _, err := os.Stat(metadataFile); os.IsNotExist(err) {
		return nil // 元数据文件不存在是正常的
	}

	data, err := os.ReadFile(metadataFile)
	if err != nil {
		return fmt.Errorf("读取元数据文件失败: %w", err)
	}

	var entries map[string]diskCacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("反序列化元数据失败: %w", err)
	}

	// 过滤掉过期的条目
	now := time.Now()
	validEntries := make(map[string]diskCacheEntry)
	for key, entry := range entries {
		if now.Before(entry.ExpireTime) {
			validEntries[key] = entry
		} else {
			// 异步删除过期的磁盘文件
			go os.Remove(entry.Filepath)
		}
	}

	dc.mu.Lock()
	dc.entries = validEntries
	dc.stats.Size = int64(len(validEntries))
	dc.mu.Unlock()

	return nil
}

// saveMetadata 保存缓存元数据
func (dc *DiskCache) saveMetadata() error {
	dc.mu.RLock()
	entries := dc.entries
	dc.mu.RUnlock()

	data, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("序列化元数据失败: %w", err)
	}

	metadataFile := filepath.Join(dc.cacheDir, "metadata.json")
	tempFile := metadataFile + ".tmp"

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("写入元数据临时文件失败: %w", err)
	}

	if err := os.Rename(tempFile, metadataFile); err != nil {
		return fmt.Errorf("重命名元数据文件失败: %w", err)
	}

	return nil
}

// cleanupWorker 定期清理过期条目的工作协程
func (dc *DiskCache) cleanupWorker() {
	ticker := time.NewTicker(dc.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dc.cleanupExpired()
		case <-dc.closeChan:
			return
		}
	}
}

// cleanupExpired 清理过期条目
func (dc *DiskCache) cleanupExpired() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	now := time.Now()
	deletedKeys := make([]string, 0)
	deletedFiles := make([]string, 0)

	for key, entry := range dc.entries {
		if now.After(entry.ExpireTime) {
			deletedKeys = append(deletedKeys, key)
			deletedFiles = append(deletedFiles, entry.Filepath)
		}
	}

	for _, key := range deletedKeys {
		delete(dc.entries, key)
		dc.stats.Size--
	}

	// 异步删除磁盘文件
	if len(deletedFiles) > 0 {
		go func(files []string) {
			for _, file := range files {
				os.Remove(file)
			}
		}(deletedFiles)
	}

	dc.stats.LastCleanup = now
}

var _ core.Cache = (*DiskCache)(nil)