//go:build integration

package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stocksub/pkg/provider/tencent"
)

// TestTencentAPIFormat 验证腾讯API返回的数据格式是否符合预期
func TestTencentAPIFormat(t *testing.T) {
	// 使用一个有效的股票代码列表
	symbols := []string{"sh600000", "sz000001", "sh601398"}

	// 创建腾讯Provider实例
	provider := tencent.NewProvider()

	// 调用FetchData获取数据
	data, err := provider.FetchData(context.Background(), symbols)

	// 断言没有错误发生
	require.NoError(t, err, "调用腾讯API不应返回错误")

	// 断言返回的数据量与请求的符号数量一致
	require.Len(t, data, len(symbols), "返回的数据量应与请求的符号数量一致")

	// 遍历返回的每一条数据，进行详细格式验证
	for _, stock := range data {
		t.Run("Stock_"+stock.Symbol, func(t *testing.T) {
			assert.NotEmpty(t, stock.Symbol, "股票代码不应为空")
			assert.True(t, strings.HasPrefix(stock.Symbol, "sh") || strings.HasPrefix(stock.Symbol, "sz"), "股票代码应包含sh或sz前缀")

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
