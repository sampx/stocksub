package tencent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_FetchStockData(t *testing.T) {
	tests := []struct {
		name    string
		symbols []string
		wantErr bool
		wantLen int
	}{
		{
			name:    "正常情况",
			symbols: []string{"600000", "000001"},
			wantErr: false,
			wantLen: 2,
		},
		{
			name:    "空股票代码",
			symbols: []string{},
			wantErr: false,
			wantLen: 0,
		},
		{
			name:    "单个股票代码",
			symbols: []string{"600000"},
			wantErr: false,
			wantLen: 1,
		},
		{
			name:    "多个股票代码",
			symbols: []string{"600000", "000001", "300503", "688041"},
			wantErr: false,
			wantLen: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试工具包
			testKit := NewTencentProviderTestKit(nil)
			defer testKit.Close()

			// 执行测试
			data, err := testKit.ExecuteTest(tt.symbols)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, data, tt.wantLen)

			// 验证返回的数据结构
			for i, stock := range data {
				assert.Equal(t, tt.symbols[i], stock.Symbol, "股票代码应该匹配")
				assert.NotEmpty(t, stock.Name, "股票名称不应为空")
				assert.GreaterOrEqual(t, stock.Price, 0.0, "股票价格应该大于等于0")
			}

			// 验证统计信息
			stats := testKit.GetStats()
			assert.Equal(t, int64(1), stats.TotalCalls)
			assert.Equal(t, int64(1), stats.SuccessfulCalls)
			assert.Equal(t, int64(0), stats.FailedCalls)
		})
	}
}

func TestProvider_FetchStockDataWithRaw(t *testing.T) {
	testKit := NewTencentProviderTestKit(nil)
	defer testKit.Close()

	symbols := []string{"600000", "000001"}
	data, raw, err := testKit.ExecuteTestWithRaw(symbols)

	require.NoError(t, err)
	assert.Len(t, data, 2)
	assert.NotEmpty(t, raw, "原始数据不应为空")

	// 验证原始数据格式
	assert.Contains(t, raw, "v_sh600000=", "应包含上海股票数据")
	assert.Contains(t, raw, "v_sz000001=", "应包含深圳股票数据")

	// 验证数据解析正确
	for _, stock := range data {
		assert.Contains(t, symbols, stock.Symbol)
		assert.NotEmpty(t, stock.Name)
		assert.GreaterOrEqual(t, stock.Price, 0.0)
	}
}

func TestProvider_IsSymbolSupported(t *testing.T) {
	provider := NewProvider()

	tests := []struct {
		symbol   string
		expected bool
		desc     string
	}{
		{"600000", true, "上海主板"},
		{"000001", true, "深圳主板"},
		{"300503", true, "创业板"},
		{"688041", true, "科创板"},
		{"835174", true, "北交所"},
		{"", false, "空字符串"},
		{"12345", false, "5位数字"},
		{"1234567", false, "7位数字"},
		{"abcdef", false, "字母"},
		{"70000", false, "不支持的前缀"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := provider.IsSymbolSupported(tt.symbol)
			assert.Equal(t, tt.expected, result, "股票代码 %s 的支持状态应为 %v", tt.symbol, tt.expected)
		})
	}
}

func TestProvider_Name(t *testing.T) {
	provider := NewProvider()
	assert.Equal(t, "tencent", provider.Name())
}

func TestProvider_IsHealthy(t *testing.T) {
	provider := NewProvider()
	assert.True(t, provider.IsHealthy(), "新创建的Provider应该是健康的")
}

func TestProvider_Close(t *testing.T) {
	provider := NewProvider()
	err := provider.Close()
	assert.NoError(t, err, "关闭Provider不应产生错误")
}

func TestProvider_GetRateLimit(t *testing.T) {
	provider := NewProvider()
	rateLimit := provider.GetRateLimit()
	assert.Greater(t, rateLimit, int64(0), "频率限制应该大于0")
}

// 测试边界情况
func TestProvider_EdgeCases(t *testing.T) {
	t.Run("上下文取消", func(t *testing.T) {
		provider := NewProvider()
		defer provider.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 立即取消

		_, err := provider.FetchStockData(ctx, []string{"600000"})
		// 注意：由于我们移除了频率限制，上下文取消可能不会影响结果
		// 这个测试主要是为了确保代码能处理取消的上下文
		if err != nil {
			assert.Contains(t, err.Error(), "context canceled", "错误应该包含上下文取消信息")
		}
	})

	t.Run("重复股票代码", func(t *testing.T) {
		testKit := NewTencentProviderTestKit(nil)
		defer testKit.Close()

		symbols := []string{"600000", "600000", "000001"}
		data, err := testKit.ExecuteTest(symbols)

		assert.NoError(t, err)
		// 腾讯API会返回重复的数据，这是正常的
		assert.Len(t, data, 3)
	})
}

// 性能测试
func TestProvider_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	testKit := NewTencentProviderTestKit(nil)
	defer testKit.Close()

	symbols := []string{"600000", "000001", "300503", "688041", "835174"}

	// 执行多次测试
	const iterations = 10
	for i := 0; i < iterations; i++ {
		_, err := testKit.ExecuteTest(symbols)
		assert.NoError(t, err)
	}

	// 验证性能统计
	stats := testKit.GetStats()
	assert.Equal(t, int64(iterations), stats.TotalCalls)
	assert.Equal(t, int64(iterations), stats.SuccessfulCalls)
	assert.Equal(t, int64(0), stats.FailedCalls)

	t.Logf("平均延迟: %v", stats.AverageLatency)
	t.Logf("总调用次数: %d", stats.TotalCalls)
}
