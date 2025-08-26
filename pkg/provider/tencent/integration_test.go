//go:build integration

package tencent_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/subscriber"
)

// TestProvider_APIFormat 验证腾讯API返回的数据格式是否符合预期
func TestProvider_APIFormat_ValidResponse(t *testing.T) {
	// 使用一个有效的、不带前缀的股票代码列表
	symbols := []string{"600000", "000001", "601398"}

	// 创建腾讯Provider实例
	provider := tencent.NewProvider()

	// 调用FetchData获取数据
	data, err := provider.FetchData(context.Background(), symbols)

	// 断言没有错误发生
	require.NoError(t, err, "调用腾讯API不应返回错误")

	// 断言返回的数据量与请求的符号数量一致
	require.Len(t, data, len(symbols), "返回的数据量应与请求的符号数量一致")

	// 将返回结果转换为 map 以便快速查找和验证
	resultMap := make(map[string]subscriber.StockData, len(data))
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
	provider := tencent.NewProvider()

	// 选择代表性样本（每个市场1个）
	testSymbols := []string{
		"600000", // 上海主板
		"000001", // 深圳主板
		"300750", // 创业板
		"688036", // 科创板
		"835174", // 北交所
	}

	// 注意：此测试会真实调用外部API
	results, err := provider.FetchData(context.Background(), testSymbols)
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
