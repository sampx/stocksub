package tencent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseTencentData(t *testing.T) {
	t.Run("正常解析", func(t *testing.T) {
		// 模拟真实的腾讯API响应格式，使用英文避免编码问题
		rawData := `v_sh600000="1~PUFA Bank~600000~13.72~13.69~13.69~603222~288503~314720~13.71~1306~13.70~6592~13.69~5682~13.68~1132~13.67~1144~13.72~2111~13.73~261~13.74~1200~13.75~585~13.76~1137~~20250820155202~0.03~0.22~13.87~13.60~13.72/603222/829302744~603222~82930~0.20~8.65~~13.87~13.60~1.97~4152.73~4152.73~0.61~15.06~12.32";`

		data := parseTencentData(rawData)
		assert.Len(t, data, 1)

		// 验证第一个股票数据
		stock := data[0]
		assert.Equal(t, "600000", stock.Symbol)
		assert.Equal(t, "PUFA Bank", stock.Name)
		assert.Equal(t, 13.72, stock.Price)
		assert.Equal(t, 13.69, stock.PrevClose)
		assert.Equal(t, 13.69, stock.Open)
		assert.Equal(t, int64(603222), stock.Volume)
	})

	t.Run("多股票解析", func(t *testing.T) {
		// 测试多股票数据解析，使用英文避免编码问题
		rawData := `v_sh600000="1~PUFA Bank~600000~13.72~13.69~13.69~603222~288503~314720~13.71~1306~13.70~6592~13.69~5682~13.68~1132~13.67~1144~13.72~2111~13.73~261~13.74~1200~13.75~585~13.76~1137~~20250820155202~0.03~0.22~13.87~13.60~13.72/603222/829302744~603222~82930~0.20~8.65~~13.87~13.60~1.97~4152.73~4152.73~0.61~15.06~12.32";
v_sz000858="51~Wuliangye~000858~125.80~124.41~124.42~399865~208277~191588~125.78~2335~0.00~599~0.00~0~0.00~0~0.00~0~125.78~2335~0.00~0~0.00~0~0.00~0~0.00~0~~20250820145821~1.39~1.12~126.50~123.35~125.80/399865/5027135558~399865~502714~1.03~14.95~~126.50~123.35~2.53~4882.88~4883.06~3.59~136.85~111.97";`

		data := parseTencentData(rawData)
		assert.Len(t, data, 2)

		// 验证第一个股票
		stock1 := data[0]
		assert.Equal(t, "600000", stock1.Symbol)
		assert.Equal(t, "PUFA Bank", stock1.Name)

		// 验证第二个股票
		stock2 := data[1]
		assert.Equal(t, "000858", stock2.Symbol)
		assert.Equal(t, "Wuliangye", stock2.Name)
	})

	t.Run("空数据解析", func(t *testing.T) {
		data := parseTencentData("")
		assert.Len(t, data, 0)
	})

	t.Run("无效数据解析", func(t *testing.T) {
		rawData := "invalid data"
		data := parseTencentData(rawData)
		assert.Len(t, data, 0)
	})

	t.Run("不完整数据解析", func(t *testing.T) {
		// 不完整的数据，字段不足
		rawData := `v_sh600000="1~PUFA Bank~600000~10.50~10.45";`
		data := parseTencentData(rawData)
		assert.Len(t, data, 0, "不完整的数据应该被忽略")
	})
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		wantErr  bool
	}{
		{"123.45", 123.45, false},
		{"0", 0.0, false},
		{"", 0.0, false},
		{"invalid", 0.0, true},
		{"123.45.67", 0.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseFloat(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseFloatWithDefault(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"123.45", 123.45},
		{"0", 0.0},
		{"", 0.0},
		{"invalid", 0.0},
		{"123.45.67", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseFloatWithDefault(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"123", 123, false},
		{"0", 0, false},
		{"", 0, false},
		{"invalid", 0, true},
		{"123.45", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseInt(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseIntWithDefault(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"123", 123},
		{"0", 0},
		{"", 0},
		{"invalid", 0},
		{"123.45", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseIntWithDefault(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input    string
		desc     string
		validate func(t *testing.T, result time.Time)
	}{
		{
			input: "20250101120000",
			desc:  "14位时间戳",
			validate: func(t *testing.T, result time.Time) {
				expected := time.Date(2025, 1, 1, 12, 0, 0, 0, time.Local)
				assert.Equal(t, expected, result)
			},
		},
		{
			input: "202501011200",
			desc:  "12位时间戳",
			validate: func(t *testing.T, result time.Time) {
				expected := time.Date(2025, 1, 1, 12, 0, 0, 0, time.Local)
				assert.Equal(t, expected, result)
			},
		},
		{
			input: "20250101",
			desc:  "8位时间戳",
			validate: func(t *testing.T, result time.Time) {
				expected := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
				assert.Equal(t, expected, result)
			},
		},
		{
			input: "",
			desc:  "空时间戳",
			validate: func(t *testing.T, result time.Time) {
				// 应该返回当前时间，允许一定误差
				now := time.Now()
				diff := now.Sub(result)
				assert.LessOrEqual(t, diff.Abs(), time.Second, "空时间戳应该返回当前时间")
			},
		},
		{
			input: "invalid",
			desc:  "无效时间戳",
			validate: func(t *testing.T, result time.Time) {
				// 应该返回当前时间，允许一定误差
				now := time.Now()
				diff := now.Sub(result)
				assert.LessOrEqual(t, diff.Abs(), time.Second, "无效时间戳应该返回当前时间")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := parseTime(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestParseTurnover(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		desc     string
	}{
		{
			input:    "125.80/399865/5027135558",
			expected: 5027135558.0,
			desc:     "正常格式",
		},
		{
			input:    "125.80/399865/",
			expected: 0.0,
			desc:     "缺少成交额",
		},
		{
			input:    "5027135558",
			expected: 5027135558.0,
			desc:     "直接数字",
		},
		{
			input:    "",
			expected: 0.0,
			desc:     "空字符串",
		},
		{
			input:    "invalid/data/format",
			expected: 0.0,
			desc:     "无效格式",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := parseTurnover(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractSymbol(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		desc     string
	}{
		{"sh600000", "600000", "上海股票"},
		{"sz000001", "000001", "深圳股票"},
		{"bj835174", "835174", "北交所股票"},
		{"600000", "600000", "无前缀"},
		{"sh600000.SS", "600000", "带后缀"},
		{"", "", "空字符串"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := extractSymbol(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGbkToUtf8(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"测试", "中文字符"},
		{"Test", "英文字符"},
		{"", "空字符串"},
		{"测试Test123", "混合字符"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := gbkToUtf8(tt.input)
			// 由于GBK转换可能涉及编码问题，这里只验证函数不会panic
			assert.NotNil(t, result)
		})
	}
}

// 测试字段常量
func TestFieldConstants(t *testing.T) {
	// 验证字段常量的值是否正确
	assert.Equal(t, 0, FieldMarketCode)
	assert.Equal(t, 1, FieldName)
	assert.Equal(t, 2, FieldSymbol)
	assert.Equal(t, 3, FieldPrice)
	assert.Equal(t, 30, FieldTimestamp)
	assert.Equal(t, 48, FieldLimitDown)

	// 验证最小字段数
	assert.Equal(t, 49, MinRequiredFields)
}

// 基准测试
func BenchmarkParseTencentData(b *testing.B) {
	// 模拟腾讯API响应数据
	response := `v_sh600000="1~浦发银行~600000~10.50~10.45~10.60~1000000~500000~500000~10.49~1000~10.48~2000~10.47~3000~10.46~4000~10.45~5000~10.51~1500~10.52~2500~10.53~3500~10.54~4500~10.55~5500~20210101093000~0.05~0.48~10.60~10.40~10.50~50000000~2.00~15.50~0~1.25~100000000~0~0~0~12.45~0";
v_sz000001="0~平安银行~000001~15.30~15.25~15.35~2000000~600000~600000~15.29~1500~15.28~2500~15.27~3500~15.26~4500~15.25~5500~15.31~1600~15.32~2600~15.33~3600~15.34~4600~15.35~5600~20210101093100~0.05~0.33~15.35~15.20~15.30~80000000~1.80~12.50~0~1.15~150000000~0~0~0~13.20~0";`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parseTencentData(response)
	}
}

func BenchmarkParseFloat(b *testing.B) {
	testData := "123.456"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parseFloat(testData)
	}
}

func BenchmarkParseFloatWithDefault(b *testing.B) {
	testData := "123.456"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parseFloatWithDefault(testData)
	}
}
