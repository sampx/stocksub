package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"stocksub/pkg/testkit/core"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试磁盘缓存基本操作
func TestDiskCache_BasicOperations(t *testing.T) {
	// 创建临时目录用于测试
	tempDir, err := os.MkdirTemp("", "disk_cache_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := DiskCacheConfig{
		BaseDir:         tempDir,
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 0, // 禁用清理协程，避免测试卡住
		FilePrefix:      "test",
	}

	cache, err := NewDiskCache(config)
	assert.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// 测试Set和Get
	err = cache.Set(ctx, "key1", "value1", 0)
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

// 测试磁盘缓存的TTL功能
func TestDiskCache_TTL(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk_cache_ttl_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := DiskCacheConfig{
		BaseDir:         tempDir,
		MaxSize:         100,
		DefaultTTL:      100 * time.Millisecond, // 短TTL用于测试
		CleanupInterval: 0, // 禁用清理协程，避免测试卡住
		FilePrefix:      "test",
	}

	cache, err := NewDiskCache(config)
	assert.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// 设置短期有效的数据
	err = cache.Set(ctx, "key1", "value1", 100*time.Millisecond)
	assert.NoError(t, err)

	// 立即获取应该成功
	value, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)

	// 等待TTL过期
	time.Sleep(150 * time.Millisecond)

	// 再次获取应该失败（数据已过期）
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	var testKitErr *core.TestKitError
	assert.ErrorAs(t, err, &testKitErr)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)
}

// 测试磁盘缓存的Clear操作
func TestDiskCache_Clear(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk_cache_clear_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := DiskCacheConfig{
		BaseDir:         tempDir,
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 0, // 禁用清理协程，避免测试卡住
		FilePrefix:      "test",
	}

	cache, err := NewDiskCache(config)
	assert.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// 添加数据
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	// 验证数据存在
	_, err = cache.Get(ctx, "key1")
	assert.NoError(t, err)
	_, err = cache.Get(ctx, "key2")
	assert.NoError(t, err)

	// 清空缓存
	err = cache.Clear(ctx)
	assert.NoError(t, err)

	// 验证数据已被清除
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	_, err = cache.Get(ctx, "key2")
	assert.Error(t, err)
}

// 测试磁盘缓存的统计信息
func TestDiskCache_Stats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk_cache_stats_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := DiskCacheConfig{
		BaseDir:         tempDir,
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 0, // 禁用清理协程，避免测试卡住
		FilePrefix:      "test",
	}

	cache, err := NewDiskCache(config)
	assert.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// 初始统计
	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.Size)
	assert.Equal(t, int64(0), stats.HitCount)
	assert.Equal(t, int64(0), stats.MissCount)

	// 添加数据
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	// 获取更新后的统计信息
	stats = cache.Stats()
	assert.Equal(t, int64(2), stats.Size)

	// 测试命中和未命中
	cache.Get(ctx, "key1") // hit
	cache.Get(ctx, "key3") // miss

	stats = cache.Stats()
	assert.Equal(t, int64(1), stats.HitCount)
	assert.Equal(t, int64(1), stats.MissCount)
}

// 测试磁盘缓存的并发安全性
func TestDiskCache_Concurrency(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk_cache_concurrency_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := DiskCacheConfig{
		BaseDir:         tempDir,
		MaxSize:         1000,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 0, // 禁用清理协程，避免测试卡住
		FilePrefix:      "test",
	}

	cache, err := NewDiskCache(config)
	assert.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// 并发设置和获取
	numGoroutines := 5
	numOperations := 20
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", goroutineID, j)
				value := fmt.Sprintf("value_%d_%d", goroutineID, j)
				
				// 并发设置
				err := cache.Set(ctx, key, value, 0)
				assert.NoError(t, err)
				
				// 并发获取
				retrieved, err := cache.Get(ctx, key)
				assert.NoError(t, err)
				assert.Equal(t, value, retrieved)
			}
		}(i)
	}

	wg.Wait()

	// 验证所有数据都正确存储
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numOperations; j++ {
			key := fmt.Sprintf("key_%d_%d", i, j)
			expected := fmt.Sprintf("value_%d_%d", i, j)
			
			value, err := cache.Get(ctx, key)
			assert.NoError(t, err)
			assert.Equal(t, expected, value)
		}
	}
}

// 测试磁盘缓存关闭操作
func TestDiskCache_Close(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk_cache_close_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := DiskCacheConfig{
		BaseDir:         tempDir,
		MaxSize:         100,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 0, // 禁用清理协程，避免测试卡住
		FilePrefix:      "test",
	}

	cache, err := NewDiskCache(config)
	assert.NoError(t, err)

	ctx := context.Background()

	// 添加一些数据
	err = cache.Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err)

	// 关闭缓存
	err = cache.Close()
	assert.NoError(t, err)

	// 验证关闭后不能再操作
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	
	err = cache.Set(ctx, "key2", "value2", 0)
	assert.Error(t, err)
}

// 测试磁盘缓存的文件清理功能
func TestDiskCache_Cleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk_cache_cleanup_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := DiskCacheConfig{
		BaseDir:         tempDir,
		MaxSize:         100,
		DefaultTTL:      100 * time.Millisecond, // 短TTL
		CleanupInterval: 0, // 禁用清理协程，避免测试卡住  // 频繁清理
		FilePrefix:      "test",
	}

	cache, err := NewDiskCache(config)
	assert.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// 添加会过期的数据
	err = cache.Set(ctx, "expiring_key", "temp_value", 100*time.Millisecond)
	assert.NoError(t, err)

	// 等待清理周期
	time.Sleep(200 * time.Millisecond)

	// 验证过期数据已被清理
	_, err = cache.Get(ctx, "expiring_key")
	assert.Error(t, err)

	// 检查磁盘文件是否也被清理
	files, err := filepath.Glob(filepath.Join(tempDir, "*.data"))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(files), "过期数据文件应该被清理")
}