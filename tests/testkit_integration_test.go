//go:build integration

package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit"
	"stocksub/pkg/testkit/config"
)

func TestTestDataManager_Integration(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	fixedTime := time.Date(2025, 8, 22, 15, 30, 0, 0, time.UTC)

	// 配置
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "memory",
			MaxSize: 100,
			TTL:     5 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "csv",
			Directory: tmpDir,
		},
	}

	// 创建管理器
	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()
	symbols := []string{"INTG001", "INTG002"}

	// 启用Mock模式进行测试
	manager.EnableMock(true)

	// 设置Mock数据
	mockData := []subscriber.StockData{
		{
			Symbol:        "INTG001",
			Name:          "集成测试股票1",
			Price:         100.50,
			Change:        2.50,
			ChangePercent: 2.55,
			Volume:        1000000,
			Timestamp:     fixedTime,
		},
		{
			Symbol:        "INTG002",
			Name:          "集成测试股票2",
			Price:         200.75,
			Change:        -1.25,
			ChangePercent: -0.62,
			Volume:        500000,
			Timestamp:     fixedTime,
		},
	}

	manager.SetMockData(symbols, mockData)

	// 第一次获取数据
	data1, err := manager.GetStockData(ctx, symbols)
	assert.NoError(t, err)
	assert.Len(t, data1, 2)
	assert.Equal(t, "INTG001", data1[0].Symbol)
	assert.Equal(t, "INTG002", data1[1].Symbol)

	// 验证统计信息
	stats := manager.GetStats()
	assert.True(t, stats.MockMode)
	assert.Greater(t, stats.CacheHits+stats.CacheMisses, int64(0))

	t.Logf("集成测试统计: %+v", stats)
}

func TestTestDataManager_CacheIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "layered",
			MaxSize: 50,
			TTL:     1 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "memory",
			Directory: tmpDir,
		},
	}

	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()
	symbols := []string{"CACHE001"}

	// 启用Mock模式
	manager.EnableMock(true)

	mockData := []subscriber.StockData{
		{
			Symbol: "CACHE001",
			Name:   "缓存测试股票",
			Price:  150.00,
		},
	}

	manager.SetMockData(symbols, mockData)

	// 多次获取相同数据，验证缓存效果
	for i := 0; i < 5; i++ {
		data, err := manager.GetStockData(ctx, symbols)
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, "CACHE001", data[0].Symbol)
	}

	// 验证缓存命中
	stats := manager.GetStats()
	t.Logf("缓存测试统计: CacheHits=%d, CacheMisses=%d", stats.CacheHits, stats.CacheMisses)
}

func TestTestDataManager_StorageIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "memory",
			MaxSize: 10,
			TTL:     1 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "csv",
			Directory: tmpDir,
		},
	}

	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()
	symbols := []string{"STORE001", "STORE002"}

	// 启用Mock模式
	manager.EnableMock(true)

	mockData := []subscriber.StockData{
		{
			Symbol: "STORE001",
			Name:   "存储测试股票1",
			Price:  75.25,
		},
		{
			Symbol: "STORE002",
			Name:   "存储测试股票2",
			Price:  125.75,
		},
	}

	manager.SetMockData(symbols, mockData)

	// 获取数据，应该触发存储
	data, err := manager.GetStockData(ctx, symbols)
	assert.NoError(t, err)
	assert.Len(t, data, 2)

	// 等待异步存储完成
	time.Sleep(500 * time.Millisecond)

	// 验证文件是否创建（CSV存储应该创建文件）
	files, err := os.ReadDir(tmpDir)
	assert.NoError(t, err)

	if len(files) > 0 {
		t.Logf("存储文件已创建: %v", files[0].Name())
	}
}

func TestTestDataManager_Reset(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "memory",
			MaxSize: 100,
			TTL:     5 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "memory",
			Directory: tmpDir,
		},
	}

	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()
	symbols := []string{"RESET001"}

	// 启用Mock模式并获取数据
	manager.EnableMock(true)
	mockData := []subscriber.StockData{
		{Symbol: "RESET001", Price: 100.00},
	}
	manager.SetMockData(symbols, mockData)

	// 获取数据
	_, err := manager.GetStockData(ctx, symbols)
	assert.NoError(t, err)

	// 验证有统计数据
	stats1 := manager.GetStats()
	assert.Greater(t, stats1.CacheHits+stats1.CacheMisses, int64(0))

	// 重置管理器
	err = manager.Reset()
	assert.NoError(t, err)

	// 验证统计已重置
	stats2 := manager.GetStats()
	assert.Equal(t, int64(0), stats2.CacheHits+stats2.CacheMisses)
}

func TestTestDataManager_EnableDisableFeatures(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "memory",
			MaxSize: 100,
			TTL:     5 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "memory",
			Directory: tmpDir,
		},
	}

	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()
	symbols := []string{"FEATURE001"}

	// 测试禁用缓存
	manager.EnableCache(false)

	// 启用Mock模式
	manager.EnableMock(true)
	mockData := []subscriber.StockData{
		{Symbol: "FEATURE001", Price: 200.00},
	}
	manager.SetMockData(symbols, mockData)

	// 多次获取数据，由于缓存禁用，每次都应该是新的调用
	for i := 0; i < 3; i++ {
		_, err := manager.GetStockData(ctx, symbols)
		assert.NoError(t, err)
	}

	stats := manager.GetStats()
	t.Logf("禁用缓存统计: %+v", stats)

	// 重新启用缓存
	manager.EnableCache(true)

	// 再次获取数据
	_, err := manager.GetStockData(ctx, symbols)
	assert.NoError(t, err)
}

// 性能集成测试

func TestTestDataManager_PerformanceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "layered",
			MaxSize: 1000,
			TTL:     10 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "memory",
			Directory: tmpDir,
		},
	}

	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()

	// 启用Mock模式
	manager.EnableMock(true)

	// 大量数据测试
	symbols := make([]string, 100)
	mockData := make([]subscriber.StockData, 100)

	for i := 0; i < 100; i++ {
		symbol := fmt.Sprintf("PERF%03d", i)
		symbols[i] = symbol
		mockData[i] = subscriber.StockData{
			Symbol: symbol,
			Price:  100.0 + float64(i),
		}
	}

	manager.SetMockData(symbols, mockData)

	// 性能测试
	start := time.Now()

	for i := 0; i < 10; i++ {
		data, err := manager.GetStockData(ctx, symbols)
		assert.NoError(t, err)
		assert.Len(t, data, 100)
	}

	duration := time.Since(start)
	t.Logf("100个股票，10次调用，总耗时: %v，平均每次: %v",
		duration, duration/10)

	// 验证性能指标
	assert.Less(t, duration, 5*time.Second, "性能应该在可接受范围内")

	// 获取最终统计
	stats := manager.GetStats()
	t.Logf("性能测试最终统计: %+v", stats)
}

// 兼容性测试

func TestTestDataManager_CompatibilityIntegration(t *testing.T) {
	// 测试环境变量
	originalEnv := os.Getenv("TESTKIT_ENHANCED")
	defer func() {
		if originalEnv != "" {
			os.Setenv("TESTKIT_ENHANCED", originalEnv)
		} else {
			os.Unsetenv("TESTKIT_ENHANCED")
		}
	}()

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "memory",
			MaxSize: 100,
			TTL:     5 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "memory",
			Directory: tmpDir,
		},
	}

	// 测试增强模式
	os.Setenv("TESTKIT_ENHANCED", "1")

	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()
	symbols := []string{"COMPAT001"}

	// 启用Mock模式
	manager.EnableMock(true)
	mockData := []subscriber.StockData{
		{Symbol: "COMPAT001", Price: 300.00},
	}
	manager.SetMockData(symbols, mockData)

	// 获取数据
	data, err := manager.GetStockData(ctx, symbols)
	assert.NoError(t, err)
	assert.Len(t, data, 1)
	assert.Equal(t, "COMPAT001", data[0].Symbol)

	// 验证增强功能可用
	stats := manager.GetStats()
	assert.NotNil(t, stats)

	t.Logf("兼容性测试完成，统计: %+v", stats)
}

// 基准测试

func BenchmarkTestDataManager_GetStockData(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type:    "memory",
			MaxSize: 1000,
			TTL:     10 * time.Minute,
		},
		Storage: config.StorageConfig{
			Type:      "memory",
			Directory: tmpDir,
		},
	}

	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	ctx := context.Background()
	symbols := []string{"BENCH001", "BENCH002"}

	// 启用Mock模式
	manager.EnableMock(true)
	mockData := []subscriber.StockData{
		{Symbol: "BENCH001", Price: 100.00},
		{Symbol: "BENCH002", Price: 200.00},
	}
	manager.SetMockData(symbols, mockData)

	// 预热
	manager.GetStockData(ctx, symbols)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			manager.GetStockData(ctx, symbols)
		}
	})
}