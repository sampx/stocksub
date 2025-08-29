//go:build integration

package tencent_test

import (
	"context"
	"testing"
	"time"

	"stocksub/pkg/core"
	"stocksub/pkg/provider/tencent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProvider_APIFormat 验证腾讯API返回的数据格式是否符合预期
func TestProvider_APIFormat_ValidResponse(t *testing.T) {
	// 使用一个有效的、不带前缀的股票代码列表
	symbols := []string{"600000", "000001", "601398"}

	// 创建腾讯Provider实例
	provider := tencent.NewClient()

	// 调用FetchStockData获取数据
	data, err := provider.FetchStockData(context.Background(), symbols)

	// 断言没有错误发生
	require.NoError(t, err, "调用腾讯API不应返回错误")

	// 断言返回的数据量与请求的符号数量一致
	require.Len(t, data, len(symbols), "返回的数据量应与请求的符号数量一致")

	// 将返回结果转换为 map 以便快速查找和验证
	resultMap := make(map[string]core.StockData, len(data))
	for _, stock := range data {
		resultMap[stock.Symbol] = stock
	}

	// 遍历请求的每一个股票代码，验证返回结果的准确性
	for _, symbol := range symbols {
		t.Run("Stock_"+symbol, func(t *testing.T) {
			// 确认每个请求的股票都存在于返回结果中
			stock, ok := resultMap[symbol]
			require.True(t, ok, "结果中未找到股票: %s", symbol)

			// 验证股票代码是正确的6位纯数字
			assert.Equal(t, symbol, stock.Symbol, "股票代码应与请求的完全一致")
			assert.Equal(t, 6, len(stock.Symbol), "股票代码长度应为6")

			assert.NotEmpty(t, stock.Name, "股票名称不应为空")
			assert.NotEqual(t, 0.0, stock.Price, "当前价格不应为0")
			assert.NotEqual(t, 0.0, stock.PrevClose, "昨日收盘价不应为0")
			assert.NotEqual(t, 0.0, stock.Open, "今日开盘价不应为0")
			assert.NotEqual(t, int64(0), stock.Volume, "成交量不应为0")
			assert.NotEqual(t, 0.0, stock.High, "最高价不应为0")
			assert.NotEqual(t, 0.0, stock.Low, "最低价不应为0")

			// 验证时间戳是否合理
			assert.False(t, stock.Timestamp.IsZero(), "时间戳不应为零值")
			assert.True(t, stock.Timestamp.Year() >= 2023, "时间戳年份应大于等于2023")
		})
	}
}

// TestTencent_TimeFieldAPIIntegration 集成测试：验证API返回数据的时间字段可被正确解析
func TestTencent_TimeFieldAPIIntegration(t *testing.T) {
	provider := tencent.NewClient()

	// 选择代表性样本（每个市场1个）
	testSymbols := []string{
		"600000", // 上海主板
		"000001", // 深圳主板
		"300750", // 创业板
		"688036", // 科创板
		"835174", // 北交所
	}

	// 注意：此测试会真实调用外部API
	results, err := provider.FetchStockData(context.Background(), testSymbols)
	assert.NoError(t, err, "API数据获取失败")
	assert.Equal(t, len(testSymbols), len(results), "返回数据数量不匹配")

	for _, result := range results {
		// 验证时间字段不为零值
		assert.False(t, result.Timestamp.IsZero(),
			"股票 %s 的时间字段为零值", result.Symbol)

		// 验证时间在合理范围内（不太过旧或过新）
		now := time.Now()
		age := now.Sub(result.Timestamp)
		assert.True(t, age >= 0 && age <= 24*time.Hour,
			"股票 %s 的时间戳 %s 不在合理范围内（与当前时间相差 %v）",
			result.Symbol, result.Timestamp.Format("2006-01-02 15:04:05"), age)
	}
}

// TestProvider_MarketCoverage 验证所有支持的市场都能正常工作
func TestProvider_MarketCoverage_AllMarkets(t *testing.T) {
	provider := tencent.NewClient()

	// 测试每个市场的代表性股票
	marketTests := map[string][]string{
		"上海主板": {"600000", "601398", "600036"},
		"深圳主板": {"000001", "000002", "000858"},
		"创业板":  {"300750", "300014", "300059"},
		"科创板":  {"688036", "688599", "688981"},
		"北交所":  {"835174", "832000", "873527"},
	}

	for marketName, symbols := range marketTests {
		t.Run("Market_"+marketName, func(t *testing.T) {
			results, err := provider.FetchStockData(context.Background(), symbols)

			require.NoError(t, err, "市场 %s 的API调用失败", marketName)
			assert.Equal(t, len(symbols), len(results),
				"市场 %s 返回数据数量不匹配", marketName)

			// 验证每个股票的数据质量
			for _, result := range results {
				assert.Contains(t, symbols, result.Symbol,
					"返回了未请求的股票代码: %s", result.Symbol)
				assert.NotEmpty(t, result.Name,
					"股票 %s 名称为空", result.Symbol)
				assert.Greater(t, result.Price, 0.0,
					"股票 %s 价格异常: %f", result.Symbol, result.Price)
			}
		})
	}
}

// TestProvider_ErrorHandling 验证错误处理机制
func TestProvider_ErrorHandling_InvalidSymbols(t *testing.T) {
	provider := tencent.NewClient()

	testCases := []struct {
		name        string
		symbols     []string
		expectError bool
		description string
	}{
		{
			name:        "无效股票代码",
			symbols:     []string{"999999", "111111"},
			expectError: false, // API可能返回空数据而不是错误
			description: "不存在的股票代码",
		},
		{
			name:        "混合有效无效代码",
			symbols:     []string{"600000", "999999", "000001"},
			expectError: false,
			description: "混合有效和无效的股票代码",
		},
		{
			name:        "空请求",
			symbols:     []string{},
			expectError: false, // 空请求返回空结果而不是错误
			description: "空的股票代码列表",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := provider.FetchStockData(context.Background(), tc.symbols)

			if tc.expectError {
				assert.Error(t, err, "预期 %s 应该返回错误", tc.description)
			} else {
				assert.NoError(t, err, "%s 不应该返回错误", tc.description)
				// 对于无效代码，结果可能为空或部分数据
				assert.True(t, len(results) <= len(tc.symbols),
					"返回数据不应超过请求数量")
			}
		})
	}
}

// TestProvider_DataQuality 验证数据质量和合理性
func TestProvider_DataQuality_ReasonableValues(t *testing.T) {
	provider := tencent.NewClient()

	// 选择一些知名的大盘股进行数据质量验证
	symbols := []string{"600000", "000001", "601398", "000002", "600036"}

	results, err := provider.FetchStockData(context.Background(), symbols)
	require.NoError(t, err, "数据获取失败")

	for _, result := range results {
		t.Run("DataQuality_"+result.Symbol, func(t *testing.T) {
			// 价格合理性验证
			assert.True(t, result.Price > 0 && result.Price < 10000,
				"股票 %s 当前价格异常: %f", result.Symbol, result.Price)
			assert.True(t, result.PrevClose > 0 && result.PrevClose < 10000,
				"股票 %s 昨收价异常: %f", result.Symbol, result.PrevClose)
			assert.True(t, result.Open > 0 && result.Open < 10000,
				"股票 %s 开盘价异常: %f", result.Symbol, result.Open)
			assert.True(t, result.High >= result.Low,
				"股票 %s 最高价应大于等于最低价", result.Symbol)
			assert.True(t, result.High >= result.Price && result.Low <= result.Price,
				"股票 %s 当前价格应在最高最低价范围内", result.Symbol)

			// 成交量合理性
			assert.True(t, result.Volume >= 0,
				"股票 %s 成交量不应为负数", result.Symbol)

			// 涨跌幅合理性（一般不超过±20%）
			if result.PrevClose > 0 {
				changePercent := (result.Price - result.PrevClose) / result.PrevClose
				assert.True(t, changePercent >= -0.25 && changePercent <= 0.25,
					"股票 %s 涨跌幅异常: %.2f%%", result.Symbol, changePercent*100)
			}
		})
	}
}

// TestProvider_FetchStockDataWithRaw 验证原始数据获取功能
func TestProvider_FetchStockDataWithRaw_Integration(t *testing.T) {
	provider := tencent.NewClient()
	symbols := []string{"600000", "000001"}

	results, rawData, err := provider.FetchStockDataWithRaw(context.Background(), symbols)

	require.NoError(t, err, "FetchStockDataWithRaw调用失败")
	assert.Equal(t, len(symbols), len(results), "解析后数据数量不匹配")
	assert.NotEmpty(t, rawData, "原始数据不应为空")

	// 验证原始数据包含预期内容
	for _, symbol := range symbols {
		assert.Contains(t, rawData, symbol,
			"原始数据应包含股票代码: %s", symbol)
	}

	// 验证解析后的数据与原始数据一致性
	for _, result := range results {
		assert.Contains(t, symbols, result.Symbol,
			"解析结果包含未请求的股票: %s", result.Symbol)
		assert.NotEmpty(t, result.Name, "股票名称不应为空")
	}
}

// TestProvider_ContextTimeout 验证上下文超时处理
func TestProvider_ContextTimeout_Handling(t *testing.T) {
	provider := tencent.NewClient()
	symbols := []string{"600000", "000001"}

	// 创建一个非常短的超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := provider.FetchStockData(ctx, symbols)

	// 应该返回超时错误或上下文取消错误
	if err != nil {
		// 检查错误是否包含超时相关信息
		assert.Contains(t, err.Error(), "context deadline exceeded",
			"应该返回超时相关错误，实际错误: %v", err)
	}
}

// TestProvider_PublicMethods 验证Provider公共方法的集成行为
func TestProvider_PublicMethods_Integration(t *testing.T) {
	provider := tencent.NewClient()

	t.Run("基础方法测试", func(t *testing.T) {
		// 测试名称
		assert.Equal(t, "tencent", provider.Name(), "Provider名称应为tencent")

		// 测试健康状态
		assert.True(t, provider.IsHealthy(), "Provider应该是健康的")

		// 测试频率限制
		rateLimit := provider.GetRateLimit()
		assert.True(t, rateLimit > 0, "频率限制应大于0")
	})

	t.Run("股票代码支持测试", func(t *testing.T) {
		supportedSymbols := []string{
			"600000", // 上海主板
			"000001", // 深圳主板
			"300750", // 创业板
			"688036", // 科创板
			"835174", // 北交所
		}

		unsupportedSymbols := []string{
			"12345",   // 长度不足
			"1234567", // 长度过长
			"abc123",  // 包含字母
			"HK0001",  // 港股代码格式
		}

		// 验证支持的股票代码
		for _, symbol := range supportedSymbols {
			assert.True(t, provider.IsSymbolSupported(symbol),
				"应该支持股票代码: %s", symbol)
		}

		// 验证不支持的股票代码
		for _, symbol := range unsupportedSymbols {
			assert.False(t, provider.IsSymbolSupported(symbol),
				"不应该支持股票代码: %s", symbol)
		}
	})

	t.Run("资源清理测试", func(t *testing.T) {
		// 测试Close方法
		err := provider.Close()
		assert.NoError(t, err, "Close方法不应返回错误")

		// Close后Provider仍应该可用（因为是无状态的）
		assert.True(t, provider.IsHealthy(), "Close后Provider应该仍可用")
	})
}
