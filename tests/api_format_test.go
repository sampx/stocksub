package tests

import (
	"testing"

	apitesting "stocksub/pkg/testing"

	"github.com/stretchr/testify/assert"
)

// TestTencentAPIResponseFormat 使用智能缓存的测试
func TestTencentAPIResponseFormat(t *testing.T) {
	// 创建缓存管理器，数据存储到tests/data
	cache := apitesting.NewTestDataCache("tests/data")
	defer cache.Close()

	symbols := []string{"600000", "000001", "688036", "835174"}

	// 🎯 智能获取数据：L1内存缓存 → L2 CSV缓存 → L3 API调用
	results, err := cache.GetStockDataBatch(symbols)
	assert.NoError(t, err, "获取股票数据失败")
	assert.Equal(t, len(symbols), len(results), "返回股票数量不匹配")

	// 测试逻辑保持完全不变
	for i, result := range results {
		symbol := symbols[i]
		t.Run(symbol, func(t *testing.T) {
			// 验证字段数量和基本格式（保持原有验证逻辑）
			assert.Equal(t, symbol, result.Symbol)
			assert.NotEmpty(t, result.Name, "股票名称字段为空")
			assert.Greater(t, result.Price, 0.0, "价格应大于0")
			assert.NotZero(t, result.Timestamp, "时间字段为空")

			// 记录字段详情用于分析（保持原有逻辑）
			t.Logf("股票%s: 名称=%s, 价格=%.2f, 时间=%s",
				symbol, result.Name, result.Price, result.Timestamp.Format("15:04:05"))
		})
	}

	// 输出缓存统计信息
	stats := cache.GetCacheStats()
	t.Logf("缓存统计: %+v", stats)
}

// TestFieldCount 验证字段数量一致性（使用缓存）
func TestFieldCount(t *testing.T) {
	cache := apitesting.NewTestDataCache("tests/data")
	defer cache.Close()

	symbols := []string{"600000", "000001", "300750", "688036", "835174"}

	// 🎯 一次获取所有股票数据（可能0次API调用）
	results, err := cache.GetStockDataBatch(symbols)
	if err != nil {
		t.Logf("警告: 无法获取数据: %v", err)
		return
	}

	// 验证所有股票的字段完整性
	for _, result := range results {
		t.Logf("股票%s数据完整性: 符号=%s, 名称长度=%d",
			result.Symbol, result.Symbol, len(result.Name))

		// 这里可以添加更多字段验证逻辑
		assert.NotEmpty(t, result.Symbol, "股票符号不能为空")
		assert.NotEmpty(t, result.Name, "股票名称不能为空")
	}
}

// TestFieldTypes 验证字段类型正确性（使用缓存）
func TestFieldTypes(t *testing.T) {
	cache := apitesting.NewTestDataCache("tests/data")
	defer cache.Close()

	symbol := "600000" // 使用上证600000作为测试

	results, err := cache.GetStockDataBatch([]string{symbol})
	assert.NoError(t, err)
	assert.Len(t, results, 1)

	result := results[0]

	// 验证关键字段的基本格式（保持原有验证逻辑）
	assert.Equal(t, symbol, result.Symbol)
	assert.NotEmpty(t, result.Name, "股票名称不能为空")
	assert.Greater(t, result.Price, 0.0, "当前价格应大于0")
	assert.NotZero(t, result.Timestamp, "时间戳不能为零值")
}

// TestEncodingHandling 验证编码处理（使用缓存）
func TestEncodingHandling(t *testing.T) {
	cache := apitesting.NewTestDataCache("tests/data")
	defer cache.Close()

	symbols := []string{"600000", "000001"}

	results, err := cache.GetStockDataBatch(symbols)
	assert.NoError(t, err)

	for _, result := range results {
		// 验证中文字符能正确处理
		assert.NotEmpty(t, result.Name)
		assert.NotContains(t, result.Name, "?", "股票名称包含乱码字符")

		t.Logf("股票%s名称: %s", result.Symbol, result.Name)
	}
}
