package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"stocksub/pkg/testkit/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDiskCache 统一创建测试环境
func setupTestDiskCache(t *testing.T, config DiskCacheConfig) (*DiskCache, string) {
	tempDir, err := os.MkdirTemp("", "disk_cache_test_*")
	require.NoError(t, err)

	if config.BaseDir == "" {
		config.BaseDir = tempDir
	}
	if config.FilePrefix == "" {
		config.FilePrefix = "test_cache"
	}
	// 默认设置长TTL，避免在非TTL测试中意外过期
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 1 * time.Hour
	}

	cache, err := NewDiskCache(config)
	require.NoError(t, err)

	// 使用 t.Cleanup 确保测试目录总是被清理
	t.Cleanup(func() {
		cache.Close()
		os.RemoveAll(tempDir)
	})

	return cache, tempDir
}

// TestDiskCache_BasicOperations 测试磁盘缓存基本操作
func TestDiskCache_BasicOperations(t *testing.T) {
	cache, _ := setupTestDiskCache(t, DiskCacheConfig{})
	ctx := context.Background()

	// 测试Set和Get
	err := cache.Set(ctx, "key1", "value1", 0) // 使用默认的长TTL
	assert.NoError(t, err)

	value, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)

	// 测试不存在的键
	_, err = cache.Get(ctx, "nonexistent")
	assert.Error(t, err)
	var testKitErr *core.TestKitError
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)

	// 测试Delete
	err = cache.Delete(ctx, "key1")
	assert.NoError(t, err)

	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)
}

// TestDiskCache_TTL 测试磁盘缓存的TTL功能
func TestDiskCache_TTL(t *testing.T) {
	cache, _ := setupTestDiskCache(t, DiskCacheConfig{})
	ctx := context.Background()

	// 设置短期有效的数据
	err := cache.Set(ctx, "key1", "value1", 50*time.Millisecond)
	assert.NoError(t, err)

	// 立即获取应该成功
	value, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)

	// 等待TTL过期
	time.Sleep(60 * time.Millisecond)

	// 再次获取应该失败（数据已过期）
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	var testKitErr *core.TestKitError
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)
	assert.Contains(t, testKitErr.Error(), "cache expired")
}

// TestDiskCache_Clear 测试磁盘缓存的Clear操作
func TestDiskCache_Clear(t *testing.T) {
	cache, _ := setupTestDiskCache(t, DiskCacheConfig{})
	ctx := context.Background()

	// 添加数据
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	// 验证数据存在
	assert.Equal(t, int64(2), cache.Stats().Size)

	// 清空缓存
	err := cache.Clear(ctx)
	assert.NoError(t, err)

	// 验证数据已被清除
	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.Size)
	assert.Equal(t, int64(0), stats.HitCount)
	assert.Equal(t, int64(0), stats.MissCount)
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
}

// TestDiskCache_Stats 测试磁盘缓存的统计信息
func TestDiskCache_Stats(t *testing.T) {
	cache, _ := setupTestDiskCache(t, DiskCacheConfig{})
	ctx := context.Background()

	// 添加数据
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	assert.Equal(t, int64(2), cache.Stats().Size)

	// 测试命中和未命中
	cache.Get(ctx, "key1") // hit
	cache.Get(ctx, "key3") // miss

	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.HitCount)
	assert.Equal(t, int64(1), stats.MissCount)
	assert.Equal(t, 0.5, stats.HitRate)
}

// TestDiskCache_Concurrency 简化并发测试以避免文件系统竞态
func TestDiskCache_Concurrency(t *testing.T) {
	cache, _ := setupTestDiskCache(t, DiskCacheConfig{MaxSize: 200})
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("key-%d-%d", gid, j)
				val := fmt.Sprintf("val-%d-%d", gid, j)
				err := cache.Set(ctx, key, val, 0)
				assert.NoError(t, err)
			}
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int64(numGoroutines*numOps), cache.Stats().Size)
}

// TestDiskCache_Close 测试磁盘缓存关闭操作
func TestDiskCache_Close(t *testing.T) {
	cache, _ := setupTestDiskCache(t, DiskCacheConfig{})
	ctx := context.Background()

	err := cache.Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err)

	// 关闭缓存
	err = cache.Close()
	assert.NoError(t, err)

	// 再次关闭应无影响
	err = cache.Close()
	assert.NoError(t, err)

	// 验证关闭后不能再操作
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	assert.Equal(t, "cache is closed", err.Error())
}

// TestDiskCache_CleanupWorker 测试后台清理协程
func TestDiskCache_CleanupWorker(t *testing.T) {
	cache, _ := setupTestDiskCache(t, DiskCacheConfig{
		CleanupInterval: 20 * time.Millisecond,
	})
	ctx := context.Background()

	err := cache.Set(ctx, "expiring_key", "temp_value", 10*time.Millisecond)
	require.NoError(t, err)
	err = cache.Set(ctx, "stable_key", "stable_value", time.Hour)
	require.NoError(t, err)

	assert.Equal(t, int64(2), cache.Stats().Size)

	// 等待足够长的时间以确保清理协程运行
	time.Sleep(50 * time.Millisecond)

	// 检查缓存内部状态
	cache.mu.RLock()
	_, exists := cache.entries["expiring_key"]
	cache.mu.RUnlock()

	assert.False(t, exists, "过期条目应被后台清理")
	assert.Equal(t, int64(1), cache.Stats().Size, "清理后大小应为1")
}

// TestDiskCache_MetadataLoad 测试元数据加载
func TestDiskCache_MetadataLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk_cache_metadata_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := DiskCacheConfig{
		BaseDir:    tempDir,
		FilePrefix: "meta_test",
		DefaultTTL: time.Minute,
	}

	// 第一次创建并添加数据
	cache1, err := NewDiskCache(config)
	require.NoError(t, err)
	ctx := context.Background()
	cache1.Set(ctx, "key1", "value1", time.Minute)
	cache1.Set(ctx, "key2", "value2", 10*time.Millisecond) // 这个会过期
	err = cache1.Close()
	require.NoError(t, err)

	// 等待 key2 过期
	time.Sleep(20 * time.Millisecond)

	// 第二次创建，应加载元数据
	cache2, err := NewDiskCache(config)
	require.NoError(t, err)
	defer cache2.Close()

	// 验证 key1 存在
	val, err := cache2.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", val)

	// 验证 key2 因过期未被加载
	_, err = cache2.Get(ctx, "key2")
	assert.Error(t, err)

	assert.Equal(t, int64(1), cache2.Stats().Size)
}

// TestDiskCache_MetadataLoad_Corrupted 测试加载损坏的元数据
func TestDiskCache_MetadataLoad_Corrupted(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk_cache_corrupted_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := DiskCacheConfig{
		BaseDir:    tempDir,
		FilePrefix: "corrupted_test",
	}
	cacheDir := filepath.Join(tempDir, config.FilePrefix)
	err = os.MkdirAll(cacheDir, 0755)
	require.NoError(t, err)

	// 手动写入一个损坏的元数据文件
	metadataFile := filepath.Join(cacheDir, "metadata.json")
	err = os.WriteFile(metadataFile, []byte("{not a valid json}"), 0644)
	require.NoError(t, err)

	// 在这个被污染的目录中创建缓存，应该会失败
	_, err = NewDiskCache(config)
	require.Error(t, err, "NewDiskCache should fail with corrupted metadata")
	assert.Contains(t, err.Error(), "反序列化元数据失败")
}

// TestDiskCache_Eviction 测试缓存淘汰
func TestDiskCache_Eviction(t *testing.T) {
	cache, _ := setupTestDiskCache(t, DiskCacheConfig{MaxSize: 2})
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", time.Hour)
	time.Sleep(5 * time.Millisecond) // 确保访问时间不同
	cache.Set(ctx, "key2", "value2", time.Hour)
	time.Sleep(5 * time.Millisecond)

	// 访问 key1，使其成为最近访问的
	cache.Get(ctx, "key1")

	// 添加 key3，应该会淘汰 key2 (最久未访问)
	cache.Set(ctx, "key3", "value3", time.Hour)

	assert.Equal(t, int64(2), cache.Stats().Size)
	_, err := cache.Get(ctx, "key2")
	assert.Error(t, err, "key2 should have been evicted")

	_, err = cache.Get(ctx, "key1")
	assert.NoError(t, err, "key1 should still exist")
	_, err = cache.Get(ctx, "key3")
	assert.NoError(t, err, "key3 should exist")
}
