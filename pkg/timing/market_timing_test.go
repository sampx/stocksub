package timing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockTimeService 模拟时间服务
type MockTimeService struct {
	current time.Time
}

func (m *MockTimeService) Now() time.Time {
	return m.current
}

func TestMarketTiming_TradingTime_AllCases(t *testing.T) {
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

func TestMarketTiming_TradingDay(t *testing.T) {
	tests := []struct {
		name     string
		mockTime string
		expected bool
	}{
		{"周一-交易日", "2025-08-25", true},
		{"周二-交易日", "2025-08-26", true},
		{"周三-交易日", "2025-08-27", true},
		{"周四-交易日", "2025-08-28", true},
		{"周五-交易日", "2025-08-29", true},
		{"周六-休市", "2025-08-23", false},
		{"周日-休市", "2025-08-24", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTime, _ := time.Parse("2006-01-02", tt.mockTime)
			mockService := &MockTimeService{current: mockTime}

			mt := NewMarketTime(mockService)
			actual := mt.IsTradingDay(mockTime)

			assert.Equal(t, tt.expected, actual, "日期 %s 的交易日状态应匹配预期", mockTime.Format("2006-01-02"))
		})
	}
}

func TestMarketTiming_AfterTradingEnd(t *testing.T) {
	tests := []struct {
		name     string
		mockTime string
		expected bool
	}{
		{"收盘后1秒", "2025-08-21 15:00:11", true},
		{"收盘后1分钟", "2025-08-21 15:01:00", true},
		{"收盘后2小时", "2025-08-21 17:00:00", true},
		{"收盘当时", "2025-08-21 15:00:10", false},
		{"下午盘", "2025-08-21 14:59:59", false},
		{"下午正常", "2025-08-21 14:30:00", false},
		{"早盘正常", "2025-08-21 10:30:00", false},
		{"周末", "2025-08-23 16:00:00", false},
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

// 是否收盘临近5分钟
func TestMarketTiming_IsCloseToEnd(t *testing.T) {
	tests := []struct {
		name     string
		mockTime string
		expected bool
	}{
		{"收盘前5分钟-不是", "2025-08-21 14:50:00", false},
		{"收盘前5分钟-是", "2025-08-21 14:55:00", true},
		{"收盘前3分钟-是", "2025-08-21 14:57:00", true},
		{"收盘前1分钟-是", "2025-08-21 14:59:00", true},
		{"刚刚收盘-是", "2025-08-21 15:00:00", true},
		{"收盘后5秒内-是", "2025-08-21 15:00:05", true},
		{"收盘后10秒-是", "2025-08-21 15:00:10", true},
		{"收盘后11秒-不是", "2025-08-21 15:00:11", false},
		{"下午正常时段-不是", "2025-08-21 14:30:00", false},
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

// 距离当前时间最近的交易日开盘时间
func TestMarketTiming_GetNextTradingDayStart(t *testing.T) {
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
		{"月末", "2025-08-29 16:00:00", "2025-09-01 09:13:30"},      // 8月29日是周五，过了交易时间，下一个交易日是下周一(9月1日)
		{"月初", "2025-09-01 09:00:00", "2025-09-01 09:13:30"},
		{"年末", "2025-12-31 16:00:00", "2026-01-01 09:13:30"}, // 下一年
		{"年初", "2026-01-01 09:00:00", "2026-01-01 09:13:30"},
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

// 当天交易结束时间
func TestMarketTiming_GetTradingEndTime(t *testing.T) {
	location := time.FixedZone("CST", 8*3600) // 中国时区

	tests := []struct {
		name         string
		mockTime     string
		expectedTime string
	}{
		{"上午10点", "2025-08-21 10:00:00", "2025-08-21 15:00:10"},
		{"下午2点", "2025-08-21 14:00:00", "2025-08-21 15:00:10"},
		{"下午3点", "2025-08-21 15:00:00", "2025-08-21 15:00:10"},
		{"不同日期", "2025-08-22 09:00:00", "2025-08-22 15:00:10"},
		{"月初", "2025-08-01 10:00:00", "2025-08-01 15:00:10"},
		{"年末", "2025-12-31 10:00:00", "2025-12-31 15:00:10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTime, _ := time.ParseInLocation("2006-01-02 15:04:05", tt.mockTime, location)
			expectedEnd, _ := time.ParseInLocation("2006-01-02 15:04:05", tt.expectedTime, location)

			mockService := &MockTimeService{current: mockTime}
			mt := NewMarketTime(mockService)

			actualEnd := mt.GetTradingEndTime()

			assert.WithinDuration(t, expectedEnd, actualEnd, time.Second, "交易结束时间应匹配预期")
		})
	}
}
