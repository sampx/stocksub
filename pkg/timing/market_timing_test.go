package timing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTimeService 模拟时间服务
type MockTimeService struct {
	current time.Time
}

func (m *MockTimeService) Now() time.Time {
	return m.current
}

func TestMarketTimingEdgeCases(t *testing.T) {
	// 测试边界条件：准确的时间窗口检测
	tests := []struct {
		name     string
		mockTime string
		expected bool
	}{
		// 上午时段边界测试
		{"禁止启动-09:13:29", "2025-08-21 09:13:29", false},
		{"允许启动-09:13:30", "2025-08-21 09:13:30", true},
		{"正常交易-10:00:00", "2025-08-21 10:00:00", true},
		{"上午结束-11:30:10", "2025-08-21 11:30:10", true},
		{"上午结束-11:30:11", "2025-08-21 11:30:11", false},

		// 下午时段边界测试
		{"下午禁止-12:57:29", "2025-08-21 12:57:29", false},
		{"下午允许-12:57:30", "2025-08-21 12:57:30", true},
		{"下午正常-14:00:00", "2025-08-21 14:00:00", true},
		{"下午结束-15:00:10", "2025-08-21 15:00:10", true},
		{"下午结束-15:00:11", "2025-08-21 15:00:11", false},

		// 非交易日测试
		{"周六-禁止", "2025-08-23 10:00:00", false},
		{"周日-禁止", "2025-08-24 10:00:00", false},

		// 边沿时间测试
		{"凌晨禁止-08:59:59", "2025-08-21 08:59:59", false},
		{"中午恢复-13:00:00", "2025-08-21 13:00:00", true},
		{"深夜禁止-22:00:00", "2025-08-21 22:00:00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTime, _ := time.Parse("2006-01-02 15:04:05", tt.mockTime)
			mockService := &MockTimeService{current: mockTime}

			mt := NewMarketTime(mockService)
			actual := mt.IsTradingTime()

			assert.Equal(t, tt.expected, actual, "时间 %s 的交易状态应匹配预期", mockTime.Format("15:04:05"))
		})
	}
}

func TestIsTradingDay(t *testing.T) {
	tests := []struct {
		name     string
		mockTime string
		expected bool
	}{
		{"周一-交易日", "2025-08-25 10:00:00", true},
		{"周二-交易日", "2025-08-26 10:00:00", true},
		{"周三-交易日", "2025-08-27 10:00:00", true},
		{"周四-交易日", "2025-08-28 10:00:00", true},
		{"周五-交易日", "2025-08-29 10:00:00", true},
		{"周六-休市", "2025-08-23 10:00:00", false},
		{"周日-休市", "2025-08-24 10:00:00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTime, _ := time.Parse("2006-01-02 15:04:05", tt.mockTime)
			mockService := &MockTimeService{current: mockTime}

			mt := NewMarketTime(mockService)
			actual := mt.IsTradingDay(mockTime)

			assert.Equal(t, tt.expected, actual, "日期 %s 的交易日状态应匹配预期", mockTime.Format("2006-01-02"))
		})
	}
}

func TestIsAfterTradingEnd(t *testing.T) {
	tests := []struct {
		name     string
		mockTime string
		expected bool
	}{
		{"收盘后1秒-应检查", "2025-08-21 15:00:11", true},
		{"收盘后1分钟-应检查", "2025-08-21 15:01:00", true},
		{"收盘后2小时-应检查", "2025-08-21 17:00:00", true},
		{"收盘当时-不检查", "2025-08-21 15:00:10", false},
		{"收盘前-不检查", "2025-08-21 14:59:59", false},
		{"下午正常-不检查", "2025-08-21 14:30:00", false},
		{"上午时段-不检查", "2025-08-21 10:30:00", false},
		{"周末-不检查", "2025-08-23 16:00:00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTime, _ := time.Parse("2006-01-02 15:04:05", tt.mockTime)
			mockService := &MockTimeService{current: mockTime}

			mt := NewMarketTime(mockService)
			actual := mt.IsAfterTradingEnd()

			assert.Equal(t, tt.expected, actual, "时间 %s 的收盘后状态应匹配预期", mockTime.Format("15:04:05"))
		})
	}
}

func TestIsCloseToEnd(t *testing.T) {
	tests := []struct {
		name     string
		mockTime string
		expected bool
	}{
		{"收盘前5分钟-允许", "2025-08-21 14:55:00", true},
		{"收盘前3分钟-允许", "2025-08-21 14:57:00", true},
		{"收盘前1分钟-允许", "2025-08-21 14:59:00", true},
		{"收盘前0分钟-允许", "2025-08-21 15:00:00", true},
		{"收盘后禁止", "2025-08-21 15:00:11", false},
		{"下午正常时段-禁止", "2025-08-21 14:30:00", false},
		{"未知时间点-允许", "2025-08-21 14:58:00", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTime, _ := time.Parse("2006-01-02 15:04:05", tt.mockTime)
			mockService := &MockTimeService{current: mockTime}

			mt := NewMarketTime(mockService)
			actual := mt.IsCloseToEnd()

			assert.Equal(t, tt.expected, actual, "时间 %s 的收盘检测状态应匹配预期", mockTime.Format("15:04:05"))
		})
	}
}

func TestGetNextTradingDayStart(t *testing.T) {
	location := time.FixedZone("CST", 8*3600) // 中国时区

	tests := []struct {
		name         string
		mockTime     string
		expectedTime string
	}{
		{"工作日上午9点", "2025-08-21 09:00:00", "2025-08-21 09:13:30"},
		{"工作日下午16点", "2025-08-21 16:00:00", "2025-08-22 09:13:30"},
		{"周五下午16点", "2025-08-22 16:00:00", "2025-08-25 09:13:30"}, // 下周一
		{"周六上午", "2025-08-23 10:00:00", "2025-08-25 09:13:30"},    // 下周一
		{"周日上午", "2025-08-24 10:00:00", "2025-08-25 09:13:30"},    // 下周一
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTime, _ := time.ParseInLocation("2006-01-02 15:04:05", tt.mockTime, location)
			expected, _ := time.ParseInLocation("2006-01-02 15:04:05", tt.expectedTime, location)

			mockService := &MockTimeService{current: mockTime}
			mt := NewMarketTime(mockService)

			actual := mt.GetNextTradingDayStart()

			// 忽略纳秒级差异，只比较到秒级别
			assert.WithinDuration(t, expected, actual, time.Second, "下一个交易日开始时间应匹配预期")
		})
	}
}

func TestGetTradingEndTime(t *testing.T) {
	location := time.FixedZone("CST", 8*3600) // 中国时区
	mockTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-21 10:00:00", location)
	expectedEnd, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-21 15:00:10", location)

	mockService := &MockTimeService{current: mockTime}
	mt := NewMarketTime(mockService)

	actualEnd := mt.GetTradingEndTime()

	assert.WithinDuration(t, expectedEnd, actualEnd, time.Second, "交易结束时间应匹配预期")
}

func TestDefaultMarketTime(t *testing.T) {
	mt := DefaultMarketTime()
	require.NotNil(t, mt, "DefaultMarketTime应返回非空实例")
	assert.NotNil(t, mt, "系统时间服务应正确初始化")

	// 由于测试环境差异，这里只需验证调用不会panic
	result := mt.IsTradingTime()
	assert.IsType(t, true, result, "IsTradingTime应返回布尔值")
}
