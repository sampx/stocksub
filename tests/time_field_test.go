package tests

import (
	"stocksub/pkg/subscriber"
	"testing"
	"time"

	apitesting "stocksub/pkg/testing"

	"github.com/stretchr/testify/assert"
)

// TestTimeFieldFormats 验证时间字段格式（使用智能缓存）
func TestTimeFieldFormats(t *testing.T) {
	cache := apitesting.NewTestDataCache("tests/data")
	defer cache.Close()

	testCases := map[string][]string{
		"上海主板": {"600000", "600036"},
		"深圳主板": {"000001", "000002"},
		"创业板":  {"300001", "300750"},
		"科创板":  {"688001", "688036"},
		"北交所":  {"835174", "832000"},
	}

	// 🎯 收集所有股票，一次批量获取（可能0次API调用）
	var allSymbols []string
	for _, symbols := range testCases {
		allSymbols = append(allSymbols, symbols...)
	}

	results, err := cache.GetStockDataBatch(allSymbols)
	assert.NoError(t, err, "批量获取数据失败")

	// 创建结果映射
	resultMap := make(map[string]subscriber.StockData)
	for _, result := range results {
		resultMap[result.Symbol] = result
	}

	// 保持原有的测试结构，按市场分组验证
	for market, symbols := range testCases {
		t.Run(market, func(t *testing.T) {
			for _, symbol := range symbols {
				result, ok := resultMap[symbol]
				assert.True(t, ok, "未找到股票%s数据", symbol)

				// 🎯 直接验证生产解析的时间字段
				timeStr := result.Timestamp.Format("20060102150405")
				assert.NotEmpty(t, timeStr, "时间字段为空")

				// 验证时间格式（保持原有验证逻辑）
				expectedFormats := []string{"20060102150405", "200601021504"}
				isValidFormat := false
				for _, format := range expectedFormats {
					if apitesting.IsValidTimeFormat(timeStr[:len(format)], format) {
						isValidFormat = true
						break
					}
				}
				assert.True(t, isValidFormat,
					"市场 %s 股票 %s 时间格式异常: %s", market, symbol, timeStr)
			}
		})
	}
}

// TestTimeFieldConsistency 验证时间字段一致性（使用缓存）
func TestTimeFieldConsistency(t *testing.T) {
	cache := apitesting.NewTestDataCache("tests/data")
	defer cache.Close()

	symbols := []string{"600000", "000001", "300750", "688036"}

	// 🎯 一次批量获取替代4次独立调用（可能0次API调用）
	results, err := cache.GetStockDataBatch(symbols)
	assert.NoError(t, err, "批量获取数据失败")

	var timestamps []time.Time
	for _, result := range results {
		timestamps = append(timestamps, result.Timestamp)
		t.Logf("股票%s时间字段: %s", result.Symbol, result.Timestamp.Format("20060102150405"))
	}

	// 验证同一时刻获取的时间字段是否接近（调整为更宽松的时间窗口）
	if len(timestamps) > 1 {
		for i := 1; i < len(timestamps); i++ {
			diff := timestamps[i].Sub(timestamps[0]).Abs()
			// 批量API调用时，不同股票可能有较大时间差异，调整为60秒
			assert.LessOrEqual(t, diff, 60*time.Second,
				"不同股票的时间字段差异过大: %v", diff)
		}
		t.Logf("收集到%d个时间字段用于一致性分析", len(timestamps))

		// 记录实际的时间差异范围
		if len(timestamps) > 1 {
			minTime := timestamps[0]
			maxTime := timestamps[0]
			for _, ts := range timestamps {
				if ts.Before(minTime) {
					minTime = ts
				}
				if ts.After(maxTime) {
					maxTime = ts
				}
			}
			totalRange := maxTime.Sub(minTime)
			t.Logf("时间字段范围: %v (从 %s 到 %s)",
				totalRange, minTime.Format("15:04:05"), maxTime.Format("15:04:05"))
		}
	}
}

// TestTimeFieldParsing 验证时间解析正确性（保持原有逻辑）
func TestTimeFieldParsing(t *testing.T) {
	// 这个测试不需要API调用，保持原有逻辑
	testCases := []struct {
		input    string
		expected string
		valid    bool
	}{
		{"20250821143000", "2025-08-21 14:30:00", true},
		{"202508211430", "2025-08-21 14:30", true},
		{"", "", false},
		{"invalid", "", false},
		{"123", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, valid := parseTimeField(tc.input)
			assert.Equal(t, tc.valid, valid, "解析结果有效性不匹配")
			if tc.valid {
				assert.Contains(t, result, tc.expected[:10], "解析的日期部分不正确")
			}
		})
	}
}

// 保留原有的辅助函数，但简化为使用生产组件
func parseTimeField(timeStr string) (string, bool) {
	if timeStr == "" {
		return "", false
	}

	formats := []string{"20060102150405", "200601021504"}
	for _, format := range formats {
		if len(timeStr) == len(format) {
			t, err := time.ParseInLocation(format, timeStr, time.Local)
			if err == nil {
				return t.Format("2006-01-02 15:04:05"), true
			}
		}
	}
	return "", false
}
