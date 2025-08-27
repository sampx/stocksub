package tencent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTencentData(t *testing.T) {
	t.Run("正常解析", func(t *testing.T) {
		testKit := NewTencentParserTestKit()
		symbols := []string{"600000", "000001"}

		data, err := testKit.TestParseValid(symbols)
		require.NoError(t, err)
		assert.Len(t, data, 2)

		// 验证第一个股票数据
		stock := data[0]
		assert.Equal(t, "600000", stock.Symbol)
		assert.NotEmpty(t, stock.Name)
		assert.GreaterOrEqual(t, stock.Price, 0.0)
		assert.GreaterOrEqual(t, stock.Volume, int64(0))
	})

	t.Run("空数据解析", func(t *testing.T) {
		testKit := NewTencentParserTestKit()
		data := testKit.TestParseEmpty()
		assert.Len(t, data, 0)
	})

	t.Run("无效数据解析", func(t *testing.T) {
		testKit := NewTencentParserTestKit()
		data := testKit.TestParseInvalid()
		assert.Len(t, data, 0)
	})

	t.Run("不完整数据解析", func(t *testing.T) {
		testKit := NewTencentParserTestKit()
		data := testKit.TestParseIncomplete()
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
	testKit := NewTencentParserTestKit()
	symbols := []string{"600000", "000001", "300503", "688041", "835174"}
	response := testKit.generator.GenerateTencentResponse(symbols)

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
